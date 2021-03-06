package main

import "log"

import "github.com/nsf/termbox-go"
import "github.com/bpowers/go-tmux"

const (
	EvTermbox EventType = iota
	EvTmux
)

type EventType int

// FIXME: this is :(  refactor to use interfaces
type Event struct {
	Ev EventType
	termbox.Event
	MousePos Point
	Window   tmux.Window
}

// Represent a position in terms of the terminal's columns (X) and
// rows (Y)
type Point struct {
	X int
	Y int
}

func (a Point) Add(b Point) Point {
	return Point{a.X + b.X, a.Y + b.Y}
}

func (a Point) Sub(b Point) Point {
	return Point{a.X - b.X, a.Y - b.Y}
}

// Shorthand for Point{X, Y}
func P(x, y int) Point {
	return Point{x, y}
}

// Represent the size of a UI element in terms of the terminal's
// columns (Width) and rows (Height)
type Size struct {
	Width  int
	Height int
}

// Returns true if _either_ the width or height is zero.  If either
// are zero, we can't be expected to see any output in this region.
func (s Size) Empty() bool {
	return s.Width == 0 || s.Height == 0
}

type Rect struct {
	Point
	Size
}

func NewRect(x, y, w, h int) Rect {
	return Rect{Point{x, y}, Size{w, h}}
}

func (r Rect) Contains(p Point) bool {
	return r.X <= p.X && r.Y <= p.Y &&
		p.X < r.X+r.Width && p.Y < r.Y+r.Height
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
	NeedsResize() bool
	Resize(available Rect) (desired Rect)
	NeedsDisplay() bool
	Draw(View)         // Draw yourself into this view
	Handle(Event) bool // Event's MousePos is Box-relative
}

type BoundedBox struct {
	Bounds Rect
	Box    Box
}

// A view is a bounded subview of the terminal we're allowed to draw
// into, along with convienence methods for drawing.  Prefer this to
// termbox primitives.
type View struct {
	bounds Rect // bounds we're allowed to draw in
}

func (v *View) Subview(bounds Rect) View {
	return View{Rect{v.bounds.Point.Add(bounds.Point), bounds.Size}}
}

// allow intuitive use of negative offsets
func (v *View) normalize(pos Point) Point {
	var result Point
	if pos.X >= 0 {
		result.X = v.bounds.X + pos.X
	} else {
		result.X = v.bounds.X + v.bounds.Width + pos.X
	}
	if pos.Y >= 0 {
		result.Y = v.bounds.Y + pos.Y
	} else {
		result.Y = v.bounds.Y + v.bounds.Height + pos.Y
	}
	return result
}

func (v *View) SetCell(relPos Point, ch rune, fg, bg termbox.Attribute) {
	pos := v.normalize(relPos)
	if !v.bounds.Contains(pos) {
		log.Printf("SetCell(%#v, %s) OOB: %#v", relPos, ch, v.bounds)
		return
	}
	termbox.SetCell(pos.X, pos.Y, ch, fg, bg)
}

func (v *View) String(relPos Point, fg, bg termbox.Attribute, msg string) {
	for i, c := range msg {
		v.SetCell(P(relPos.X+i, relPos.Y), c, fg, bg)
	}
}

type Element struct {
	size Size
	// if accessed from multiple goroutines, must be atomic
	needsDisplay bool
	needsResize  bool
}

type Container struct {
	Element
	children []BoundedBox
}

func (e *Element) Size() Size {
	return e.size
}

func (e *Element) NeedsDisplay() bool {
	return e.needsDisplay
}

func (e *Element) NeedsResize() bool {
	return e.needsResize
}

// A container fills up all available size.  If you want a header +
// footer + flex content in the middle, add the header and footer
// first to the parent.
func (e *Container) Resize(available Rect) (desired Rect) {
	e.needsResize = false
	e.size = available.Size

	// children's Rects are relatively positioned inside the
	// parent.
	relBounds := Rect{Size: e.Size()}
	for i, c := range e.children {
		childBounds := c.Box.Resize(relBounds)
		e.children[i].Bounds = childBounds
		relBounds = relBounds.Sub(childBounds)
	}

	return Rect{available.Point, e.Size()}
}

func (e *Container) Draw(view View) {
	for _, c := range e.children {
		c.Box.Draw(view.Subview(c.Bounds))
	}
}

func (e *Container) Handle(ev Event) bool {
	for _, c := range e.children {
		if ev.Type == termbox.EventMouse {
			if !c.Bounds.Contains(ev.MousePos) {
				continue
			}
			// make position relative to this subview
			ev.MousePos = ev.MousePos.Sub(c.Bounds.Point)
		}
		if c.Box.Handle(ev) {
			return true
		}
	}
	return false
}

func (e *Container) AddChild(child Box) {
	e.children = append(e.children, BoundedBox{Box: child})
}

func (e *Container) NeedsResize() bool {
	if e.needsResize {
		return true
	}
	for _, c := range e.children {
		if c.Box.NeedsResize() {
			return true
		}
	}
	return false
}

func (e *Container) NeedsDisplay() bool {
	if e.needsDisplay {
		return true
	}
	for _, c := range e.children {
		if c.Box.NeedsDisplay() {
			return true
		}
	}
	return false
}

// something that allows subelements to be selected.
type Selector interface {
	Selection() int
	SetSelection(int)
	SetNamedSelection(string, bool) bool
	ClearSelection()
	SelectableCount() int
}

type SelectorsContainer struct {
	Container
}

func (e *SelectorsContainer) Handle(ev Event) bool {
	if ev.Type == termbox.EventKey &&
		(ev.Key == termbox.KeyArrowUp || ev.Key == termbox.KeyArrowDown) {
		var off int
		if ev.Key == termbox.KeyArrowUp {
			off = -1
		} else {
			off = 1
		}
		for i, child := range e.children {
			selector := child.Box.(Selector)

			j := selector.Selection()
			selector.ClearSelection()
			if j < 0 {
				continue
			}
			if j+off == selector.SelectableCount() {
				j = 0
				i = (i + 1) % len(e.children)
			} else if j+off < 0 {
				if i == 0 {
					i = len(e.children) - 1
				} else {
					i--
				}
				j = e.children[i].Box.(Selector).SelectableCount() - 1
			} else {
				j += off
			}
			// XXX: users are not allowed to close the
			// '#general' channel, so we will never have
			// all 3 sections (im, channels, groups)
			// empty, which means this will always
			// terminate.  If that changes, this logic
			// needs to change.
			for e.children[i].Box.(Selector).SelectableCount() == 0 {
				if i+off < 0 {
					i = len(e.children) - 1
				} else {
					i = (i + off) % len(e.children)
				}
				if off < 0 {
					j = e.children[i].Box.(Selector).SelectableCount() - 1
				} else {
					j = 0
				}
			}
			e.children[i].Box.(Selector).SetSelection(j)
			break
		}
		return true
	}
	return e.Container.Handle(ev)
}

func (e *SelectorsContainer) ClearSelection() {
	for _, child := range e.children {
		if selector, ok := child.Box.(Selector); ok {
			selector.ClearSelection()
		}
	}
}

type Window struct {
	Container
}

func (e *Window) Paint() {
	e.needsDisplay = false

	w, h := termbox.Size()
	bounds := NewRect(0, 0, w, h)

	termbox.Clear(fgColor, bgColor)
	e.Container.Draw(View{bounds})
	termbox.Flush()
}

func (e *Window) Handle(ev Event) bool {
	if ev.Type == termbox.EventResize {
		e.needsDisplay = true
		e.Resize(NewRect(0, 0, ev.Width, ev.Height))
		return true
	}
	// set MousePos in the window, so that sub containers can
	// always use it.
	ev.MousePos = P(ev.MouseX, ev.MouseY)
	return e.Container.Handle(ev)
}
