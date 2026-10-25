package lyrics

import (
	"fmt"
	"sort"
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
		if hasEditedTimestamp(text) {
			return parseEditedSyncedText(text, duration)
		}
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
	hasLyrics := false

	for i, raw := range rawLines {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		if !strings.HasPrefix(raw, "[") {
			return Result{}, editLineError(i+1, "synced lines must start with an LRC timestamp")
		}

		var starts []time.Duration
		for strings.HasPrefix(raw, "[") {
			close := strings.IndexByte(raw, ']')
			if close < 0 {
				return Result{}, editLineError(i+1, "incomplete timestamp or metadata tag")
			}
			tag := raw[1:close]
			if len(starts) == 0 && isLRCMetadataTag(tag) {
				break
			}
			start, ok := ParseLRCTimestamp(tag)
			if !ok {
				return Result{}, editLineError(i+1, "invalid LRC timestamp")
			}
			starts = append(starts, start)
			raw = raw[close+1:]
		}
		if len(starts) == 0 {
			if isLRCMetadataLine(raw) {
				continue
			}
			return Result{}, editLineError(i+1, "synced lines must start with an LRC timestamp")
		}

		text := strings.TrimSpace(raw)
		for _, start := range starts {
			lines = append(lines, Line{Text: text, Start: start})
		}
		if strings.TrimSpace(text) != "" {
			hasLyrics = true
		}
	}
	if !hasLyrics {
		return Result{}, editLineError(1, "lyrics cannot be empty")
	}

	sort.SliceStable(lines, func(i, j int) bool {
		return lines[i].Start < lines[j].Start
	})
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

func hasEditedTimestamp(text string) bool {
	for _, raw := range strings.Split(text, "\n") {
		raw = strings.TrimSpace(raw)
		if !strings.HasPrefix(raw, "[") {
			continue
		}
		close := strings.IndexByte(raw, ']')
		tag := raw[1:]
		if close >= 0 {
			tag = raw[1:close]
		}
		if isLRCMetadataTag(tag) {
			continue
		}
		separator := strings.IndexAny(tag, ":;")
		if separator > 0 {
			minutes := tag[:separator]
			allDigits := true
			for _, r := range minutes {
				if r < '0' || r > '9' {
					allDigits = false
					break
				}
			}
			if allDigits {
				return true
			}
		}
		if _, ok := ParseLRCTimestamp(tag); ok {
			return true
		}
	}
	return false
}

func isLRCMetadataLine(raw string) bool {
	if !strings.HasPrefix(raw, "[") {
		return false
	}
	close := strings.IndexByte(raw, ']')
	return close > 1 && isLRCMetadataTag(raw[1:close])
}

func isLRCMetadataTag(tag string) bool {
	key, _, ok := strings.Cut(tag, ":")
	if !ok || key == "" {
		return false
	}
	for _, r := range key {
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')) {
			return false
		}
	}
	return true
}

func editLineError(line int, message string) error {
	return &EditError{Line: line, Err: fmt.Errorf("%s", message)}
}

func ParseLRCTimestamp(raw string) (time.Duration, bool) {
	separator := strings.IndexAny(raw, ":;")
	if separator < 1 {
		return 0, false
	}
	minutePart := raw[:separator]
	secondPart := raw[separator+1:]
	for _, r := range minutePart {
		if r < '0' || r > '9' {
			return 0, false
		}
	}
	minutes, err := strconv.ParseInt(minutePart, 10, 64)
	if err != nil || minutes < 0 {
		return 0, false
	}

	secondsPart, fractionPart, hasFraction := strings.Cut(secondPart, ".")
	if !hasFraction {
		secondsPart, fractionPart, hasFraction = strings.Cut(secondPart, ":")
	}
	if len(secondsPart) < 1 || len(secondsPart) > 2 || (hasFraction && (len(fractionPart) < 1 || len(fractionPart) > 3)) {
		return 0, false
	}
	for _, r := range secondsPart + fractionPart {
		if r < '0' || r > '9' {
			return 0, false
		}
	}
	seconds, err := strconv.Atoi(secondsPart)
	if err != nil || seconds < 0 || seconds >= 60 {
		return 0, false
	}

	milliseconds := 0
	if hasFraction {
		fraction, err := strconv.Atoi(fractionPart + strings.Repeat("0", 3-len(fractionPart)))
		if err != nil {
			return 0, false
		}
		milliseconds = fraction
	}

	const maxDuration = int64(^uint64(0) >> 1)
	remainder := int64(seconds)*int64(time.Second) + int64(milliseconds)*int64(time.Millisecond)
	if minutes > (maxDuration-remainder)/int64(time.Minute) {
		return 0, false
	}
	return time.Duration(minutes*int64(time.Minute) + remainder), true
}
