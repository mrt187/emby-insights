// Package push sends Web Push notifications (W3C Push API) to subscribed
// browsers via VAPID-authenticated requests. It knows nothing about
// Postgres or HTTP routing — callers hand it a Subscription (endpoint +
// browser keys) and a JSON payload, and it does the actual delivery.
package push

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	webpush "github.com/SherClockHolmes/webpush-go"
)

// Subscription is the minimal shape needed to deliver a push message —
// mirrors the browser's PushSubscription object (endpoint + p256dh/auth
// keys), independent of how the caller stores it.
type Subscription struct {
	Endpoint string
	P256dh   string
	Auth     string
}

// Sender delivers Web Push messages using a fixed VAPID keypair/subject.
type Sender struct {
	publicKey  string
	privateKey string
	subject    string
	httpClient *http.Client
}

// NewSender builds a Sender from VAPID config. publicKey/privateKey are the
// base64url-encoded keypair (e.g. from GenerateVAPIDKeys or
// `npx web-push generate-vapid-keys`); subject is a mailto: or https: URL
// identifying the sender, as required by the VAPID spec.
func NewSender(publicKey, privateKey, subject string) *Sender {
	return &Sender{
		publicKey:  publicKey,
		privateKey: privateKey,
		subject:    normalizeSubject(subject),
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

// normalizeSubject strips a leading "mailto:": webpush-go prepends it itself
// to any non-"https:" subscriber, so a caller-supplied "mailto:x@y" would
// otherwise become the doubly-prefixed, invalid JWT sub claim
// "mailto:mailto:x@y" — which Apple's push service rejects with 403.
func normalizeSubject(subject string) string {
	if strings.HasPrefix(subject, "https:") {
		return subject
	}
	return strings.TrimPrefix(subject, "mailto:")
}

// PublicKey is safe to hand to the browser — it's how the frontend builds
// the applicationServerKey for pushManager.subscribe().
func (sender *Sender) PublicKey() string { return sender.publicKey }

// Send delivers one payload to one subscription. A 404/410 response from the
// push service means the subscription is gone (browser unsubscribed, user
// cleared site data, etc.) — ErrSubscriptionExpired lets callers clean up
// their store without treating it as a transient failure.
func (sender *Sender) Send(ctx context.Context, subscription Subscription, payload []byte) error {
	response, err := webpush.SendNotificationWithContext(ctx, payload, &webpush.Subscription{
		Endpoint: subscription.Endpoint,
		Keys: webpush.Keys{
			P256dh: subscription.P256dh,
			Auth:   subscription.Auth,
		},
	}, &webpush.Options{
		HTTPClient:      sender.httpClient,
		Subscriber:      sender.subject,
		VAPIDPublicKey:  sender.publicKey,
		VAPIDPrivateKey: sender.privateKey,
		TTL:             60,
	})
	if err != nil {
		return fmt.Errorf("send push notification: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode == http.StatusNotFound || response.StatusCode == http.StatusGone {
		return ErrSubscriptionExpired
	}
	if response.StatusCode >= 300 {
		return fmt.Errorf("push service returned %s", response.Status)
	}
	return nil
}

// ErrSubscriptionExpired signals the push endpoint no longer accepts
// deliveries and the subscription should be deleted from the store.
var ErrSubscriptionExpired = fmt.Errorf("push subscription expired")
