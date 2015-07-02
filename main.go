// Copyright 2015 Bobby Powers. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/signal"
	"os/user"
	"runtime"
	"syscall"
	"time"

	"github.com/bpowers/fuse"
	"github.com/bpowers/fuse/fs"
)

const (
	usage = `Usage: %s [OPTION...] MOUNTPOINT
Slack as a filesystem.

The -token-path option or the SLACKFS_TOKEN_PATH environmental
variable can be used to specify the location of a file containing your
slack API token.  If neither is set, the default token file location
of '%s' is tried.

Options:
`

	noTokenFound = `No token file could be read.  If you haven't generated one yet, go to
https://api.slack.com/web and scroll to the bottom. If 'none' is
listed as the token, then click the green 'Create Token' button next
to it. Copy the token that appears in red into an otherwise empty file
stored at ~/.slack-token (or a location of your choosing, specified in
the environment by SLACKFS_TOKEN_PATH or after the commandline flag
-token-path).

`
)

var defaultTokenPath string // initalized in init() below

func debugOut(msg interface{}) {
	log.Printf("%s", msg)
}

func init() {
	// with more than 1 maxproc, we end up with wild, terrible
	// memory fragmentation. pss goes from ~14 MB to 180 MB.
	runtime.GOMAXPROCS(1)

	home := ""

	// If we're statically linked, user.Current won't work.
	if u, err := user.Current(); err == nil {
		home = u.HomeDir
	} else {
		home = os.Getenv("HOME")
	}
	// If we don't have HOME in our environment, we might be
	// running under something like a systemd chroot.  Check the
	// chroot root.
	defaultTokenPath = fmt.Sprintf("%s/.slack-token", home)
}

func getToken(flagPath string) string {
	path := defaultTokenPath
	if envPath := os.Getenv("SLACKFS_TOKEN_PATH"); envPath != "" {
		path = envPath
	}
	if flagPath != "" {
		path = flagPath
	}
	if path == "" {
		return ""
	}
	buf, err := ioutil.ReadFile(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error reading token file: %s\n\n", err)
		return ""
	}
	return string(bytes.TrimSpace(buf))
}

var memProfile, cpuProfile string

func main() {
	flag.StringVar(&memProfile, "memprofile", "",
		"write memory profile to this file")
	flag.StringVar(&cpuProfile, "cpuprofile", "",
		"write cpu profile to this file")
	offline := flag.String("offline", "",
		"specified JSON info response file to use offline")
	tokenPath := flag.String("token-path", "", "Slack API token")

	verbose := flag.Bool("v", false, "verbose FUSE logging")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, usage, os.Args[0], defaultTokenPath)
		flag.PrintDefaults()
	}
	flag.Parse()
	if flag.NArg() != 1 {
		flag.Usage()
		os.Exit(1)
	}
	token := getToken(*tokenPath)
	if token == "" {
		fmt.Fprintf(os.Stderr, noTokenFound)
		flag.Usage()
		os.Exit(1)
	}

	var debugFn func(msg interface{})
	if *verbose {
		debugFn = debugOut
	}

	mountpoint := flag.Arg(0)

	prof, err := NewProf(memProfile, cpuProfile)
	if err != nil {
		log.Fatal(err)
	}
	// if -memprof or -cpuprof haven't been set on the command
	// line, these are nops
	prof.Start()
	// Set up channel on which to send signal notifications.  We
	// must use a buffered channel or risk missing the signal if
	// we're not ready to receive when the signal is sent.
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGQUIT)
	go func() {
		for range sigChan {
			prof.Stop()
		}
	}()

	if err := os.MkdirAll(mountpoint, 0777); err != nil {
		log.Fatalf("couldn't create mountpoint: %s", err)
	}

	var conn *FSConn
	if *offline != "" {
		conn, err = NewOfflineFSConn(*offline)
	} else {
		conn, err = NewFSConn(token)
	}
	if err != nil {
		log.Fatalf("NewFS: %s", err)
	}

	c, err := fuse.Mount(
		mountpoint,
		fuse.FSName("slack"),
		fuse.Subtype("slackfs"),
		fuse.LocalVolume(),
		fuse.VolumeName("Slack"),
	)
	if err != nil {
		log.Fatalf("Mount(%s): %s", mountpoint, err)
	}
	defer c.Close()

	err = fs.Serve(c, conn.super, debugFn)
	if err != nil {
		log.Fatal(err)
	}

	// check if the mount process has an error to report
	<-c.Ready
	if err := c.MountError; err != nil {
		log.Fatal(err)
	}

	log.Printf("almost done")
	time.Sleep(120 * time.Second)
	log.Printf("done done")
}
