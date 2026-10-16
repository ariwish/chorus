package providers

import (
	"net/http"
	"testing"
)

func TestLRCLIBIsAvailableAndConstructible(t *testing.T) {
	available := Available()
	if len(available) == 0 || available[0].ID != "lrclib" {
		t.Fatalf("first available provider = %#v, want LRCLIB", available)
	}

	provider, err := New("lrclib", nil, &http.Client{})
	if err != nil {
		t.Fatalf("New(lrclib) error = %v", err)
	}
	if provider.ID() != "lrclib" {
		t.Errorf("provider ID = %q, want lrclib", provider.ID())
	}

	fields := Fields("lrclib")
	if len(fields) != 1 || fields[0].Key != "base_url" || fields[0].Default != "https://lrclib.net/api" {
		t.Errorf("LRCLIB config fields = %#v", fields)
	}
}
