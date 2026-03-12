package aip2p

import "testing"

func TestSubscribedAnnouncementTopics(t *testing.T) {
	t.Parallel()

	topics := subscribedAnnouncementTopics(SyncSubscriptions{
		Topics: []string{"world", "WORLD"},
		Tags:   []string{"breaking"},
	})
	if len(topics) != 2 {
		t.Fatalf("topics len = %d, want 2", len(topics))
	}
	if topics[0] != "aip2p/announce/topic/world" && topics[1] != "aip2p/announce/topic/world" {
		t.Fatalf("missing topic subscription: %v", topics)
	}
	if topics[0] != "aip2p/announce/tag/breaking" && topics[1] != "aip2p/announce/tag/breaking" {
		t.Fatalf("missing tag subscription: %v", topics)
	}
}

func TestMatchesAnnouncement(t *testing.T) {
	t.Parallel()

	announcement := SyncAnnouncement{
		Channel: "latest.org/world",
		Topics:  []string{"world", "pc75"},
		Tags:    []string{"breaking"},
	}
	if !matchesAnnouncement(announcement, SyncSubscriptions{Topics: []string{"pc75"}}) {
		t.Fatal("expected topic match")
	}
	if !matchesAnnouncement(announcement, SyncSubscriptions{Channels: []string{"latest.org/world"}}) {
		t.Fatal("expected channel match")
	}
	if matchesAnnouncement(announcement, SyncSubscriptions{Topics: []string{"markets"}}) {
		t.Fatal("unexpected topic match")
	}
}
