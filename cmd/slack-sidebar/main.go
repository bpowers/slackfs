package main

import (
	"log"
	"os"

	"github.com/nsf/termbox-go"
)

// Represent a position in terms of the terminal's columns (X) and
// rows (Y)
type Point struct {
	X int
	Y int
}

func (a Point) Add(b Point) Point {
	return Point{a.X + b.X, a.Y + b.Y}
}

// Represent the size of a UI element in terms of the terminal's
// columns (Width) and rows (Height)
type Size struct {
	Width  int
	Height int
}

type Rect struct {
	Point
	Size
}

func NewRect(x, y, w, h int) Rect {
	return Rect{Point{x, y}, Size{w, h}}
}

func (a Rect) Sub(b Rect) Rect {
	// FIXME: this isn't general, it only works for the code I've
	// written here.
	if a.Y == b.Y {
		return NewRect(a.X, a.Y+b.Height, a.Width, a.Height-b.Height)
	} else if a.Y+a.Height == b.Y+b.Height {
		return NewRect(a.X, a.Y, a.Width, a.Height-b.Height)
	}
	return a
}

type Box interface {
	Size() Size
	Resize(available Rect) (desired Rect)
	NeedsDisplay() bool
	Draw(bounds Rect)          // Draw yourself in these bounds
	Handle(termbox.Event) bool // Event's MouseX/Y is Box-relative
}

type BoundedBox struct {
	Bounds Rect
	Box    Box
}

type Element struct {
	size   Size
	parent *Element
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

func (e *Element) Size() Size {
	return e.size
}

func (e *Element) NeedsDisplay() bool {
	return e.needsDisplay
}

// A container fills up all available size.  If you want a header +
// footer + flex content in the middle, add the header and footer
// first to the parent.
func (e *Container) Resize(available Rect) (desired Rect) {
	e.size = available.Size

	// children's Rects are relatively positioned inside the
	// parent.
	relBounds := Rect{Size: e.Size()}
	for i, c := range e.children {
		childBounds := c.Box.Resize(relBounds)
		e.children[i].Bounds = childBounds
		relBounds = relBounds.Sub(childBounds)
	}

	return Rect{desired.Point, e.Size()}
}

func (e *Container) Draw(bounds Rect) {
	for _, c := range e.children {
		c.Box.Draw(c.Bounds)
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
	bounds := Rect{Point{0, 0}, Size{w, h}}

	termbox.Clear(fgColor, bgColor)
	e.Container.Draw(bounds)
	termbox.Flush()
}

func (e *Window) Handle(ev termbox.Event) bool {
	if ev.Type == termbox.EventResize {
		e.needsDisplay = true
		e.Resize(Rect{Point{0, 0}, Size{ev.Width, ev.Height}})
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

func (e *Header) Resize(available Rect) (desired Rect) {
	e.size = Size{available.Width, 6}
	return Rect{available.Point, e.Size()}
}

func (e *Header) Draw(bounds Rect) {
	printString(bounds.Point, fgColor|bold, bgColor, "TEAM NAME")

	termbox.SetCell(0, 1, '●', termbox.ColorGreen, bgColor)
	printString(Point{2, 1}, fgColor, bgColor, "Bobby Powers")
	printString(Point{0, 3}, fgColor, bgColor, "CHANNELS")
	printString(Point{0, 4}, fgColor, bgColor, "#general")
	termbox.SetCell(19, 4, '✓', fgColor, bgColor)
	termbox.SetCell(22, 4, '⊗', fgColor, bgColor)
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

func (e *Footer) Draw(bounds Rect) {
	printString(bounds.Point, fgColor|bold, bgColor, "--+--")
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

func (e *Outline) Draw(bounds Rect) {
	termbox.SetCell(bounds.X, bounds.Y, '┌', fgColor, bgColor)
	termbox.SetCell(bounds.X, bounds.Y+bounds.Height-1, '└', fgColor, bgColor)
	termbox.SetCell(bounds.X+bounds.Width-1, bounds.Y, '┐', fgColor, bgColor)
	termbox.SetCell(bounds.X+bounds.Width-1, bounds.Y+bounds.Height-1, '┘', fgColor, bgColor)
}

const fgColor = termbox.ColorDefault
const bgColor = termbox.ColorDefault
const bold = termbox.AttrBold

func printString(pos Point, fg, bg termbox.Attribute, msg string) {
	for i, c := range msg {
		termbox.SetCell(pos.X+i, pos.Y, c, fg, bg)
	}
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
	size := Size{w, h}
	window.Resize(Rect{Point{0, 0}, size})

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
