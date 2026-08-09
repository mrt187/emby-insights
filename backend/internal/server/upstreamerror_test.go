package server

import (
	"bytes"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// The point of upstreamUnavailable is that the cause survives in the log
// while the browser still only learns the generic message — a 502 used to
// leave no trace at all, which is what made issue #2 undiagnosable.
func TestUpstreamUnavailableLogsCauseAndHidesItFromTheResponse(t *testing.T) {
	var logged bytes.Buffer
	previous := log.Writer()
	log.SetOutput(&logged)
	log.SetFlags(0)
	defer func() {
		log.SetOutput(previous)
		log.SetFlags(log.LstdFlags)
	}()

	recorder := httptest.NewRecorder()
	cause := errors.New("Emby Insights personal stats returned 404 Not Found")
	upstreamUnavailable(recorder, "personal statistics are unavailable", cause)

	if recorder.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusBadGateway)
	}

	var body map[string]string
	if err := json.NewDecoder(recorder.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body["error"] != "personal statistics are unavailable" {
		t.Errorf("body error = %q, want the generic message", body["error"])
	}
	if strings.Contains(body["error"], "404") {
		t.Errorf("body leaked the upstream cause: %q", body["error"])
	}

	want := "personal statistics are unavailable: " + cause.Error()
	if got := strings.TrimSpace(logged.String()); got != want {
		t.Errorf("log = %q, want %q", got, want)
	}
}
