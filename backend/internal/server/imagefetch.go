package server

import (
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"syscall"
	"time"
)

// maxPosterBytes caps a remote poster response. Anything larger is rejected
// outright rather than truncated: a truncated image is not a usable poster,
// and storing half a file only hides the problem until it is served.
const maxPosterBytes = 5 << 20

const posterFetchTimeout = 10 * time.Second

// allowedImageContentTypes is the set of formats we are willing to store and
// serve, keyed by what http.DetectContentType reports from the actual bytes.
// The Content-Type a remote server sends is never trusted — that header is
// exactly what an attacker would set to get text/html served back from our
// own origin.
var allowedImageContentTypes = map[string]bool{
	"image/jpeg": true,
	"image/png":  true,
	"image/webp": true,
	"image/gif":  true,
}

// errBlockedAddress is returned from the dialer's Control hook, so a poster
// URL that resolves into the homelab is refused at the socket layer. Doing
// the check here rather than on the hostname is deliberate: it sees the IP
// the connection actually goes to, which is what makes DNS rebinding (a name
// that resolves public once and internal on the second lookup) ineffective.
var errBlockedAddress = errors.New("address is not publicly routable")

// blockedNetworks covers loopback, private, link-local, carrier-grade NAT and
// the reserved/special-purpose ranges from IANA's registries. A poster URL is
// always a public CDN or Emby image, so blocking everything else costs
// nothing and removes the whole SSRF surface.
var blockedNetworks = mustParseCIDRs(
	// IPv4
	"0.0.0.0/8",       // this host on this network
	"10.0.0.0/8",      // private
	"100.64.0.0/10",   // carrier-grade NAT
	"127.0.0.0/8",     // loopback
	"169.254.0.0/16",  // link-local (incl. cloud metadata 169.254.169.254)
	"172.16.0.0/12",   // private
	"192.0.0.0/24",    // IETF protocol assignments
	"192.0.2.0/24",    // TEST-NET-1
	"192.88.99.0/24",  // deprecated 6to4 relay anycast
	"192.168.0.0/16",  // private
	"198.18.0.0/15",   // benchmarking
	"198.51.100.0/24", // TEST-NET-2
	"203.0.113.0/24",  // TEST-NET-3
	"224.0.0.0/4",     // multicast
	"240.0.0.0/4",     // reserved (incl. 255.255.255.255 broadcast)
	// IPv6
	"::/128",        // unspecified
	"::1/128",       // loopback
	"64:ff9b::/96",  // NAT64 — embeds an IPv4 address
	"100::/64",      // discard-only
	"2001::/32",     // Teredo
	"2001:db8::/32", // documentation
	"2002::/16",     // 6to4 — embeds an IPv4 address
	"fc00::/7",      // unique local
	"fe80::/10",     // link-local
	"ff00::/8",      // multicast
)

func mustParseCIDRs(entries ...string) []*net.IPNet {
	networks := make([]*net.IPNet, 0, len(entries))
	for _, entry := range entries {
		_, network, err := net.ParseCIDR(entry)
		if err != nil {
			panic(fmt.Sprintf("imagefetch: invalid CIDR %q: %v", entry, err))
		}
		networks = append(networks, network)
	}
	return networks
}

// isPubliclyRoutable reports whether ip is an address a poster may legitimately
// live on. IPv4-mapped IPv6 addresses are normalised to their 4-byte form
// first, so "::ffff:127.0.0.1" is judged by the IPv4 rules instead of slipping
// past them.
func isPubliclyRoutable(ip net.IP) bool {
	if ip == nil {
		return false
	}
	if v4 := ip.To4(); v4 != nil {
		ip = v4
	}
	for _, network := range blockedNetworks {
		if network.Contains(ip) {
			return false
		}
	}
	return true
}

// newImageFetchClient builds the HTTP client used for remote poster URLs.
// Every restriction here exists because the URL is attacker-controlled: it
// arrives in the body of a tracking upsert from any logged-in user.
func newImageFetchClient() *http.Client {
	dialer := &net.Dialer{
		Timeout:   5 * time.Second,
		KeepAlive: 30 * time.Second,
		Control: func(_, address string, _ syscall.RawConn) error {
			host, _, err := net.SplitHostPort(address)
			if err != nil {
				return fmt.Errorf("%w: %s", errBlockedAddress, address)
			}
			if !isPubliclyRoutable(net.ParseIP(host)) {
				return fmt.Errorf("%w: %s", errBlockedAddress, host)
			}
			return nil
		},
	}
	transport := &http.Transport{
		DialContext:           dialer.DialContext,
		TLSHandshakeTimeout:   5 * time.Second,
		ResponseHeaderTimeout: 5 * time.Second,
		DisableKeepAlives:     true,
	}
	return &http.Client{
		Timeout:   posterFetchTimeout,
		Transport: transport,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			// Not followed at all: a redirect is the standard way to get a
			// public hostname to hand out an internal target after the URL
			// itself has already been accepted.
			return errors.New("redirects are not followed for poster URLs")
		},
	}
}

// detectImageContentType sniffs the real format of data and returns the
// content type to store and serve, or false if the bytes are not one of the
// image formats we accept. This is the single gate that decides what a
// browser is ever told a stored poster is.
func detectImageContentType(data []byte) (string, bool) {
	if len(data) == 0 {
		return "", false
	}
	contentType := http.DetectContentType(data)
	if !allowedImageContentTypes[contentType] {
		return "", false
	}
	return contentType, true
}

// readImageBody reads at most maxPosterBytes from body and rejects anything
// larger, then verifies the bytes really are an image. It reads one byte past
// the limit on purpose so an oversized response is detected rather than
// silently truncated to exactly the limit.
func readImageBody(body io.Reader) ([]byte, string, bool) {
	data, err := io.ReadAll(io.LimitReader(body, maxPosterBytes+1))
	if err != nil {
		return nil, "", false
	}
	if len(data) > maxPosterBytes {
		return nil, "", false
	}
	contentType, ok := detectImageContentType(data)
	if !ok {
		return nil, "", false
	}
	return data, contentType, true
}
