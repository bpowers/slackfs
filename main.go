// Copyright 2015 Bobby Powers. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"bazil.org/fuse"
	"bazil.org/fuse/fs"
)

const usage = `Usage: %s [OPTION...] MOUNTPOINT
Slack as a filesystem.

Options:
`

func debug(msg interface{}) {
	log.Printf("%s", msg)
}

func init() {
	runtime.GOMAXPROCS(runtime.NumCPU())
}

var memProfile, cpuProfile string

func main() {
	flag.StringVar(&memProfile, "memprofile", "",
		"write memory profile to this file")
	flag.StringVar(&cpuProfile, "cpuprofile", "",
		"write cpu profile to this file")
	offline := flag.String("offline", "",
		"specified JSON info response file to use offline")
	token := flag.String("token", "", "Slack API token")

	verbose := flag.Bool("v", false, "verbose FUSE logging")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, usage, os.Args[0])
		flag.PrintDefaults()
	}
	flag.Parse()
	if flag.NArg() != 1 {
		flag.Usage()
		os.Exit(1)
	}

	if *verbose {
		fuse.Debug = debug
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
			//prof, err := NewProf(memProfile, cpuProfile)
			//if err != nil {
			//	log.Fatal(err)
			//}
			//prof.Start()
		}
	}()

	var conn *FSConn
	if *offline != "" {
		log.Printf("offline mode")
		conn, err = NewOfflineFS(*offline)
	} else {
		conn, err = NewFS(*token)
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

	err = fs.Serve(c, conn.super)
	if err != nil {
		log.Fatal(err)
	}

	// check if the mount process has an error to report
	<-c.Ready
	if err := c.MountError; err != nil {
		log.Fatal(err)
	}

	time.Sleep(120 * time.Second)
	log.Printf("done done")
}
