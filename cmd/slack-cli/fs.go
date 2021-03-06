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
	"syscall"

	"github.com/bpowers/fuse"
	"github.com/bpowers/fuse/fs"
	"github.com/bpowers/slackfs"
)

const noTokenFound = `No token file could be read.  If you haven't generated one yet, go to
https://api.slack.com/web and scroll to the bottom. If 'none' is
listed as the token, then click the green 'Create Token' button next
to it. Copy the token that appears in red into an otherwise empty file
stored at ~/.slack-token (or a location of your choosing, specified in
the environment by SLACKFS_TOKEN_PATH or after the commandline flag
-token-path).

`

var defaultTokenPath string // initalized in init() below

func debugOut(msg interface{}) {
	log.Printf("%s", msg)
}

func init() {
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

	flag.StringVar(&offline, "offline", "",
		"specified JSON info response file to use offline")
	flag.StringVar(&tokenPath, "token-path", "", "Slack API token")

	flag.BoolVar(&verbose, "v", false, "verbose logging (fs)")
}

var offline, tokenPath string
var verbose bool

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

func fsMain(mountpoint string) {
	token := getToken(tokenPath)
	if token == "" {
		fmt.Fprintf(os.Stderr, noTokenFound)
		flag.Usage()
		os.Exit(1)
	}

	var debugFn func(msg interface{})
	if verbose {
		debugFn = debugOut
	}

	prof, err := slackfs.NewProf(memProfile, cpuProfile)
	if err != nil {
		log.Fatal(err)
	}
	// if -memprof or -cpuprof haven't been set on the command
	// line, these are nops
	prof.Start()
	// Set up channel on which to send signal notifications.  We
	// must use a buffered channel or risk missing the signal if
	// we're not ready to receive when the signal is sent.
	sigChan := make(chan os.Signal, 16)
	signal.Notify(sigChan, syscall.SIGQUIT, syscall.SIGINT, syscall.SIGKILL, syscall.SIGTERM)
	go func() {
		for sig := range sigChan {
			log.Printf("got signal %s", sig)
			switch sig {
			case syscall.SIGQUIT:
				prof.Stop()
			default:
				if err := fuse.Unmount(mountpoint); err != nil {
					log.Fatal("unmount: %s", err)
				}
			}
		}
	}()

	// If a previous slackfs instance exited without unmounting,
	// the FS will still be visiible in mtab, but stat will return
	// ENOTCONN.  Unmount if that is the case.
	if _, err := os.Stat(mountpoint); err != nil && err.(*os.PathError).Err == syscall.ENOTCONN {
		fuse.Unmount(mountpoint)
	}

	if err := os.MkdirAll(mountpoint, 0777); err != nil {
		log.Fatalf("couldn't create mountpoint: %s", err)
	}

	var conn *slackfs.FSConn
	if offline != "" {
		conn, err = slackfs.NewOfflineFSConn(offline)
	} else {
		conn, err = slackfs.NewFSConn(token)
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
	defer fuse.Unmount(mountpoint)
	defer func() {
		log.Printf("closed + unmounted fs")
	}()

	// check if the mount process has an error to report
	<-c.Ready
	if err := c.MountError; err != nil {
		log.Fatal(err)
	}

	log.Printf("FS ready, serving requests")

	err = fs.Serve(c, conn.Super, debugFn)
	if err != nil {
		log.Fatal(err)
	}
}
