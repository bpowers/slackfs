package main

import (
	"log"

	"github.com/nsf/termbox-go"
)

// Represent a position in terms of the terminal's columns (X) and
// rows (Y)
type Pos struct {
	X int
	Y int
}

func (a Pos) Add(b Pos) Pos {
	return Pos{a.X + b.X, a.Y + b.Y}
}

// Represent the size of a UI element in terms of the terminal's
// columns (Width) and rows (Height)
type Size struct {
	Width  int
	Height int
}

type Rect struct {
	Pos
	Size
}

type Box interface {
	Bounds() Size
	Resize(available Size) (reserved Size)
	NeedsDisplay() bool
	Draw(bounds Rect)          // Draw yourself at this position offset
	Handle(termbox.Event) bool // Event's MouseX/Y is Box-relative
}

type BoundedBox struct {
	Rect
	Box Box
}

type Element struct {
	Size
	Parent *Element
	// if accessed from multiple goroutines, must be atomic
	needsDisplay bool
}

type Container struct {
	Element
	children  []BoundedBox
	canScroll bool // set only at initialization
}

type Window struct {
	Container
}

func (e *Element) Bounds() Size {
	return e.Size
}

func (e *Element) NeedsDisplay() bool {
	return e.needsDisplay
}

func (e *Container) Resize(available Size) (reserved Size) {
	// TODO: implement this.
	e.Size = available
	return e.Bounds()
}

func (e *Container) Draw(bounds Rect) {
	for _, c := range e.children {
		c.Box.Draw(bounds)
	}
}

func (e *Container) Handle(ev termbox.Event) bool {
	for _, c := range e.children {
		if c.Box.Handle(ev) {
			return true
		}
	}
	return false
}

func (e *Container) AddChild(child Box) {
	e.children = append(e.children, BoundedBox{Box: child})
}

func (e *Window) Paint() {
	w, h := termbox.Size()
	bounds := Rect{Pos{0, 0}, Size{w, h}}

	termbox.Clear(fgColor, bgColor)
	e.Container.Draw(bounds)
	termbox.Flush()
}

func (e *Window) Handle(ev termbox.Event) bool {
	if ev.Type == termbox.EventResize {
		e.needsDisplay = true
		// TODO: perform resize
		return true
	}
	return e.Container.Handle(ev)
}

type Header struct {
	Element
}

func (e *Header) Handle(ev termbox.Event) bool {
	return false
}

func (e *Header) Resize(available Size) (reserved Size) {
	e.Size = Size{available.Width, 4}
	return e.Bounds()
}

func (e *Header) Draw(bounds Rect) {
	printString(Pos{0, 0}, fgColor|bold, bgColor, "TEAM NAME")

	termbox.SetCell(0, 1, '●', termbox.ColorGreen, bgColor)
	printString(Pos{2, 1}, fgColor, bgColor, "Bobby Powers")
	printString(Pos{0, 3}, fgColor, bgColor, "CHANNELS")
	printString(Pos{0, 4}, fgColor, bgColor, "#general")
	termbox.SetCell(19, 4, '✓', fgColor, bgColor)
	termbox.SetCell(22, 4, '⊗', fgColor, bgColor)
}

const fgColor = termbox.ColorDefault
const bgColor = termbox.ColorDefault
const bold = termbox.AttrBold

func printString(pos Pos, fg, bg termbox.Attribute, msg string) {
	for i, c := range msg {
		termbox.SetCell(pos.X+i, pos.Y, c, fg, bg)
	}
}

func main() {
	err := termbox.Init()
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

	w, h := termbox.Size()
	size := Size{w, h}
	window.Resize(size)

	window.Paint()

	for {
		ev := termbox.PollEvent()

		// quit on escape or 'q'
		if ev.Type == termbox.EventKey && (ev.Key == termbox.KeyEsc || ev.Ch == 'q') {
			break
		}

		window.Handle(ev)
		if window.NeedsDisplay() {
			window.Paint()
		}
	}
}
