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
	s := "^"
	if e.expanded {
		s = "⌵"
		view.String(P(e.size.Width/2-len("quit")/2, 2), fgColor|bold, bgColor, "quit")
	}
	view.String(P(e.size.Width/2, 0), fgColor|bold, bgColor, s)
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
	e.needsDisplay = false
	view.SetCell(P(0, 0), '┌', fgColor, bgColor)
	view.SetCell(P(0, -1), '└', fgColor, bgColor)
	view.SetCell(P(-1, 0), '┐', fgColor, bgColor)
	view.SetCell(P(-1, -1), '┘', fgColor, bgColor)
}

type Channel struct {
	name string
	// users have 'away/avail'
	away      bool
	offline   bool
	hasUnread bool
	selected  bool
}

type Grouping struct {
	Element
	parent      *SelectorsContainer
	mount       string
	path        string
	displayName string
	chanPrefix  string // channels should be prefixed by '#'
	hasStatus   bool

	items []*Channel
}

func NewGrouping(parent *SelectorsContainer, mount, path, displayName, chanPrefix string, hasStatus bool) (*Grouping, error) {
	g := new(Grouping)
	g.parent = parent
	g.mount = mount
	g.path = path
	g.displayName = displayName
	g.chanPrefix = chanPrefix
	g.hasStatus = hasStatus
	g.items = make([]*Channel, 0, 16)

	err := g.updateItems()
	if err != nil {
		return nil, fmt.Errorf("updateItems(%s): %s", displayName, err)
	}

	return g, nil
}

func (e *Grouping) SetNamedSelection(name string) bool {
	for _, ch := range e.items {
		if ch.name == name {
			ch.selected = true
			return true
		}
	}
	return false
}

func (e *Grouping) SetSelection(i int) {
	if i >= 0 && i < len(e.items) {
		e.items[i].selected = true
		e.needsDisplay = true
	}
}

func (e *Grouping) Selection() int {
	for i, ch := range e.items {
		if ch.selected {
			return i
		}
	}
	return -1
}

func (e *Grouping) ClearSelection() {
	for _, ch := range e.items {
		ch.selected = false
	}
}

func (e *Grouping) SelectableCount() int {
	return len(e.items)
}

func (e *Grouping) updateItems() error {
	dPath := path.Join(e.mount, e.path, "by-name")
	dents, err := ioutil.ReadDir(dPath)
	if err != nil {
		return fmt.Errorf("ReadDir(%s): %s", dPath, err)
	}
	for _, dent := range dents {
		c := &Channel{name: dent.Name()}
		// FIXME: hack to get user away/offline/available
		if e.hasStatus {
			presence, err := readFile(e.mount, "users", "by-name", dent.Name(), "presence")
			if err == nil {
				c.away = presence == "away"
			}
		}
		e.items = append(e.items, c)
	}
	return nil
}

func (e *Grouping) Handle(ev Event) bool {
	if ev.Type == termbox.EventMouse && ev.Key == termbox.MouseLeft {
		i := ev.MousePos.Y - 1
		if i >= 0 && i < len(e.items) {
			e.parent.ClearSelection()
			e.items[i].selected = true
			e.needsDisplay = true
			return true
		}
	}
	return false
}

func (e *Grouping) Resize(available Rect) (desired Rect) {
	e.size = Size{available.Width, 2 + len(e.items)}
	return Rect{available.Point, e.Size()}
}

func (e *Grouping) Draw(view View) {
	e.needsDisplay = false
	view.String(P(0, 0), fgColor, bgColor, e.displayName)
	for i, item := range e.items {
		bg := bgColor
		text := fgColor
		if item.selected {
			bg = termbox.ColorWhite
			text = termbox.ColorBlack
			for j := 0; j < e.size.Width; j++ {
				view.SetCell(P(j, 1+i), ' ', text, bg)
			}
			view.SetCell(P(-3, 1+i), '✓', text, bg)
			view.SetCell(P(-1, 1+i), '⊗', text, bg)
		}
		if e.chanPrefix != "" {
			view.String(P(1, 1+i), text, bg, e.chanPrefix)
		}
		view.String(P(2, 1+i), text, bg, item.name)
		if e.hasStatus {
			fg := termbox.ColorGreen
			if item.away {
				fg = termbox.ColorBlack
			}
			view.SetCell(P(0, 1+i), '●', fg, bg)
		}
	}
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

	expandable := new(SelectorsContainer)

	chans, err := NewGrouping(expandable, mountpoint, "channels", "CHANNELS", "#", false)
	if err != nil {
		log.Fatalf("NewGroup(channels): %s", err)
	}
	ims, err := NewGrouping(expandable, mountpoint, "ims", "DIRECT MESSAGES", "", true)
	if err != nil {
		log.Fatalf("NewGroup(ims): %s", err)
	}
	groups, err := NewGrouping(expandable, mountpoint, "groups", "PRIVATE GROUPS", "", false)
	if err != nil {
		log.Fatalf("NewGroup(groups): %s", err)
	}
	expandable.AddChild(chans)
	expandable.AddChild(ims)
	expandable.AddChild(groups)
	//expandable.AddChild(new(Outline))

	chans.SetNamedSelection("general")

	window.AddChild(expandable)

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
