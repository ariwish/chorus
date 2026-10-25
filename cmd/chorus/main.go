package main

import (
	"context"
	"io/fs"
	"log"
	"log/slog"
	"os"
	"time"

	"github.com/diamondburned/gotk4-adwaita/pkg/adw"
	"github.com/diamondburned/gotk4/pkg/gdk/v4"
	"github.com/diamondburned/gotk4/pkg/gio/v2"
	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/godbus/dbus/v5"

	"github.com/f1nniboy/chorus/data"
	"github.com/f1nniboy/chorus/internal/art"
	"github.com/f1nniboy/chorus/internal/cache"
	"github.com/f1nniboy/chorus/internal/config"
	"github.com/f1nniboy/chorus/internal/locale"
	"github.com/f1nniboy/chorus/internal/meta"
	"github.com/f1nniboy/chorus/internal/mpris"
	"github.com/f1nniboy/chorus/internal/providers/base"
	"github.com/f1nniboy/chorus/ui/about"
	"github.com/f1nniboy/chorus/ui/lyricseditor"
	"github.com/f1nniboy/chorus/ui/settings"
	"github.com/f1nniboy/chorus/ui/window"
)

const (
	httpClientTimeout = 15 * time.Second
	positionPollMs    = 100
)

//go:generate glib-compile-schemas ../../data
//go:generate go run ../potgen

func init() {
	po, _ := fs.Sub(data.PO, "po")
	locale.Load(po)
}

func main() {
	app := adw.NewApplication(meta.AppID, 0)

	app.ConnectActivate(func() {
		if win := app.ActiveWindow(); win != nil {
			win.Present()
			return
		}

		cfg, err := config.New()
		if err != nil {
			log.Fatal(err)
		}

		cssProvider := gtk.NewCSSProvider()
		cssProvider.LoadFromString(string(data.CSS))
		gtk.StyleContextAddProviderForDisplay(gdk.DisplayGetDefault(), cssProvider, gtk.STYLE_PROVIDER_PRIORITY_APPLICATION)

		ca, err := cache.New()
		if err != nil {
			log.Fatal(err)
		}

		httpClient := base.NewClient(httpClientTimeout)
		artResolver := art.New(httpClient, ca)

		win := window.New(app, cfg, artResolver)

		aboutAction := gio.NewSimpleAction("about", nil)
		aboutAction.ConnectActivate(func(_ *glib.Variant) {
			about.New().Present(win)
		})
		app.AddAction(aboutAction)

		conn, err := dbus.SessionBus()
		if err != nil {
			log.Fatal(err)
		}
		mgr := mpris.New(conn)

		controller, err := newController(cfg, httpClient, ca, win.Lyrics, mgr)
		if err != nil {
			log.Fatal(err)
		}
		updateRomaji := func() {
			if err := win.Lyrics.SetRomaji(cfg.Boolean("romaji")); err != nil {
				slog.Error("lyrics: cannot initialize romaji", "err", err)
			}
		}
		updateRomaji()
		cfg.ConnectChanged(func(key string) {
			if key == "romaji" {
				updateRomaji()
			}
		})

		refetchAction := gio.NewSimpleAction("refetch-lyrics", nil)
		refetchAction.SetEnabled(false)
		refetchAction.ConnectActivate(func(_ *glib.Variant) {
			controller.RefetchLyrics()
		})
		app.AddAction(refetchAction)

		editLyricsAction := gio.NewSimpleAction("edit-lyrics", nil)
		editLyricsAction.SetEnabled(false)
		editLyricsAction.ConnectActivate(func(_ *glib.Variant) {
			text, level, trackKey, generation, err := controller.EditableLyrics()
			if err != nil {
				slog.Error("lyrics: cannot open editor", "err", err)
				return
			}
			editor := lyricseditor.New(
				&win.ApplicationWindow.ApplicationWindow.Window,
				text,
				func(text string) error {
					return controller.SaveEditedLyrics(trackKey, generation, level, text)
				},
			)
			editor.Present()
		})
		app.AddAction(editLyricsAction)
		controller.OnEditAvailable(editLyricsAction.SetEnabled)

		prefs := settings.New(cfg)
		preferencesAction := gio.NewSimpleAction("preferences", nil)
		preferencesAction.ConnectActivate(func(_ *glib.Variant) {
			prefs.Present(win)
		})
		app.AddAction(preferencesAction)

		quitAction := gio.NewSimpleAction("quit", nil)
		quitAction.ConnectActivate(func(_ *glib.Variant) {
			app.Quit()
		})
		app.AddAction(quitAction)

		app.SetAccelsForAction("app.refetch-lyrics", []string{"<Primary>r"})
		app.SetAccelsForAction("app.edit-lyrics", []string{"<Primary>e"})
		app.SetAccelsForAction("app.preferences", []string{"<Primary>comma"})
		app.SetAccelsForAction("app.quit", []string{"<Primary>q"})

		win.Header.Picker.OnSelect(mgr.SelectPlayer)
		win.Lyrics.OnSeek(controller.SeekTo)

		glib.TimeoutAdd(positionPollMs, func() bool {
			controller.UpdatePosition(mgr.CurrentPosition())
			return true
		})

		go func() {
			for {
				select {
				case entries := <-mgr.Roster():
					glib.IdleAdd(func() {
						win.Header.Picker.SetRoster(entries)
					})

				case pb := <-mgr.Playback():
					glib.IdleAdd(func() {
						win.Header.Picker.SetCurrent(pb.Player.BusName)
						refetchAction.SetEnabled(!pb.IsIdle())

						if pb.IsIdle() {
							controller.Idle()
							win.Background.SetArtURL("")
							return
						}

						win.Background.SetArtURL(pb.Track.ArtURL)
						controller.ChangeTrack(pb)
					})
				}
			}
		}()

		go func() {
			if err := mgr.Start(context.Background()); err != nil {
				slog.Error("mpris: stopped", "err", err)
			}
		}()

		win.Present()
	})

	os.Exit(app.Run(os.Args))
}
