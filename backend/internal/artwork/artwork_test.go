package artwork

import (
	"net/url"
	"strings"
	"testing"
)

func TestProxyURLRewritesSupportedHosts(t *testing.T) {
	cases := map[string]string{
		"https://image.tmdb.org/t/p/w300/abc.jpg":      "https://image.tmdb.org/t/p/w300/abc.jpg",
		"http://image.tmdb.org/t/p/w500/abc.jpg":       "https://image.tmdb.org/t/p/w500/abc.jpg",
		"https://artworks.thetvdb.com/banners/x/1.jpg": "https://artworks.thetvdb.com/banners/x/1.jpg",
		"http://ARTWORKS.THETVDB.COM/banners/x/1.jpg":  "https://ARTWORKS.THETVDB.COM/banners/x/1.jpg",
	}
	for raw, wantTarget := range cases {
		got := ProxyURL(raw)
		if !strings.HasPrefix(got, ProxyPath+"?u=") {
			t.Fatalf("ProxyURL(%q) = %q, want a %s path", raw, got, ProxyPath)
		}
		parsed, err := url.Parse(got)
		if err != nil {
			t.Fatalf("ProxyURL(%q) produced an unparseable URL: %v", raw, err)
		}
		if target := parsed.Query().Get("u"); target != wantTarget {
			t.Errorf("ProxyURL(%q) targets %q, want %q — the scheme must be normalised to https", raw, target, wantTarget)
		}
	}
}

// TestProxyURLDropsEverythingElse is the guard that keeps the proxy from being
// pointed at anything but the artwork CDNs. It runs the same host tricks the
// image fetcher is hardened against, because a URL that reaches ProxyURL is
// what the handler will later be asked to fetch.
func TestProxyURLDropsEverythingElse(t *testing.T) {
	for _, raw := range []string{
		"https://evil.example/x.jpg",
		"https://image.tmdb.org.evil.example/x.jpg",
		"https://evil.example/image.tmdb.org/x.jpg",
		"https://image.tmdb.org@evil.example/x.jpg",
		"https://127.0.0.1/x.jpg",
		"https://10.18.2.2:8096/x.jpg",
		"https://169.254.169.254/latest/meta-data/",
		"https://[::ffff:127.0.0.1]/x.jpg",
		"file:///etc/passwd",
		"gopher://image.tmdb.org/x",
		"javascript:alert(1)",
		"data:image/png;base64,AAAA",
		"",
	} {
		if got := ProxyURL(raw); got != "" {
			t.Errorf("ProxyURL(%q) = %q, want \"\" — that host must not be proxied", raw, got)
		}
	}
}

func TestProxyURLLeavesOurOwnPathsAlone(t *testing.T) {
	for _, raw := range []string{"/api/images?itemId=1&type=Primary&tag=a&maxWidth=400", "/api/tracking/poster?source=emby&id=1"} {
		if got := ProxyURL(raw); got != raw {
			t.Errorf("ProxyURL(%q) = %q, want it unchanged", raw, got)
		}
	}
}

func TestAllowedHostIgnoresPortAndCase(t *testing.T) {
	for _, host := range []string{"image.tmdb.org", "IMAGE.TMDB.ORG", "image.tmdb.org:443", "artworks.thetvdb.com"} {
		if !AllowedHost(host) {
			t.Errorf("%s should be allowed", host)
		}
	}
	for _, host := range []string{"", "evil.example", "image.tmdb.org.evil.example", "tmdb.org"} {
		if AllowedHost(host) {
			t.Errorf("%s must not be allowed", host)
		}
	}
}
