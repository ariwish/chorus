package lyricseditor

import (
	"github.com/diamondburned/gotk4/pkg/gtk/v4"

	"github.com/f1nniboy/chorus/internal/locale"
)

type Editor struct {
	window *gtk.Window
	text   *gtk.TextView
	error  *gtk.Label
	onSave func(string) error
}

func New(parent *gtk.Window, initial string, onSave func(string) error) *Editor {
	window := gtk.NewWindow()
	window.SetTitle(locale.Get("Edit Lyrics"))
	window.SetTransientFor(parent)
	window.SetModal(true)
	window.SetDefaultSize(640, 480)

	editor := &Editor{window: window, onSave: onSave}

	content := gtk.NewBox(gtk.OrientationVertical, 12)
	content.SetMarginTop(18)
	content.SetMarginBottom(18)
	content.SetMarginStart(18)
	content.SetMarginEnd(18)

	note := gtk.NewLabel(locale.Get("For synced lyrics, keep each [mm:ss.xxx] timestamp at the start of its line."))
	note.SetWrap(true)
	note.SetHAlign(gtk.AlignStart)
	content.Append(note)

	editor.text = gtk.NewTextView()
	editor.text.SetMonospace(true)
	editor.text.SetWrapMode(gtk.WrapWordChar)
	editor.text.Buffer().SetText(initial)

	scroll := gtk.NewScrolledWindow()
	scroll.SetPolicy(gtk.PolicyAutomatic, gtk.PolicyAutomatic)
	scroll.SetVExpand(true)
	scroll.SetChild(editor.text)
	content.Append(scroll)

	editor.error = gtk.NewLabel("")
	editor.error.SetWrap(true)
	editor.error.SetHAlign(gtk.AlignStart)
	editor.error.AddCSSClass("error")
	editor.error.SetVisible(false)
	content.Append(editor.error)

	buttons := gtk.NewBox(gtk.OrientationHorizontal, 8)
	buttons.SetHAlign(gtk.AlignEnd)
	cancel := gtk.NewButtonWithLabel(locale.Get("Cancel"))
	cancel.ConnectClicked(window.Close)
	buttons.Append(cancel)

	save := gtk.NewButtonWithLabel(locale.Get("Save"))
	save.AddCSSClass("suggested-action")
	save.ConnectClicked(editor.save)
	buttons.Append(save)
	content.Append(buttons)

	window.SetChild(content)
	window.SetDefaultWidget(save)
	return editor
}

func (editor *Editor) Present() {
	editor.window.Present()
}

func (editor *Editor) save() {
	start, end := editor.text.Buffer().StartIter(), editor.text.Buffer().EndIter()
	text := editor.text.Buffer().Text(start, end, true)
	if err := editor.onSave(text); err != nil {
		editor.error.SetText(err.Error())
		editor.error.SetVisible(true)
		return
	}
	editor.window.Close()
}
