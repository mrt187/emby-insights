package server

import (
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mrt187/EmbyInsights/internal/emby"
)

func newLoginTestApp(trustedProxies ...string) *App {
	return &App{
		authenticator:  fakeAuthenticator{err: emby.ErrInvalidCredentials},
		sessions:       &memorySessionStore{},
		trustedProxies: parseTrustedProxies(trustedProxies),
		loginLimiters:  make(map[string]*loginRateLimiter),
	}
}

func postLogin(app *App, username, remoteAddr, forwardedFor string) *httptest.ResponseRecorder {
	body := fmt.Sprintf(`{"username":%q,"password":"wrong"}`, username)
	request := httptest.NewRequest(http.MethodPost, "/api/auth/emby/login", strings.NewReader(body))
	request.RemoteAddr = remoteAddr
	if forwardedFor != "" {
		request.Header.Set("X-Forwarded-For", forwardedFor)
	}
	recorder := httptest.NewRecorder()
	app.Handler().ServeHTTP(recorder, request)
	return recorder
}

// TestLoginLimitIgnoresSpoofedForwardedFor is the regression for the old
// behaviour: the header was read unconditionally, so a fresh fake address per
// request meant the limiter never triggered.
func TestLoginLimitIgnoresSpoofedForwardedFor(t *testing.T) {
	app := newLoginTestApp() // no trusted proxies configured

	var throttled bool
	for attempt := range loginAttemptsPerUserAndIP + 3 {
		spoofed := fmt.Sprintf("203.0.113.%d", attempt+1)
		if postLogin(app, "victim", "198.51.100.7:5555", spoofed).Code == http.StatusTooManyRequests {
			throttled = true
			break
		}
	}
	if !throttled {
		t.Fatal("rotating X-Forwarded-For bypassed the login limit")
	}
}

func TestLoginLimitHonoursForwardedForFromTrustedProxy(t *testing.T) {
	app := newLoginTestApp("198.51.100.7/32")

	// Exhaust the per-(user, IP) budget for one client behind the proxy.
	for range loginAttemptsPerUserAndIP {
		if code := postLogin(app, "tom", "198.51.100.7:5555", "203.0.113.10").Code; code == http.StatusTooManyRequests {
			t.Fatal("throttled before the limit was reached")
		}
	}
	if code := postLogin(app, "tom", "198.51.100.7:5555", "203.0.113.10").Code; code != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want 429 for the exhausted client", code)
	}

	// A different client behind the same proxy must still get through — this
	// is what reading the header from a trusted proxy buys us.
	if code := postLogin(app, "tom", "198.51.100.7:5555", "203.0.113.99").Code; code == http.StatusTooManyRequests {
		t.Fatal("a different client behind the proxy was throttled")
	}
}

func TestLoginLimitNormalizesUsername(t *testing.T) {
	app := newLoginTestApp()

	for range loginAttemptsPerUserAndIP {
		postLogin(app, "tom", "203.0.113.5:1234", "")
	}
	// Same account, different spelling: must share the budget, not reset it.
	if code := postLogin(app, "  ToM  ", "203.0.113.5:1234", "").Code; code != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want 429 — username casing reset the limit", code)
	}
}

// TestLoginLimitPerUserAllowsOtherAccounts guards against the lockout risk of
// a username-only limit: one account being throttled must not affect another.
func TestLoginLimitPerUserAllowsOtherAccounts(t *testing.T) {
	app := newLoginTestApp()

	for range loginAttemptsPerUser + 5 {
		postLogin(app, "victim", "203.0.113.5:1234", "")
	}
	if code := postLogin(app, "victim", "203.0.113.5:1234", "").Code; code != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want 429 for the hammered account", code)
	}
	if code := postLogin(app, "someone-else", "203.0.113.5:1234", "").Code; code == http.StatusTooManyRequests {
		t.Fatal("throttling one account locked out a different one")
	}
}

// TestLoginLimitPerUserCatchesDistributedGuessing covers the second key: many
// source addresses against one account still run into the per-username limit.
func TestLoginLimitPerUserCatchesDistributedGuessing(t *testing.T) {
	app := newLoginTestApp()

	var throttled bool
	for attempt := range loginAttemptsPerUser + 5 {
		remote := fmt.Sprintf("203.0.113.%d:1234", attempt%250+1)
		if postLogin(app, "tom", remote, "").Code == http.StatusTooManyRequests {
			throttled = true
			break
		}
	}
	if !throttled {
		t.Fatal("guessing one account from many addresses was never throttled")
	}
}

func TestParseTrustedProxiesAcceptsBareIPsAndDropsGarbage(t *testing.T) {
	networks := parseTrustedProxies([]string{"10.0.0.5", "192.168.0.0/16", "not-an-ip", "  "})
	if len(networks) != 2 {
		t.Fatalf("parsed %d networks, want 2", len(networks))
	}

	app := &App{trustedProxies: networks}
	for _, candidate := range []string{"10.0.0.5", "192.168.4.4"} {
		if !app.isTrustedProxy(parseTestIP(candidate)) {
			t.Errorf("%s should be trusted", candidate)
		}
	}
	for _, candidate := range []string{"10.0.0.6", "203.0.113.1"} {
		if app.isTrustedProxy(parseTestIP(candidate)) {
			t.Errorf("%s should not be trusted", candidate)
		}
	}
	if app.isTrustedProxy(nil) {
		t.Error("a nil address must not be trusted")
	}
}

func parseTestIP(value string) net.IP { return net.ParseIP(value) }
