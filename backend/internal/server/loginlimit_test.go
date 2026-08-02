package server

import (
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/mrt187/EmbyInsights/internal/emby"
)

func newLoginTestApp(trustedProxies ...string) *App {
	return &App{
		authenticator:  fakeAuthenticator{err: emby.ErrInvalidCredentials},
		sessions:       &memorySessionStore{},
		trustedProxies: parseTrustedProxies(trustedProxies),
		loginLimiters:  make(map[string]*loginRateLimiter),
		// The spray delay is a wall-clock sleep in production; tests assert on
		// the decision instead of waiting it out.
		loginSprayDelay: time.Nanosecond,
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
	for attempt := range loginFailuresPerUserAndIP + 3 {
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
	for range loginFailuresPerUserAndIP {
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

	for range loginFailuresPerUserAndIP {
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

	for range loginFailuresPerUser + 5 {
		postLogin(app, "victim", "203.0.113.5:1234", "")
	}
	if code := postLogin(app, "victim", "203.0.113.5:1234", "").Code; code != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want 429 for the hammered account", code)
	}
	if code := postLogin(app, "someone-else", "203.0.113.5:1234", "").Code; code == http.StatusTooManyRequests {
		t.Fatal("throttling one account locked out a different one")
	}
}

// TestLoginLimitCannotLockOutAnAccount is the regression for the account
// lockout: hammering one username — from a single address or from many — must
// never stop the real owner from logging in somewhere else.
func TestLoginLimitCannotLockOutAnAccount(t *testing.T) {
	t.Run("from one address", func(t *testing.T) {
		app := newLoginTestApp()
		for range loginFailuresPerUser * 3 {
			postLogin(app, "victim", "203.0.113.66:1234", "")
		}
		if allowed, _ := app.loginDecision("victim", "198.51.100.5"); !allowed {
			t.Fatal("victim locked out from a clean address by a single attacker IP")
		}
	})

	t.Run("from many addresses", func(t *testing.T) {
		app := newLoginTestApp()
		for attempt := range loginFailuresPerUser * 3 {
			remote := fmt.Sprintf("203.0.113.%d:1234", attempt%200+1)
			postLogin(app, "victim", remote, "")
		}
		allowed, delay := app.loginDecision("victim", "198.51.100.5")
		if !allowed {
			t.Fatal("victim locked out from a clean address by distributed guessing")
		}
		if delay == 0 {
			t.Fatal("distributed guessing did not slow anything down")
		}
	})
}

// TestLoginLimitPerUserDelaysDistributedGuessing covers the second key: many
// source addresses against one account are slowed down rather than blocked.
func TestLoginLimitPerUserDelaysDistributedGuessing(t *testing.T) {
	app := newLoginTestApp()

	for attempt := range loginFailuresPerUser {
		remote := fmt.Sprintf("203.0.113.%d:1234", attempt%200+1)
		if code := postLogin(app, "tom", remote, "").Code; code == http.StatusTooManyRequests {
			t.Fatalf("attempt %d was refused; the account-wide limit must delay, not deny", attempt)
		}
	}
	allowed, delay := app.loginDecision("tom", "192.0.2.77")
	if !allowed {
		t.Fatal("the account-wide limit denied a request")
	}
	if delay == 0 {
		t.Fatalf("after %d failures the next attempt should be delayed", loginFailuresPerUser)
	}
}

// TestLoginLimitCountsFailuresNotRequests: a correct password must neither
// accumulate budget nor leave the previous failures behind.
func TestLoginLimitCountsFailuresNotRequests(t *testing.T) {
	app := newLoginTestApp()
	app.authenticator = fakeAuthenticator{identity: emby.Identity{UserID: "u1", DisplayName: "tom"}}

	for range loginFailuresPerUserAndIP * 4 {
		if code := postLogin(app, "tom", "203.0.113.5:1234", "").Code; code == http.StatusTooManyRequests {
			t.Fatal("successful logins were counted against the limit")
		}
	}
	if len(app.loginLimiters) != 0 {
		t.Fatalf("successful logins left %d limiter entries behind", len(app.loginLimiters))
	}
}

func TestLoginLimitSuccessClearsFailures(t *testing.T) {
	app := newLoginTestApp()
	for range loginFailuresPerUserAndIP - 1 {
		postLogin(app, "tom", "203.0.113.5:1234", "")
	}
	app.authenticator = fakeAuthenticator{identity: emby.Identity{UserID: "u1", DisplayName: "tom"}}
	postLogin(app, "tom", "203.0.113.5:1234", "")

	app.authenticator = fakeAuthenticator{err: emby.ErrInvalidCredentials}
	for range loginFailuresPerUserAndIP {
		if code := postLogin(app, "tom", "203.0.113.5:1234", "").Code; code == http.StatusTooManyRequests {
			t.Fatal("a successful login did not reset the failure count")
		}
	}
}

// TestLoginLimiterMapDoesNotGrowUnbounded is the regression for the memory
// leak: the map is keyed by an attacker-chosen username and had no eviction.
func TestLoginLimiterMapDoesNotGrowUnbounded(t *testing.T) {
	app := newLoginTestApp()

	stale := time.Now().Add(-2 * loginFailureWindow)
	for i := range 5000 {
		key := fmt.Sprintf("user:ghost-%d", i)
		app.loginLimiters[key] = &loginRateLimiter{attempts: []time.Time{stale}}
	}
	app.loginLimiters["user:recent"] = &loginRateLimiter{attempts: []time.Time{time.Now()}}

	app.loginLimitersSwept = stale // force the next decision to sweep
	app.loginDecision("someone", "203.0.113.5")

	if len(app.loginLimiters) > 2 {
		t.Fatalf("sweep left %d entries, want the aged-out ones gone", len(app.loginLimiters))
	}
	if _, ok := app.loginLimiters["user:recent"]; !ok {
		t.Fatal("sweep dropped a limiter that is still inside the window")
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
