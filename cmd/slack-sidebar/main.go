package main

import (
	"bytes"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path"

	"github.com/nsf/termbox-go"
)

const usage = `Usage: %s [OPTION...] MOUNTPOINT
Sidebar controller for slack + tmux.

Mountpoint must be passed pointing at the root of the slackfs instance
we're to connect to.

Options:
`

var memProfile, cpuProfile string

const (
	fgColor = termbox.ColorDefault
	bgColor = termbox.ColorDefault
	bold    = termbox.AttrBold
)

func readFile(parts ...string) (string, error) {
	path := path.Join(parts...)
	contents, err := ioutil.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("ReadFile(%s): %s", path, err)
	}
	return string(bytes.TrimSpace(contents)), nil
}

type Header struct {
	Element
	mount string
	team  string
	user  string
}

func NewHeader(mountpoint string) (*Header, error) {
	user, err := readFile(mountpoint, "/self/user/name")
	if err != nil {
		return nil, err
	}
	team, err := readFile(mountpoint, "/self/team/name")
	if err != nil {
		return nil, err
	}
	return &Header{
		mount: mountpoint,
		user:  user,
		team:  team,
	}, nil
}

func (e *Header) Handle(ev Event) bool {
	return false
}

func (e *Header) Resize(available Rect) (desired Rect) {
	e.size = Size{available.Width, 3}
	return Rect{available.Point, e.Size()}
}

func (e *Header) Draw(view View) {
	view.String(P(0, 0), fgColor|bold, bgColor, e.team)

	view.SetCell(P(0, 1), '●', termbox.ColorGreen, bgColor)
	view.String(P(2, 1), fgColor, bgColor, e.user)

	//view.String(P(0, 3), fgColor, bgColor, "CHANNELS")
	//view.String(P(0, 4), fgColor, bgColor, "#general")
	//view.SetCell(P(19, 4), '✓', fgColor, bgColor)
	//view.SetCell(P(22, 4), '⊗', fgColor, bgColor)
}

type Footer struct {
	Element
	mount    string
	expanded bool
}

func NewFooter(mountpoint string) (*Footer, error) {
	return &Footer{mount: mountpoint}, nil
}

func (e *Footer) Handle(ev Event) bool {
	if ev.Type != termbox.EventMouse {
		return false
	}
	if ev.MousePos.Y == 0 {
		e.expanded = !e.expanded
		e.needsResize = true
		e.needsDisplay = true
	}
	return false
}

func (e *Footer) Resize(available Rect) (desired Rect) {
	e.needsResize = false
	height := 1
	if e.expanded {
		height += 4
	}
	e.size = Size{available.Width, height}
	pos := available.Point
	pos.Y += available.Height - height
	return Rect{pos, e.Size()}
}

func (e *Footer) Draw(view View) {
	e.needsDisplay = false
	s := "--+--"
	if e.expanded {
		s = "-"
	}
	view.String(P(0, 0), fgColor|bold, bgColor, s)
}

type Outline struct {
	Element
}

func (e *Outline) Handle(ev Event) bool {
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
	flag.StringVar(&memProfile, "memprofile", "",
		"write memory profile to this file")
	flag.StringVar(&cpuProfile, "cpuprofile", "",
		"write cpu profile to this file")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, usage, os.Args[0])
		flag.PrintDefaults()
	}
	flag.Parse()
	if flag.NArg() != 1 {
		flag.Usage()
		os.Exit(1)
	}

	mountpoint := flag.Arg(0)

	// FIXME: this is temporary
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
	// ensure slackfs is running?

	window := new(Window)

	// build tree of UI components
	header, err := NewHeader(mountpoint)
	if err != nil {
		log.Fatalf("NewHeader: %s", err)
	}
	window.AddChild(header)

	footer, err := NewFooter(mountpoint)
	if err != nil {
		log.Fatalf("NewFooter: %s", err)
	}
	window.AddChild(footer)

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

		window.Handle(Event{Event: ev})
		if window.NeedsResize() {
			window.Resize(Rect{Point{0, 0}, window.Size()})
		}
		if window.NeedsDisplay() {
			window.Paint()
		}
	}
}
