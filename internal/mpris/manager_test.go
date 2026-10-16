package mpris

import (
	"testing"
	"time"
)

func TestSetTrackResetsCurrentPositionWhenTrackChanges(t *testing.T) {
	m := &Manager{
		rosterCh: make(chan []Entry, 1),
		players:  map[string]Player{"player": {BusName: "player"}},
		tracks:   map[string]Track{"player": {Artist: "Artist", Title: "First"}},
		current:  attachment{busName: "player"},
		pos:      posState{base: 90 * time.Second, baseAt: time.Now(), rate: 1},
	}

	if !m.setTrack("player", Track{Artist: "Artist", Title: "Second"}) {
		t.Fatal("setTrack() did not report a track change")
	}
	if pos := m.CurrentPosition(); pos != 0 {
		t.Errorf("CurrentPosition() = %s, want 0 after track change", pos)
	}
}

func TestSetTrackDoesNotResetPositionForSameTrack(t *testing.T) {
	m := &Manager{
		rosterCh: make(chan []Entry, 1),
		players:  map[string]Player{"player": {BusName: "player"}},
		tracks:   map[string]Track{"player": {Artist: "Artist", Title: "Song"}},
		current:  attachment{busName: "player"},
		pos:      posState{base: 30 * time.Second, baseAt: time.Now(), rate: 1},
	}

	if m.setTrack("player", Track{Artist: "Artist", Title: "Song"}) {
		t.Fatal("setTrack() reported a change for the same track")
	}
	if pos := m.CurrentPosition(); pos != 30*time.Second {
		t.Errorf("CurrentPosition() = %s, want 30s for the same track", pos)
	}
}
