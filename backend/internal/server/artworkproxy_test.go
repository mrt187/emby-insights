package server

import (
	"bytes"
	"io"
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

// roundTripFunc stands in for the network. Without it this file used to let
// the proxy dial image.tmdb.org for real: the assertion still passed, but it
// passed for the wrong reason (the socket failed, not the allow list), and
// running the unit tests reached out to a third party.
type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) { return fn(request) }

// TestArtworkProxyAcceptsTheCDNHosts proves the allow list is not simply
// refusing everything: an allowed host makes it all the way to the fetch,
// with the URL intact.
func TestArtworkProxyAcceptsTheCDNHosts(t *testing.T) {
	app, cookie := artworkTestApp()

	var fetched []string
	app.imageFetchClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		fetched = append(fetched, request.URL.String())
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"image/png"}},
			Body:       io.NopCloser(bytes.NewReader(pngBytes)),
		}, nil
	})}

	for target, want := range map[string]string{
		"https%3A%2F%2Fimage.tmdb.org%2Ft%2Fp%2Fw300%2Fa.jpg":      "https://image.tmdb.org/t/p/w300/a.jpg",
		"https%3A%2F%2Fartworks.thetvdb.com%2Fbanners%2Fx%2F1.jpg": "https://artworks.thetvdb.com/banners/x/1.jpg",
	} {
		fetched = nil
		if code := getArtwork(app, cookie, target).Code; code != http.StatusOK {
			t.Errorf("target %q answered %d, want 200", target, code)
		}
		if len(fetched) != 1 || fetched[0] != want {
			t.Errorf("target %q fetched %v, want exactly [%s]", target, fetched, want)
		}
	}
}
