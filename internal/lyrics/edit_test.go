package lyrics

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestEditTextRoundTripSyncedLyrics(t *testing.T) {
	input := Result{
		Level: LevelLine,
		Lines: []Line{
			{Text: "first line", Start: 1250 * time.Millisecond, End: 3 * time.Second},
			{Text: "second line", Start: 3 * time.Second, End: 5 * time.Second},
		},
	}

	text, err := TextForEdit(input)
	if err != nil {
		t.Fatal(err)
	}
	if want := "[00:01.250]first line\n[00:03.000]second line"; text != want {
		t.Fatalf("TextForEdit() = %q, want %q", text, want)
	}

	got, err := ParseEditedText(text, LevelLine, 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Lines) != 2 ||
		got.Lines[0].Text != "first line" ||
		got.Lines[0].Start != 1250*time.Millisecond ||
		got.Lines[0].End != 3*time.Second ||
		got.Lines[1].Text != "second line" ||
		got.Lines[1].Start != 3*time.Second ||
		got.Lines[1].End != 5*time.Second {
		t.Fatalf("ParseEditedText() = %#v", got)
	}
}

func TestParseEditedPlainLyrics(t *testing.T) {
	got, err := ParseEditedText("first line\r\n\r\nlast line\n", LevelNone, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Lines) != 3 ||
		got.Lines[0].Text != "first line" ||
		got.Lines[1].Text != "" ||
		got.Lines[2].Text != "last line" {
		t.Fatalf("ParseEditedText() = %#v", got)
	}
}

func TestParseEditedSyncedLyricsRejectsUnsafeInput(t *testing.T) {
	tests := []struct {
		name string
		text string
	}{
		{name: "missing timestamp", text: "line without timing"},
		{name: "invalid timestamp", text: "[00:60.000]bad seconds"},
		{name: "invalid fraction", text: "[0:01.0000]too precise"},
		{name: "untimed line", text: "[00:01.000]first\nuntimed text"},
		{name: "empty lyrics", text: "[00:01.000]"},
		{name: "empty", text: " \n"},
		{name: "control character", text: "[00:01.000]bad\x00text"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseEditedText(tt.text, LevelLine, 0)
			if err == nil {
				t.Fatal("ParseEditedText() succeeded, want validation error")
			}
			var editErr *EditError
			if errors.As(err, &editErr) && editErr.Line < 1 {
				t.Fatalf("EditError.Line = %d, want positive line number", editErr.Line)
			}
		})
	}
}

func TestParseEditedLRCLyrics(t *testing.T) {
	got, err := ParseEditedText("[ar:Artist]\n[00:02.34]Second\n[00:01.2][00:03.456]First", LevelNone, 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	want := []Line{
		{Text: "First", Start: 1200 * time.Millisecond, End: 2340 * time.Millisecond},
		{Text: "Second", Start: 2340 * time.Millisecond, End: 3456 * time.Millisecond},
		{Text: "First", Start: 3456 * time.Millisecond, End: 5 * time.Second},
	}
	if got.Level != LevelLine || len(got.Lines) != len(want) {
		t.Fatalf("ParseEditedText() = %#v, want synced lyrics", got)
	}
	for i := range want {
		if got.Lines[i] != want[i] {
			t.Errorf("line %d = %#v, want %#v", i, got.Lines[i], want[i])
		}
	}
}

func TestParseEditedSyncedLyricsReportsFailingLine(t *testing.T) {
	_, err := ParseEditedText("[00:01.000]first\n[00:60.000]invalid", LevelLine, 0)
	var editErr *EditError
	if !errors.As(err, &editErr) {
		t.Fatalf("ParseEditedText() error = %v, want *EditError", err)
	}
	if editErr.Line != 2 {
		t.Fatalf("EditError.Line = %d, want 2", editErr.Line)
	}
}

func TestParseEditedTextRejectsOversizedInput(t *testing.T) {
	text := "[00:01.000]" + strings.Repeat("x", maxEditedTextSize)
	if _, err := ParseEditedText(text, LevelLine, 0); err == nil {
		t.Fatal("ParseEditedText() succeeded, want size error")
	}
}
