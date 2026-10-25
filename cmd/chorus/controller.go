package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/diamondburned/gotk4/pkg/glib/v2"

	"github.com/f1nniboy/chorus/internal/cache"
	"github.com/f1nniboy/chorus/internal/config"
	"github.com/f1nniboy/chorus/internal/lyrics"
	"github.com/f1nniboy/chorus/internal/mpris"
	"github.com/f1nniboy/chorus/internal/providers"
	"github.com/f1nniboy/chorus/ui/lyricsview"
)

const fetchTimeout = 30 * time.Second

type controller struct {
	cfg               *config.Config
	httpClient        *http.Client
	diskCache         *cache.Cache
	view              *lyricsview.View
	mgr               *mpris.Manager
	fetcher           atomic.Pointer[lyrics.Fetcher]
	cancel            context.CancelFunc
	playback          mpris.Playback
	generation        uint64
	result            lyrics.Result
	hasResult         bool
	onEditAvailable   func(bool)
}

func newController(cfg *config.Config, httpClient *http.Client, diskCache *cache.Cache, view *lyricsview.View, mgr *mpris.Manager) (*controller, error) {
	name := cfg.ProviderName()
	p, err := providers.New(name, cfg.ProviderConfig(), httpClient)
	if err != nil {
		return nil, err
	}
	c := &controller{
		cfg:        cfg,
		httpClient: httpClient,
		diskCache:  diskCache,
		view:       view,
		mgr:        mgr,
	}
	c.fetcher.Store(lyrics.NewFetcher(p, diskCache))
	c.watchConfig()
	return c, nil
}

func (c *controller) watchConfig() {
	c.cfg.ConnectChanged(func(key string) {
		if key == "provider" || key == "provider-config" {
			c.rebuildProvider()
		}
	})
}

func (c *controller) SeekTo(pos time.Duration) {
	if err := c.mgr.SeekTo(pos); err != nil {
		slog.Warn("mpris: seek failed", "err", err)
	}
}

func (c *controller) rebuildProvider() {
	name := c.cfg.ProviderName()
	cfg := c.cfg.ProviderConfig()

	go func() {
		p, err := providers.New(name, cfg, c.httpClient)
		if err != nil {
			slog.Error("providers: rebuild failed", "err", err)
			return
		}

		glib.IdleAdd(func() {
			c.fetcher.Store(lyrics.NewFetcher(p, c.diskCache))
			// a different provider may have different lyrics for the same track
			if c.playback.Track.Key() != "" {
				c.fetch(c.playback.Track, false)
			} else {
				c.generation++
				c.clearEditableResult()
			}
		})
	}()
}

func (c *controller) ChangeTrack(pb mpris.Playback) {
	changed := pb.Track.Key() != c.playback.Track.Key()
	c.playback = pb
	if changed {
		c.fetch(pb.Track, false)
	}
}

func (c *controller) RefetchLyrics() {
	if c.playback.Track.Key() == "" {
		return
	}
	c.fetch(c.playback.Track, true)
}

func (c *controller) UpdatePosition(pos time.Duration) {
	c.playback.Position = pos
	c.view.SetPosition(pos)
}

func (c *controller) Idle() {
	c.playback = mpris.Playback{}
	c.generation++
	c.clearEditableResult()
	if c.cancel != nil {
		c.cancel()
	}
	c.view.SetIdle()
}

func (c *controller) fetch(track mpris.Track, refetch bool) {
	c.generation++
	generation := c.generation
	c.clearEditableResult()
	c.view.SetLoading()

	if c.cancel != nil {
		c.cancel()
	}

	ctx, cancel := context.WithTimeout(context.Background(), fetchTimeout)
	c.cancel = cancel

	go func() {
		defer cancel()
		f := c.fetcher.Load()

		query := lyrics.TrackQuery{
			Artist:   track.Artist,
			Title:    track.Title,
			Album:    track.Album,
			Duration: track.Length,
		}
		var res lyrics.Result
		var err error
		if refetch {
			res, err = f.Refetch(ctx, query)
		} else {
			res, err = f.Get(ctx, query)
		}

		glib.IdleAdd(func() {
			if errors.Is(err, context.Canceled) {
				return
			}
			if c.playback.Track.Key() != track.Key() || c.generation != generation {
				return
			}
			if err == nil && !res.Instrumental && len(res.Lines) > 0 {
				if _, editErr := lyrics.TextForEdit(res); editErr != nil {
					slog.Warn("lyrics: result cannot be opened for editing", "err", editErr)
				} else {
					c.result = res
					c.hasResult = true
					c.notifyEditAvailable(true)
				}
			}
			c.view.SetResult(res, err, c.playback)
		})
	}()
}

func (c *controller) OnEditAvailable(callback func(bool)) {
	c.onEditAvailable = callback
	c.notifyEditAvailable(c.playback.Track.Key() != "")
}

func (c *controller) EditableLyrics() (string, lyrics.Level, string, uint64, error) {
	trackKey := c.playback.Track.Key()
	if trackKey == "" {
		return "", "", "", 0, errors.New("no track is available to edit")
	}
	if !c.hasResult {
		return "", lyrics.LevelNone, trackKey, c.generation, nil
	}
	text, err := lyrics.TextForEdit(c.result)
	if err != nil {
		return "", "", "", 0, err
	}
	return text, c.result.Level, trackKey, c.generation, nil
}

func (c *controller) SaveEditedLyrics(trackKey string, generation uint64, level lyrics.Level, text string) error {
	if trackKey == "" || c.playback.Track.Key() != trackKey || c.generation != generation {
		return errors.New("the track or lyrics changed while the editor was open; reopen the editor to apply your changes")
	}

	result, err := lyrics.ParseEditedText(text, level, c.playback.Track.Length)
	if err != nil {
		return err
	}

	query := lyrics.TrackQuery{
		Artist:   c.playback.Track.Artist,
		Title:    c.playback.Track.Title,
		Album:    c.playback.Track.Album,
		Duration: c.playback.Track.Length,
	}
	if err := c.fetcher.Load().Save(query, result); err != nil {
		return err
	}

	c.generation++
	c.result = result
	c.hasResult = true
	c.view.SetResult(result, nil, c.playback)
	return nil
}

func (c *controller) clearEditableResult() {
	c.result = lyrics.Result{}
	c.hasResult = false
	c.notifyEditAvailable(c.playback.Track.Key() != "")
}

func (c *controller) notifyEditAvailable(available bool) {
	if c.onEditAvailable != nil {
		c.onEditAvailable(available)
	}
}
