package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"runtime"
	"syscall"
)

const usage = `Usage: %s [OPTION...] MOUNTPOINT
Slack as a filesystem.

Options:
`

func init() {
	runtime.GOMAXPROCS(runtime.NumCPU())
}

var memProfile, cpuProfile string

func main() {
	flag.StringVar(&memProfile, "memprofile", "",
		"write memory profile to this file")
	flag.StringVar(&cpuProfile, "cpuprofile", "",
		"write cpu profile to this file")
	token := flag.String("token", "", "Slack API token")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, usage, os.Args[0])
		flag.PrintDefaults()
	}
	flag.Parse()
	if flag.NArg() != 1 {
		flag.Usage()
		os.Exit(1)
	}

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

	log.Printf("got tok: %s", *token)

	log.Printf("done done")

}
