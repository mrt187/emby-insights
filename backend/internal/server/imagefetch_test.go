package server

import (
	"bytes"
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mrt187/EmbyInsights/internal/emby"
	"github.com/mrt187/EmbyInsights/internal/store"
)

// pngBytes is a minimal but genuine PNG: the signature is what
// http.DetectContentType keys on, so this stands in for a real poster.
var pngBytes = append([]byte("\x89PNG\r\n\x1a\n"), bytes.Repeat([]byte{0}, 64)...)

func newImageFetchTestApp(tracking store.TrackingStore) *App {
	return &App{
		sessions:         &memorySessionStore{identity: emby.Identity{UserID: "user-1"}},
		tracking:         tracking,
		imageFetchClient: newImageFetchClient(),
		loginLimiters:    make(map[string]*loginRateLimiter),
	}
}

func TestIsPubliclyRoutableBlocksInternalAndReservedRanges(t *testing.T) {
	blocked := []string{
		"127.0.0.1",        // loopback
		"10.0.0.2",        // private — the homelab Gitea host
		"192.168.1.10",     // private
		"172.16.5.4",       // private
		"169.254.169.254",  // cloud metadata
		"100.64.0.1",       // carrier-grade NAT
		"0.0.0.0",          // unspecified
		"255.255.255.255",  // broadcast
		"224.0.0.1",        // multicast
		"240.0.0.1",        // reserved
		"198.18.0.1",       // benchmarking
		"192.0.2.1",        // TEST-NET-1
		"::1",              // IPv6 loopback
		"fc00::1",          // IPv6 unique local
		"fe80::1",          // IPv6 link-local
		"::ffff:127.0.0.1", // IPv4-mapped loopback must not slip through
		"::ffff:10.0.0.1",  // IPv4-mapped private must not slip through
	}
	for _, candidate := range blocked {
		if isPubliclyRoutable(net.ParseIP(candidate)) {
			t.Errorf("%s was treated as publicly routable", candidate)
		}
	}

	allowed := []string{"1.1.1.1", "8.8.8.8", "93.184.216.34", "2606:4700::1111"}
	for _, candidate := range allowed {
		if !isPubliclyRoutable(net.ParseIP(candidate)) {
			t.Errorf("%s was blocked but is a public address", candidate)
		}
	}

	if isPubliclyRoutable(nil) {
		t.Error("an unparseable address must not be treated as routable")
	}
}

// TestFetchPosterRejectsInternalTarget is the DNS-rebinding case: the URL's
// host is resolved by the dialer, and the check runs on the address actually
// connected to. The local test server is on loopback, which is exactly what a
// rebinding record would eventually point at.
func TestFetchPosterRejectsInternalTarget(t *testing.T) {
	internal := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = writer.Write([]byte("internal service response"))
	}))
	defer internal.Close()

	app := newImageFetchTestApp(nil)
	if _, _, ok := app.fetchPosterBytes(context.Background(), "tmdb", internal.URL); ok {
		t.Fatal("fetch of an internal address succeeded")
	}
}

func TestFetchPosterRejectsNonHTTPSchemes(t *testing.T) {
	app := newImageFetchTestApp(nil)
	for _, candidate := range []string{
		"file:///etc/passwd",
		"gopher://127.0.0.1:6379/_INFO",
		"ftp://example.com/poster.jpg",
		"redis://127.0.0.1:6379",
	} {
		if _, _, ok := app.fetchPosterBytes(context.Background(), "tmdb", candidate); ok {
			t.Errorf("scheme %q was accepted", candidate)
		}
	}
}

// TestImageFetchClientRefusesRedirects checks CheckRedirect directly. Driving
// it through a local httptest redirector would prove nothing: that server sits
// on loopback and the dialer would reject it before any redirect happened.
func TestImageFetchClientRefusesRedirects(t *testing.T) {
	client := newImageFetchClient()
	if client.CheckRedirect == nil {
		t.Fatal("CheckRedirect is not set, redirects would be followed")
	}
	request := httptest.NewRequest(http.MethodGet, "http://10.0.0.2/internal", nil)
	if err := client.CheckRedirect(request, nil); err == nil {
		t.Fatal("CheckRedirect allowed a redirect")
	}
	if client.Timeout == 0 {
		t.Fatal("the image fetch client has no timeout")
	}
}

func TestReadImageBodyRejectsOversizedResponse(t *testing.T) {
	oversized := append(append([]byte{}, pngBytes...), bytes.Repeat([]byte{0}, maxPosterBytes)...)
	if _, _, ok := readImageBody(bytes.NewReader(oversized)); ok {
		t.Fatal("a response larger than maxPosterBytes was accepted")
	}

	atLimit := append(append([]byte{}, pngBytes...), bytes.Repeat([]byte{0}, maxPosterBytes-len(pngBytes))...)
	if _, _, ok := readImageBody(bytes.NewReader(atLimit)); !ok {
		t.Fatal("a response exactly at maxPosterBytes was rejected")
	}
}

func TestReadImageBodyIgnoresForgedContentType(t *testing.T) {
	// The bytes decide, not the header — these are all non-images that a
	// remote server could label "image/jpeg" to get them stored and served.
	for name, payload := range map[string]string{
		"html":       "<html><script>alert(1)</script></html>",
		"svg":        `<svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script></svg>`,
		"json":       `{"aws":"credentials"}`,
		"plain text": "root:x:0:0:root:/root:/bin/bash",
	} {
		if _, _, ok := readImageBody(strings.NewReader(payload)); ok {
			t.Errorf("%s payload was accepted as an image", name)
		}
	}

	if _, _, ok := readImageBody(bytes.NewReader(nil)); ok {
		t.Error("an empty body was accepted as an image")
	}

	// A genuine PNG still has to pass, and the type must come from the bytes.
	data, contentType, ok := readImageBody(bytes.NewReader(pngBytes))
	if !ok || contentType != "image/png" || len(data) == 0 {
		t.Fatalf("genuine PNG rejected: contentType = %q, ok = %v", contentType, ok)
	}
}

type fakePosterStore struct {
	data        []byte
	contentType string
	found       bool
	err         error
}

func (s *fakePosterStore) Get(context.Context, string, string, string) (store.MediaTracking, bool, error) {
	return store.MediaTracking{}, false, nil
}
func (s *fakePosterStore) Upsert(context.Context, string, store.MediaTracking) error { return nil }
func (s *fakePosterStore) Watchlist(context.Context, string) ([]store.MediaTracking, error) {
	return nil, nil
}
func (s *fakePosterStore) Ratings(context.Context, string) ([]store.MediaTracking, error) {
	return nil, nil
}
func (s *fakePosterStore) HiddenInProgressIDs(context.Context, string) (map[string]bool, error) {
	return nil, nil
}
func (s *fakePosterStore) TopRatings(context.Context, int) ([]store.AggregatedRating, error) {
	return nil, nil
}
func (s *fakePosterStore) PosterImage(context.Context, string, string) ([]byte, string, bool, error) {
	return s.data, s.contentType, s.found, s.err
}

// TestTrackingPosterRejectsUnsafeStoredBytes covers rows written before poster
// fetching was hardened: a stored non-image with an attacker-chosen content
// type must stop being served without needing a data migration.
func TestTrackingPosterRejectsUnsafeStoredBytes(t *testing.T) {
	unsafe := &fakePosterStore{
		data:        []byte("<html><script>alert(document.cookie)</script></html>"),
		contentType: "text/html",
		found:       true,
	}
	app := newImageFetchTestApp(unsafe)

	request := httptest.NewRequest(http.MethodGet, "/api/tracking/poster?source=tmdb&id=42", nil)
	request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "session-id"})
	recorder := httptest.NewRecorder()
	app.Handler().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", recorder.Code)
	}
	if got := recorder.Header().Get("Content-Type"); strings.Contains(got, "text/html") && recorder.Code == http.StatusOK {
		t.Fatalf("unsafe content type %q was served", got)
	}
	if strings.Contains(recorder.Body.String(), "alert(document.cookie)") {
		t.Fatal("stored script payload was served to the browser")
	}
}

func TestTrackingPosterServesSniffedContentType(t *testing.T) {
	// Stored type is deliberately wrong; the response must carry the type
	// derived from the bytes themselves.
	safe := &fakePosterStore{data: pngBytes, contentType: "text/html", found: true}
	app := newImageFetchTestApp(safe)

	request := httptest.NewRequest(http.MethodGet, "/api/tracking/poster?source=tmdb&id=42", nil)
	request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "session-id"})
	recorder := httptest.NewRecorder()
	app.Handler().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if got := recorder.Header().Get("Content-Type"); got != "image/png" {
		t.Fatalf("Content-Type = %q, want image/png", got)
	}
}

func TestSecurityHeadersIncludeContentSecurityPolicy(t *testing.T) {
	app := newImageFetchTestApp(nil)
	request := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	recorder := httptest.NewRecorder()
	securityHeaders(app.Handler()).ServeHTTP(recorder, request)

	policy := recorder.Header().Get("Content-Security-Policy")
	if policy == "" {
		t.Fatal("Content-Security-Policy header is missing")
	}
	for _, directive := range []string{
		"default-src 'self'",
		"script-src 'self'",
		"object-src 'none'",
		"frame-ancestors 'none'",
		"base-uri 'self'",
		"form-action 'self'",
	} {
		if !strings.Contains(policy, directive) {
			t.Errorf("policy is missing %q: %s", directive, policy)
		}
	}
	if strings.Contains(policy, "script-src 'self' 'unsafe-inline'") {
		t.Error("script-src must not allow inline scripts")
	}
	for header, want := range map[string]string{
		"X-Content-Type-Options": "nosniff",
		"X-Frame-Options":        "DENY",
		"Referrer-Policy":        "same-origin",
	} {
		if got := recorder.Header().Get(header); got != want {
			t.Errorf("%s = %q, want %q", header, got, want)
		}
	}
}

func TestMustParseCIDRsPanicsOnGarbage(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("an invalid CIDR did not panic")
		}
	}()
	mustParseCIDRs("not-a-cidr")
}
