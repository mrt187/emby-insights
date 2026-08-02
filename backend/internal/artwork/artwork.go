// Package artwork routes poster and backdrop images that live on a public CDN
// through the dashboard's own origin.
//
// Seerr, Radarr and Sonarr all hand back absolute URLs on image.tmdb.org or
// artworks.thetvdb.com. Letting the browser load those directly costs two
// things: the page needs a Content-Security-Policy wide enough to allow the
// CDNs, and every user's browser tells those CDNs which titles that user is
// looking at. Rewriting them to a path on our own origin removes both.
package artwork

import (
	"net/url"
	"strings"
)

// ProxyPath is the route that serves proxied artwork.
const ProxyPath = "/api/artwork"

// allowedHosts is the set of artwork CDNs worth proxying. It is deliberately a
// closed list: the URL arrives from a configured upstream, but the proxy
// fetches whatever it is pointed at, so the host is the one thing that must
// not be attacker-influenced.
var allowedHosts = map[string]bool{
	"image.tmdb.org":       true,
	"artworks.thetvdb.com": true,
}

// AllowedHost reports whether artwork may be fetched from host.
func AllowedHost(host string) bool {
	if index := strings.IndexByte(host, ':'); index >= 0 {
		host = host[:index]
	}
	return allowedHosts[strings.ToLower(host)]
}

// Hosts returns the allowed hosts, for logging and tests.
func Hosts() []string {
	hosts := make([]string, 0, len(allowedHosts))
	for host := range allowedHosts {
		hosts = append(hosts, host)
	}
	return hosts
}

// ProxyURL turns an absolute CDN URL into a path on our own origin. A URL that
// is already relative is ours and passes through unchanged. Anything on a host
// we do not proxy returns "" — the browser would only be told to load
// something the policy forbids, so the caller is better off with no poster and
// the placeholder the UI already draws.
func ProxyURL(raw string) string {
	if raw == "" {
		return ""
	}
	if strings.HasPrefix(raw, "/") {
		return raw
	}
	parsed, err := url.Parse(raw)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return ""
	}
	if !AllowedHost(parsed.Host) {
		return ""
	}
	// Upstreams sometimes hand back http:// for these; the proxy fetches over
	// https either way, so normalise here rather than at every call site.
	parsed.Scheme = "https"
	return ProxyPath + "?u=" + url.QueryEscape(parsed.String())
}
