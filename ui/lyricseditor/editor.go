package lyricseditor

import (
	"errors"
	"strconv"
	"strings"

	coreglib "github.com/diamondburned/gotk4/pkg/core/glib"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"

	"github.com/f1nniboy/chorus/internal/locale"
	"github.com/f1nniboy/chorus/internal/lyrics"
)

type Editor struct {
	window      *gtk.Window
	text        *gtk.TextView
	lineNumbers *gtk.TextView
	error       *gtk.Label
	errorTag    *gtk.TextTag
	numberTag   *gtk.TextTag
	onSave      func(string) error
}

func New(parent *gtk.Window, initial string, onSave func(string) error) *Editor {
	window := gtk.NewWindow()
	window.SetTitle(locale.Get("Edit Lyrics"))
	window.SetTransientFor(parent)
	window.SetModal(true)
	window.SetDefaultSize(800, 520)

	editor := &Editor{window: window, onSave: onSave}

	content := gtk.NewBox(gtk.OrientationVertical, 12)
	content.SetHExpand(true)
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
	editor.text.SetWrapMode(gtk.WrapNone)
	editor.text.SetHExpand(true)
	editor.text.Buffer().SetText(initial)

	editor.lineNumbers = gtk.NewTextView()
	editor.lineNumbers.SetMonospace(true)
	editor.lineNumbers.SetEditable(false)
	editor.lineNumbers.SetCursorVisible(false)
	editor.lineNumbers.SetCanFocus(false)
	editor.lineNumbers.SetWrapMode(gtk.WrapNone)
	editor.lineNumbers.SetLeftMargin(0)
	editor.lineNumbers.SetRightMargin(6)
	editor.lineNumbers.AddCSSClass("dim-label")

	editor.errorTag = newErrorTag(editor.text.Buffer(), "chorus-editor-error-text")
	editor.numberTag = newErrorTag(editor.lineNumbers.Buffer(), "chorus-editor-error-number")
	editor.updateLineNumbers()
	editor.text.Buffer().ConnectChanged(func() {
		editor.clearError()
		editor.updateLineNumbers()
	})

	lineNumberScroll := gtk.NewScrolledWindow()
	lineNumberScroll.SetPolicy(gtk.PolicyNever, gtk.PolicyNever)
	lineNumberScroll.SetVExpand(true)
	lineNumberScroll.SetHExpand(false)
	lineNumberScroll.SetChild(editor.lineNumbers)

	scroll := gtk.NewScrolledWindow()
	scroll.SetPolicy(gtk.PolicyAutomatic, gtk.PolicyAutomatic)
	scroll.SetVExpand(true)
	scroll.SetHExpand(true)
	scroll.SetMinContentWidth(400)
	scroll.SetChild(editor.text)

	gutterAndText := gtk.NewBox(gtk.OrientationHorizontal, 8)
	gutterAndText.SetHExpand(true)
	gutterAndText.Append(lineNumberScroll)
	gutterAndText.Append(scroll)
	content.Append(gutterAndText)

	lineNumbersAdjustment := lineNumberScroll.VAdjustment()
	textAdjustment := scroll.VAdjustment()
	textAdjustment.ConnectValueChanged(func() {
		if lineNumbersAdjustment.Value() != textAdjustment.Value() {
			lineNumbersAdjustment.SetValue(textAdjustment.Value())
		}
	})
	lineNumbersAdjustment.ConnectValueChanged(func() {
		if textAdjustment.Value() != lineNumbersAdjustment.Value() {
			textAdjustment.SetValue(lineNumbersAdjustment.Value())
		}
	})

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
		editor.clearError()
		editor.error.SetText(err.Error())
		editor.error.SetVisible(true)
		var editErr *lyrics.EditError
		if errors.As(err, &editErr) {
			editor.highlightLine(editErr.Line)
		}
		return
	}
	editor.window.Close()
}

func (editor *Editor) updateLineNumbers() {
	count := editor.text.Buffer().LineCount()
	digits := len(strconv.Itoa(count))
	editor.lineNumbers.SetSizeRequest(10+digits*10, -1)

	var numbers strings.Builder
	for i := 1; i <= count; i++ {
		if i > 1 {
			numbers.WriteByte('\n')
		}
		numbers.WriteString(strconv.Itoa(i))
	}
	editor.lineNumbers.Buffer().SetText(numbers.String())
}

func (editor *Editor) highlightLine(line int) {
	line--
	textBuffer := editor.text.Buffer()
	numberBuffer := editor.lineNumbers.Buffer()
	textStart, ok := textBuffer.IterAtLine(line)
	if !ok {
		return
	}
	textEnd := textStart.Copy()
	if !textEnd.ForwardLine() {
		textEnd = textBuffer.EndIter()
	}
	textBuffer.ApplyTag(editor.errorTag, textStart, textEnd)
	editor.text.ScrollToIter(textStart, 0.1, false, 0, 0)

	numberStart, ok := numberBuffer.IterAtLine(line)
	if !ok {
		return
	}
	numberEnd := numberStart.Copy()
	if !numberEnd.ForwardLine() {
		numberEnd = numberBuffer.EndIter()
	}
	numberBuffer.ApplyTag(editor.numberTag, numberStart, numberEnd)
}

func (editor *Editor) clearError() {
	editor.error.SetVisible(false)
	textBuffer := editor.text.Buffer()
	textBuffer.RemoveTag(editor.errorTag, textBuffer.StartIter(), textBuffer.EndIter())
	numberBuffer := editor.lineNumbers.Buffer()
	numberBuffer.RemoveTag(editor.numberTag, numberBuffer.StartIter(), numberBuffer.EndIter())
}

func newErrorTag(buffer *gtk.TextBuffer, name string) *gtk.TextTag {
	tag := gtk.NewTextTag(name)
	coreglib.BaseObject(tag).SetObjectProperty("background", "rgba(255, 0, 0, 0.18)")
	buffer.TagTable().Add(tag)
	return tag
}
