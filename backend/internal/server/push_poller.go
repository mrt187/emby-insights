package server

import (
	"context"
	"log"
	"time"
)

// RunPushPoller checks every subscribed user for new content on a fixed
// interval and pushes a notification for anything genuinely new — the
// second push trigger besides admin messages (see notifyPush). It runs
// until ctx is cancelled; call it as a goroutine from main, same as
// BackfillPosterImages.
func (app *App) RunPushPoller(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		interval = 20 * time.Minute
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			app.pollNewContentForPush(ctx)
		}
	}
}

// pollNewContentForPush runs a single tick: for every user with at least one
// push subscription, it re-runs the same "New For You" and
// "available requests" queries the dashboard itself uses, diffs the results
// against push_seen_items, and sends a push for whatever is new.
func (app *App) pollNewContentForPush(ctx context.Context) {
	if app.pushSeen == nil {
		return
	}
	userIDs, err := app.pushSeen.DistinctUserIDs(ctx)
	if err != nil {
		log.Printf("push poller: listing subscribed users failed: %v", err)
		return
	}
	for _, userID := range userIDs {
		app.pollNewForYouForUser(ctx, userID)
		app.pollAvailableRequestsForUser(ctx, userID)
	}
}

func (app *App) pollNewForYouForUser(ctx context.Context, userID string) {
	if app.newForYou == nil {
		return
	}
	items, err := app.newForYou.NewForYou(ctx, userID, app.live.newForYouLibraries())
	if err != nil {
		log.Printf("push poller: new-for-you lookup for %s failed: %v", userID, err)
		return
	}
	if len(items) == 0 {
		return
	}
	seenKeys := make([]string, len(items))
	byKey := make(map[string]struct {
		title string
		body  string
	}, len(items))
	for index, item := range items {
		key := "newforyou:" + item.ID
		seenKeys[index] = key
		title := item.Title
		if item.SeriesName != "" {
			title = item.SeriesName
		}
		byKey[key] = struct {
			title string
			body  string
		}{title: title, body: item.Title}
	}

	unseen, err := app.pushSeen.UnseenItemIDs(ctx, userID, seenKeys)
	if err != nil {
		log.Printf("push poller: diffing new-for-you items for %s failed: %v", userID, err)
		return
	}
	for _, key := range unseen {
		entry := byKey[key]
		app.notifyPush(userID, "New for you: "+entry.title, entry.body)
	}
	if err := app.pushSeen.MarkSeen(ctx, userID, seenKeys); err != nil {
		log.Printf("push poller: marking new-for-you items seen for %s failed: %v", userID, err)
	}
}

func (app *App) pollAvailableRequestsForUser(ctx context.Context, userID string) {
	if app.availableRequests == nil {
		return
	}
	requests, err := app.availableRequests.AvailableRequests(ctx, userID, time.Now().Add(-availableRequestWindow))
	if err != nil {
		log.Printf("push poller: available-requests lookup for %s failed: %v", userID, err)
		return
	}
	if len(requests) == 0 {
		return
	}
	seenKeys := make([]string, len(requests))
	titleByKey := make(map[string]string, len(requests))
	for index, request := range requests {
		key := "request:" + request.ID
		seenKeys[index] = key
		titleByKey[key] = request.Title
	}

	unseen, err := app.pushSeen.UnseenItemIDs(ctx, userID, seenKeys)
	if err != nil {
		log.Printf("push poller: diffing available requests for %s failed: %v", userID, err)
		return
	}
	for _, key := range unseen {
		app.notifyPush(userID, "Your request is available", titleByKey[key])
	}
	if err := app.pushSeen.MarkSeen(ctx, userID, seenKeys); err != nil {
		log.Printf("push poller: marking available requests seen for %s failed: %v", userID, err)
	}
}
