package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path"
	"syscall"
	"time"

	"github.com/nsf/termbox-go"
	"github.com/bpowers/go-tmux"
	"github.com/kardianos/osext"
)

const usage = `Usage: %s [OPTION...] MOUNTPOINT
Command line slack client implemented with slackfs + tmux.

If run with the -fs flag, the -token-path option or the
SLACKFS_TOKEN_PATH environmental variable can be used to specify the
location of a file containing your slack API token.  If neither is
set, the default token file location of '%s' is tried.

Options:
`

var memProfile, cpuProfile string
var w, h int

const mountpoint = "/tmp/slack"
const sessionName = "slack"

func CreateWindow(mountpoint, kind, name string) error {
	dir := path.Join(mountpoint, kind, "by-name", name)
	windowName := fmt.Sprintf("%s/%s", kind, name)
	target := fmt.Sprintf("%s:%s", sessionName, windowName)

	// if the window already exists, exit early
	if _, err := tmux.GetWindow(windowName); err == nil {
		return nil
	}

	if _, err := tmux.GetSession(sessionName); err != nil {
		tmux.NewSession(sessionName, windowName, "tail", "-f", path.Join(dir, "session"))
	} else {
		tmux.NewWindow(sessionName+":0", windowName, "-a", "tail", "-f", path.Join(dir, "session"))
	}

	tmux.SplitWindow(target+".0", "-b", "-h", "unset TMUX; exec tmux attach -t slack-shared:sidebar")
	tmux.ResizePane(target+".1", "-x", "24")
	tmux.SplitWindow(target+".0", "-l", "2", "-v", fmt.Sprintf("cat >%s/write", dir))
	tmux.SelectPane(target + ".2")

	return nil
}

func main() {
	var err error

	flag.StringVar(&memProfile, "memprofile", "",
		"write memory profile to this file")
	flag.StringVar(&cpuProfile, "cpuprofile", "",
		"write cpu profile to this file")

	sidebar := flag.Bool("sidebar", false, "is a sidebar")
	fs := flag.Bool("fs", false, "is slackfs")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, usage, os.Args[0])
		flag.PrintDefaults()
	}
	flag.Parse()
	if flag.NArg() != 0 {
		flag.Usage()
		os.Exit(1)
	}

	if *sidebar && *fs {
		fmt.Printf("fatal: -sidebar and -fs are mutually exclusive")
		flag.Usage()
		os.Exit(1)
	}

	if *sidebar {
		sidebarMain(mountpoint)
		return
	} else if *fs {
		fsMain(mountpoint)
		return
	}

	exe, err := osext.Executable()
	if err != nil {
		log.Fatalf("unable to get path to current executable")
	}

	err = termbox.Init()
	if err != nil {
		log.Fatalf("termbox.Init: %s", err)
	}
	w, h = termbox.Size()
	termbox.Close()

	var session tmux.Session
	if session, err = tmux.GetSession("slack-shared"); err != nil {
		session, err = tmux.NewSession("slack-shared", "slackfs", exe, "-fs")
		if err != nil {
			log.Fatalf("NewSession: %s", err)
		}
	}

	selfPath := path.Join(mountpoint, "self")
	for _, err = os.Stat(selfPath); err != nil; _, err = os.Stat(selfPath) {
		time.Sleep(200*time.Millisecond)
		// TODO: exponential backoff + timeout
	}

	var window tmux.Window
	if window, err = tmux.GetWindow("sidebar"); err != nil || window.SessionName != session.Name {
		target := fmt.Sprintf("%s:1", session.Name)
		window, err = tmux.NewWindow(target, "sidebar", exe, "-sidebar")
		if err != nil {
			log.Fatalf("NewWindow: %s", err)
		}
	}

	CreateWindow(mountpoint, "channels", "general")
	CreateWindow(mountpoint, "ims", "chris")
	CreateWindow(mountpoint, "ims", "slackbot")
	CreateWindow(mountpoint, "ims", "jeff")

	tmuxPath, _ := exec.LookPath("tmux")
	err = syscall.Exec(tmuxPath, []string{"tmux", "attach", "-t", sessionName}, os.Environ())
	if err != nil {
		log.Fatalf("Exec: %s", err)
	}
}
