package control

import (
	"encoding/json"
	"testing"
	"time"

	"veltrix/internal/shared"
)

func TestLogStreamHubPublishesOnlyToMatchingOwnerDeviceAndSite(t *testing.T) {
	hub := newLogStreamHub()
	matching := hub.subscribe(7, 11, "web", 0)
	wrongUser := hub.subscribe(8, 11, "web", 0)
	wrongDevice := hub.subscribe(7, 12, "web", 0)
	wrongSite := hub.subscribe(7, 11, "api", 0)
	defer hub.unsubscribe(matching)
	defer hub.unsubscribe(wrongUser)
	defer hub.unsubscribe(wrongDevice)
	defer hub.unsubscribe(wrongSite)

	hub.publish(7, 11, shared.LogEvent{SiteId: "web", Line: "server started"})

	select {
	case raw := <-matching.messages:
		var event map[string]interface{}
		if err := json.Unmarshal(raw, &event); err != nil { t.Fatalf("decode event: %v", err) }
		if event["type"] != "log" || event["site_id"] != "web" || event["line"] != "server started" {
			t.Fatalf("unexpected event: %#v", event)
		}
		if _, ok := event["timestamp"]; !ok { t.Fatal("event timestamp missing") }
	case <-time.After(time.Second):
		t.Fatal("matching subscriber did not receive log")
	}

	for name, sub := range map[string]*logStreamSubscriber{
		"wrong user": wrongUser, "wrong device": wrongDevice, "wrong site": wrongSite,
	} {
		select {
		case <-sub.messages:
			t.Fatalf("%s received another subscriber's log", name)
		default:
		}
	}
}

func TestLogStreamHubDropsOldestWhenSubscriberIsSlow(t *testing.T) {
	hub := newLogStreamHub()
	sub := hub.subscribe(1, 2, "site", 0)
	defer hub.unsubscribe(sub)

	for i := 0; i < 140; i++ {
		hub.publish(1, 2, shared.LogEvent{SiteId: "site", Line: string(rune('a' + i%26))})
	}
	if got := len(sub.messages); got != 128 {
		t.Fatalf("queued messages = %d, want bounded queue of 128", got)
	}
}

func TestLogStreamReplayIsBoundedAndCarriesEpoch(t *testing.T) {
	hub := newLogStreamHub()
	for i := 0; i < 300; i++ {
		hub.publish(3, 4, shared.LogEvent{SiteId: "site", Line: "line"})
	}
	sub := hub.subscribe(3, 4, "site", 0)
	defer hub.unsubscribe(sub)
	if got := len(sub.messages); got != 128 {
		t.Fatalf("replay queue = %d, want 128", got)
	}
	raw := <-sub.messages
	var event map[string]interface{}
	if err := json.Unmarshal(raw, &event); err != nil {
		t.Fatalf("decode replay event: %v", err)
	}
	if event["stream_id"] != hub.streamID {
		t.Fatalf("stream_id = %v, want %s", event["stream_id"], hub.streamID)
	}
	if event["seq"] != float64(173) {
		t.Fatalf("first replay seq = %v, want 173 (latest 128 of 300)", event["seq"])
	}
}
