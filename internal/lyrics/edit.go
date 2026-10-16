package lyrics

import (
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

const maxEditedTextSize = 1 << 20

func TextForEdit(result Result) (string, error) {
	if result.Instrumental || len(result.Lines) == 0 {
		return "", fmt.Errorf("lyrics: no editable lyrics")
	}

	var b strings.Builder
	hasLyrics := false
	for i, line := range result.Lines {
		if i > 0 {
			b.WriteByte('\n')
		}
		if result.Level == LevelLine {
			if line.Start < 0 {
				return "", fmt.Errorf("lyrics: line %d has a negative timestamp", i+1)
			}
			if strings.ContainsAny(line.Text, "\r\n") {
				return "", fmt.Errorf("lyrics: synced line %d contains a line break", i+1)
			}
			minutes := int64(line.Start / time.Minute)
			seconds := int64(line.Start/time.Second) % 60
			milliseconds := int64(line.Start/time.Millisecond) % 1000
			fmt.Fprintf(&b, "[%02d:%02d.%03d]", minutes, seconds, milliseconds)
		} else if result.Level != LevelNone {
			return "", fmt.Errorf("lyrics: unsupported level %q", result.Level)
		}
		b.WriteString(line.Text)
		if strings.TrimSpace(line.Text) != "" {
			hasLyrics = true
		}
	}
	if !hasLyrics {
		return "", fmt.Errorf("lyrics: lyrics cannot be empty")
	}
	text := b.String()
	if len(text) > maxEditedTextSize {
		return "", fmt.Errorf("lyrics: edited text exceeds 1 MiB")
	}
	if !utf8.ValidString(text) {
		return "", fmt.Errorf("lyrics: lyrics are not valid UTF-8")
	}
	if strings.TrimSpace(text) == "" {
		return "", fmt.Errorf("lyrics: lyrics cannot be empty")
	}
	for _, r := range text {
		if unicode.IsControl(r) && r != '\n' && r != '\t' {
			return "", fmt.Errorf("lyrics: lyrics contain an unsupported control character")
		}
	}
	return text, nil
}

func ParseEditedText(text string, level Level, duration time.Duration) (Result, error) {
	if len(text) > maxEditedTextSize {
		return Result{}, fmt.Errorf("lyrics: edited text exceeds 1 MiB")
	}
	if !utf8.ValidString(text) {
		return Result{}, fmt.Errorf("lyrics: edited text is not valid UTF-8")
	}

	text = strings.ReplaceAll(strings.ReplaceAll(text, "\r\n", "\n"), "\r", "\n")
	for _, r := range text {
		if unicode.IsControl(r) && r != '\n' && r != '\t' {
			return Result{}, fmt.Errorf("lyrics: unsupported control character")
		}
	}
	if strings.TrimSpace(text) == "" {
		return Result{}, fmt.Errorf("lyrics: lyrics cannot be empty")
	}
	if strings.HasSuffix(text, "\n") {
		text = strings.TrimSuffix(text, "\n")
	}

	switch level {
	case LevelNone:
		lines := strings.Split(text, "\n")
		result := make([]Line, len(lines))
		for i, line := range lines {
			result[i].Text = line
		}
		return Result{Level: LevelNone, Lines: result}, nil
	case LevelLine:
		return parseEditedSyncedText(text, duration)
	default:
		return Result{}, fmt.Errorf("lyrics: unsupported level %q", level)
	}
}

func parseEditedSyncedText(text string, duration time.Duration) (Result, error) {
	rawLines := strings.Split(text, "\n")
	lines := make([]Line, 0, len(rawLines))
	var previous time.Duration
	hasLyrics := false

	for i, raw := range rawLines {
		if raw == "" {
			return Result{}, fmt.Errorf("lyrics: line %d is blank; synced lines require timestamps", i+1)
		}
		if !strings.HasPrefix(raw, "[") {
			return Result{}, fmt.Errorf("lyrics: line %d must start with an [mm:ss.xxx] timestamp", i+1)
		}
		close := strings.IndexByte(raw, ']')
		if close < 0 {
			return Result{}, fmt.Errorf("lyrics: line %d has an incomplete timestamp", i+1)
		}
		start, ok := parseEditTimestamp(raw[1:close])
		if !ok {
			return Result{}, fmt.Errorf("lyrics: line %d has an invalid timestamp", i+1)
		}
		if i > 0 && start < previous {
			return Result{}, fmt.Errorf("lyrics: timestamps must be in chronological order")
		}
		lines = append(lines, Line{Text: raw[close+1:], Start: start})
		if strings.TrimSpace(raw[close+1:]) != "" {
			hasLyrics = true
		}
		previous = start
	}
	if !hasLyrics {
		return Result{}, fmt.Errorf("lyrics: lyrics cannot be empty")
	}

	for i := range lines {
		if i+1 < len(lines) {
			lines[i].End = lines[i+1].Start
		} else if duration > lines[i].Start {
			lines[i].End = duration
		} else {
			lines[i].End = lines[i].Start
		}
	}
	return Result{Level: LevelLine, Lines: lines}, nil
}

func parseEditTimestamp(raw string) (time.Duration, bool) {
	minutePart, secondPart, ok := strings.Cut(raw, ":")
	if !ok || len(minutePart) < 2 || len(secondPart) != 6 || secondPart[2] != '.' {
		return 0, false
	}
	for _, r := range minutePart + secondPart[:2] + secondPart[3:] {
		if r < '0' || r > '9' {
			return 0, false
		}
	}
	minutes, err := strconv.ParseInt(minutePart, 10, 64)
	if err != nil || minutes < 0 {
		return 0, false
	}

	secondsPart, fraction := secondPart[:2], secondPart[3:]
	seconds, err := strconv.Atoi(secondsPart)
	if err != nil || seconds < 0 || seconds >= 60 {
		return 0, false
	}
	milliseconds, err := strconv.Atoi(fraction)
	if err != nil || milliseconds < 0 || milliseconds >= 1000 {
		return 0, false
	}

	const maxDuration = int64(^uint64(0) >> 1)
	remainder := int64(seconds)*int64(time.Second) + int64(milliseconds)*int64(time.Millisecond)
	if minutes > (maxDuration-remainder)/int64(time.Minute) {
		return 0, false
	}
	return time.Duration(minutes*int64(time.Minute) + remainder), true
}
