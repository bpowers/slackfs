package main

import (
	"log"
	"os"

	"github.com/nsf/termbox-go"
)

const fgColor = termbox.ColorDefault
const bgColor = termbox.ColorDefault
const bold = termbox.AttrBold

type Header struct {
	Element
}

func (e *Header) Handle(ev termbox.Event) bool {
	return false
}

func (e *Header) Resize(available Rect) (desired Rect) {
	e.size = Size{available.Width, 6}
	return Rect{available.Point, e.Size()}
}

func (e *Header) Draw(view View) {
	view.String(P(0, 0), fgColor|bold, bgColor, "TEAM NAME")

	view.SetCell(P(0, 1), '●', termbox.ColorGreen, bgColor)
	view.String(P(2, 1), fgColor, bgColor, "Bobby Powers")
	view.String(P(0, 3), fgColor, bgColor, "CHANNELS")
	view.String(P(0, 4), fgColor, bgColor, "#general")
	view.SetCell(P(19, 4), '✓', fgColor, bgColor)
	view.SetCell(P(22, 4), '⊗', fgColor, bgColor)
}

type Footer struct {
	Element
}

func (e *Footer) Handle(ev termbox.Event) bool {
	return false
}

func (e *Footer) Resize(available Rect) (desired Rect) {
	e.size = Size{available.Width, 1}
	pos := available.Point
	pos.Y += available.Height - 1
	return Rect{pos, e.Size()}
}

func (e *Footer) Draw(view View) {
	view.String(P(0, 0), fgColor|bold, bgColor, "--+--")
}

type Outline struct {
	Element
}

func (e *Outline) Handle(ev termbox.Event) bool {
	return false
}

func (e *Outline) Resize(available Rect) (desired Rect) {
	e.size = available.Size
	return Rect{available.Point, e.Size()}
}

func (e *Outline) Draw(view View) {
	view.SetCell(P(0, 0), '┌', fgColor, bgColor)
	view.SetCell(P(0, -1), '└', fgColor, bgColor)
	view.SetCell(P(-1, 0), '┐', fgColor, bgColor)
	view.SetCell(P(-1, -1), '┘', fgColor, bgColor)
}

func main() {
	f, err := os.Create("log")
	if err != nil {
		log.Fatalf("couldn't open log for writing: %s", err)
	}
	log.SetOutput(f)
	defer f.Close()

	err = termbox.Init()
	if err != nil {
		log.Fatalf("termbox.Init: %s", err)
	}
	defer termbox.Close()
	termbox.SetInputMode(termbox.InputEsc | termbox.InputMouse)

	// ensure we're running under tmux
	// ensure slackfs is running

	window := new(Window)

	// build tree of UI components
	window.AddChild(new(Header))
	window.AddChild(new(Footer))
	window.AddChild(new(Outline))

	w, h := termbox.Size()
	window.Resize(NewRect(0, 0, w, h))

	window.Paint()

	for {
		ev := termbox.PollEvent()

		// quit on escape or 'q'
		if ev.Type == termbox.EventKey && (ev.Key == termbox.KeyEsc || ev.Ch == 'q') {
			break
		}

		window.Handle(ev)
		if window.NeedsResize() {
			window.Resize(Rect{Point{0, 0}, window.Size()})
		}

		if window.NeedsDisplay() {
			window.Paint()
		}
	}
}
