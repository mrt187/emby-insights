package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mrt187/EmbyInsights/internal/artwork"
	"github.com/mrt187/EmbyInsights/internal/emby"
)

func artworkTestApp() (*App, *http.Cookie) {
	return &App{
			sessions:         &memorySessionStore{identity: emby.Identity{UserID: "user-1", DisplayName: "Tom"}},
			imageFetchClient: newImageFetchClient(),
			loginLimiters:    make(map[string]*loginRateLimiter),
		},
		&http.Cookie{Name: sessionCookieName, Value: "session-id"}
}

func getArtwork(app *App, cookie *http.Cookie, rawTarget string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(http.MethodGet, artwork.ProxyPath+"?u="+rawTarget, nil)
	request.AddCookie(cookie)
	recorder := httptest.NewRecorder()
	app.artworkImage(recorder, request)
	return recorder
}

// TestArtworkProxyRefusesAnythingButTheArtworkCDNs is the important one: the
// target arrives as a query parameter, so an authenticated user could try to
// aim the proxy at the network the server sits in. The host allow list has to
// hold on its own, before the fetch ever happens.
func TestArtworkProxyRefusesAnythingButTheArtworkCDNs(t *testing.T) {
	app, cookie := artworkTestApp()

	for _, target := range []string{
		"",
		"https://evil.example/x.jpg",
		"https://image.tmdb.org.evil.example/x.jpg",
		"https://evil.example/?a=image.tmdb.org",
		"https%3A%2F%2Fimage.tmdb.org%40evil.example%2Fx.jpg",
		"http://image.tmdb.org/x.jpg",
		"https://127.0.0.1/x.jpg",
		"https://localhost/x.jpg",
		"https://10.18.2.2:8096/Items/1/Images/Primary",
		"https://169.254.169.254/latest/meta-data/",
		"https://%5B%3A%3Affff%3A127.0.0.1%5D/x.jpg",
		"file:///etc/passwd",
		"gopher://image.tmdb.org/x",
		"javascript:alert(1)",
	} {
		if code := getArtwork(app, cookie, target).Code; code != http.StatusBadRequest {
			t.Errorf("BYPASS: target %q answered %d, want 400", target, code)
		}
	}
}

func TestArtworkProxyRequiresAuthentication(t *testing.T) {
	app, _ := artworkTestApp()
	request := httptest.NewRequest(http.MethodGet, artwork.ProxyPath+"?u=https://image.tmdb.org/t/p/w300/a.jpg", nil)
	recorder := httptest.NewRecorder()
	app.artworkImage(recorder, request)
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401 without a session", recorder.Code)
	}
}

// TestArtworkProxyAcceptsTheCDNHosts proves the allow list is not simply
// refusing everything — the request gets past validation and only then fails
// on the socket, because the test has no network.
func TestArtworkProxyAcceptsTheCDNHosts(t *testing.T) {
	app, cookie := artworkTestApp()
	for _, target := range []string{
		"https%3A%2F%2Fimage.tmdb.org%2Ft%2Fp%2Fw300%2Fa.jpg",
		"https%3A%2F%2Fartworks.thetvdb.com%2Fbanners%2Fx%2F1.jpg",
	} {
		if code := getArtwork(app, cookie, target).Code; code == http.StatusBadRequest {
			t.Errorf("target %q was rejected by validation, want it accepted", target)
		}
	}
}
