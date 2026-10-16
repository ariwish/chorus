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

type EditError struct {
	Line int
	Err  error
}

func (e *EditError) Error() string {
	if e.Line > 0 {
		return fmt.Sprintf("line %d: %v", e.Line, e.Err)
	}
	return e.Err.Error()
}

func (e *EditError) Unwrap() error {
	return e.Err
}

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
				return "", editLineError(i+1, "lyrics: negative timestamp")
			}
			if strings.ContainsAny(line.Text, "\r\n") {
				return "", editLineError(i+1, "lyrics: synced line contains a line break")
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
	for i, r := range text {
		if unicode.IsControl(r) && r != '\n' && r != '\t' {
			return "", editLineError(strings.Count(text[:i], "\n")+1, "lyrics: unsupported control character")
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
	for i, r := range text {
		if unicode.IsControl(r) && r != '\n' && r != '\t' {
			return Result{}, editLineError(strings.Count(text[:i], "\n")+1, "lyrics: unsupported control character")
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
			return Result{}, editLineError(i+1, "blank synced lines require timestamps")
		}
		if !strings.HasPrefix(raw, "[") {
			return Result{}, editLineError(i+1, "synced lines must start with an [mm:ss.xxx] timestamp")
		}
		close := strings.IndexByte(raw, ']')
		if close < 0 {
			return Result{}, editLineError(i+1, "incomplete timestamp")
		}
		start, ok := parseEditTimestamp(raw[1:close])
		if !ok {
			return Result{}, editLineError(i+1, "invalid timestamp")
		}
		if i > 0 && start < previous {
			return Result{}, editLineError(i+1, "timestamps must be in chronological order")
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

func editLineError(line int, message string) error {
	return &EditError{Line: line, Err: fmt.Errorf("%s", message)}
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
