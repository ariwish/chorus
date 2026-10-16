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
		{name: "noncanonical timestamp", text: "[0:1.000]not canonical"},
		{name: "out of order", text: "[00:02.000]later\n[00:01.000]earlier"},
		{name: "blank line", text: "[00:01.000]first\n\n[00:02.000]second"},
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
