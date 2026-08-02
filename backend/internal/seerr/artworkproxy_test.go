package seerr

import (
	"strings"
	"testing"
)

// TestPosterBaseURLIsProxied is a guard on the constant itself: every poster
// this package hands out has to leave through artwork.ProxyURL, so nothing in
// an API response ever points a browser at the CDN directly.
func TestPosterBaseURLIsProxied(t *testing.T) {
	if !strings.HasPrefix(posterBaseURL, "https://image.tmdb.org/") {
		t.Fatalf("posterBaseURL = %q, expected a TMDB URL that ProxyURL knows", posterBaseURL)
	}
}
