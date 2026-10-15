package lrclib

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/f1nniboy/chorus/internal/lyrics"
)

func TestParseSyncedLyrics(t *testing.T) {
	lines := parseSyncedLyrics("[ar:Artist]\n[00:01.2]First\n[00:02.34][00:03.456]Second\r\n", 5*time.Second)
	want := []lyrics.Line{
		{Text: "First", Start: 1200 * time.Millisecond, End: 2340 * time.Millisecond},
		{Text: "Second", Start: 2340 * time.Millisecond, End: 3456 * time.Millisecond},
		{Text: "Second", Start: 3456 * time.Millisecond, End: 5 * time.Second},
	}
	if len(lines) != len(want) {
		t.Fatalf("got %d lines, want %d: %#v", len(lines), len(want), lines)
	}
	for i := range want {
		if lines[i] != want[i] {
			t.Errorf("line %d = %#v, want %#v", i, lines[i], want[i])
		}
	}
}

func TestProviderFetch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/get" {
			t.Errorf("path = %q, want /api/get", r.URL.Path)
		}
		query := r.URL.Query()
		for key, want := range map[string]string{
			"artist_name": "Artist & Band",
			"track_name":  "Song?",
			"album_name":  "Album",
			"duration":    "123.5",
		} {
			if got := query.Get(key); got != want {
				t.Errorf("query %s = %q, want %q", key, got, want)
			}
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"instrumental":false,"plainLyrics":"Plain","syncedLyrics":"[00:01.00]Synced"}`))
	}))
	defer server.Close()

	p := &Provider{BaseURL: server.URL + "/api"}
	p.SetDeps(server.Client())
	p.Init()
	result, err := p.Fetch(context.Background(), lyrics.TrackQuery{
		Artist: "Artist & Band", Title: "Song?", Album: "Album", Duration: 123500 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("Fetch() error = %v", err)
	}
	if result.Level != lyrics.LevelLine || len(result.Lines) != 1 || result.Lines[0].Text != "Synced" {
		t.Fatalf("Fetch() result = %#v, want synced lyrics", result)
	}
	if result.Lines[0].End != 123500*time.Millisecond {
		t.Errorf("last line end = %s, want track duration %s", result.Lines[0].End, 123500*time.Millisecond)
	}
}

func TestProviderFetchPlainLyrics(t *testing.T) {
	p := providerForResponse(t, `{"plainLyrics":"Line one\nLine two","syncedLyrics":null}`)
	result, err := p.Fetch(context.Background(), lyrics.TrackQuery{})
	if err != nil {
		t.Fatalf("Fetch() error = %v", err)
	}
	if result.Level != lyrics.LevelNone || len(result.Lines) != 2 || result.Lines[1].Text != "Line two" {
		t.Fatalf("Fetch() result = %#v, want two plain lines", result)
	}
}

func TestProviderFetchInstrumental(t *testing.T) {
	p := providerForResponse(t, `{"instrumental":true}`)
	result, err := p.Fetch(context.Background(), lyrics.TrackQuery{})
	if err != nil {
		t.Fatalf("Fetch() error = %v", err)
	}
	if !result.Instrumental {
		t.Fatalf("Fetch() result = %#v, want instrumental", result)
	}
}

func TestProviderFetchNotFound(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()

	p := &Provider{BaseURL: server.URL}
	p.SetDeps(server.Client())
	p.Init()
	_, err := p.Fetch(context.Background(), lyrics.TrackQuery{})
	if !errors.Is(err, lyrics.ErrNotFound) {
		t.Fatalf("Fetch() error = %v, want ErrNotFound", err)
	}
}

func providerForResponse(t *testing.T, body string) *Provider {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(server.Close)

	p := &Provider{BaseURL: server.URL}
	p.SetDeps(server.Client())
	p.Init()
	return p
}
