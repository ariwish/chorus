package cache

import (
	"os"
	"testing"
)

func TestSetAtomicallyReplacesCachedValue(t *testing.T) {
	c := &Cache{dir: t.TempDir()}

	if err := c.Set("entry", []byte("first")); err != nil {
		t.Fatal(err)
	}
	if err := c.Set("entry", []byte("second")); err != nil {
		t.Fatal(err)
	}
	data, ok := c.Get("entry")
	if !ok || string(data) != "second" {
		t.Fatalf("Get() = %q, %v; want second, true", data, ok)
	}

	entries, err := os.ReadDir(c.dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "entry" {
		t.Fatalf("cache directory entries = %v, want only entry", entries)
	}
}
