package lrclib

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/f1nniboy/chorus/internal/lyrics"
	"github.com/f1nniboy/chorus/internal/providers/base"
)

type Provider struct {
	base.Common

	BaseURL string `config:"base_url" default:"https://lrclib.net/api" label:"Base URL"`
}

func (p *Provider) ID() string   { return "lrclib" }
func (p *Provider) Name() string { return "LRCLIB" }

func (p *Provider) Init() {
	p.BaseURL = strings.TrimRight(p.BaseURL, "/")
}

func (p *Provider) Fetch(ctx context.Context, q lyrics.TrackQuery) (lyrics.Result, error) {
	u, err := url.Parse(p.BaseURL + "/get")
	if err != nil {
		return lyrics.Result{}, fmt.Errorf("lrclib: invalid base URL: %w", err)
	}

	query := u.Query()
	query.Set("artist_name", q.Artist)
	query.Set("track_name", q.Title)
	if q.Album != "" {
		query.Set("album_name", q.Album)
	}
	if q.Duration > 0 {
		query.Set("duration", strconv.FormatFloat(q.Duration.Seconds(), 'f', -1, 64))
	}
	u.RawQuery = query.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return lyrics.Result{}, fmt.Errorf("lrclib: request: %w", err)
	}

	resp, err := p.HTTP.Do(req)
	if err != nil {
		return lyrics.Result{}, fmt.Errorf("lrclib: fetch: %w", err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
	case http.StatusNotFound:
		return lyrics.Result{}, lyrics.ErrNotFound
	case http.StatusTooManyRequests:
		return lyrics.Result{}, fmt.Errorf("lrclib: rate limited (retry after %s)", resp.Header.Get("Retry-After"))
	default:
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 256))
		return lyrics.Result{}, fmt.Errorf("lrclib: HTTP %d: %s", resp.StatusCode, body)
	}

	var parsed response
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return lyrics.Result{}, fmt.Errorf("lrclib: decode: %w", err)
	}

	result := parsed.toResult(q.Duration)
	if !result.Instrumental && len(result.Lines) == 0 {
		return lyrics.Result{}, lyrics.ErrNotFound
	}
	return result, nil
}

type response struct {
	Instrumental bool   `json:"instrumental"`
	PlainLyrics  string `json:"plainLyrics"`
	SyncedLyrics string `json:"syncedLyrics"`
}

func (r response) toResult(duration time.Duration) lyrics.Result {
	if r.Instrumental {
		return lyrics.Result{Instrumental: true}
	}
	if lines := parseSyncedLyrics(r.SyncedLyrics, duration); len(lines) > 0 {
		return lyrics.Result{Level: lyrics.LevelLine, Lines: lines}
	}
	if r.PlainLyrics != "" {
		lines := strings.Split(strings.TrimSpace(r.PlainLyrics), "\n")
		result := make([]lyrics.Line, 0, len(lines))
		for _, line := range lines {
			result = append(result, lyrics.Line{Text: strings.TrimSuffix(line, "\r")})
		}
		return lyrics.Result{Level: lyrics.LevelNone, Lines: result}
	}
	return lyrics.Result{}
}

func parseSyncedLyrics(raw string, duration time.Duration) []lyrics.Line {
	var lines []lyrics.Line
	for _, rawLine := range strings.Split(raw, "\n") {
		remaining := strings.TrimSuffix(rawLine, "\r")
		var starts []time.Duration
		for strings.HasPrefix(remaining, "[") {
			close := strings.IndexByte(remaining, ']')
			if close < 0 {
				break
			}
			start, ok := parseTimestamp(remaining[1:close])
			if !ok {
				break
			}
			starts = append(starts, start)
			remaining = remaining[close+1:]
		}
		if len(starts) == 0 {
			continue
		}
		text := strings.TrimSpace(remaining)
		for _, start := range starts {
			lines = append(lines, lyrics.Line{Text: text, Start: start})
		}
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
	return lines
}

func parseTimestamp(raw string) (time.Duration, bool) {
	minutePart, secondPart, ok := strings.Cut(raw, ":")
	if !ok {
		return 0, false
	}
	minutes, err := strconv.Atoi(minutePart)
	if err != nil || minutes < 0 {
		return 0, false
	}

	secondsPart, fraction, hasFraction := strings.Cut(secondPart, ".")
	if !hasFraction {
		secondsPart, fraction, hasFraction = strings.Cut(secondPart, ":")
	}
	seconds, err := strconv.Atoi(secondsPart)
	if err != nil || seconds < 0 || seconds >= 60 {
		return 0, false
	}

	var fractionDuration time.Duration
	if hasFraction {
		if fraction == "" || len(fraction) > 3 {
			return 0, false
		}
		fractionValue, err := strconv.Atoi(fraction)
		if err != nil {
			return 0, false
		}
		switch len(fraction) {
		case 1:
			fractionValue *= 100
		case 2:
			fractionValue *= 10
		}
		fractionDuration = time.Duration(fractionValue) * time.Millisecond
	}

	return time.Duration(minutes)*time.Minute + time.Duration(seconds)*time.Second + fractionDuration, true
}
