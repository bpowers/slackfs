package main

import (
	"log"
	"os"
	"runtime"
	"runtime/pprof"
)

type ProfInstance struct {
	memprofPath, cpuprofPath string
	cpuprof                  *os.File
}

func NewProf(memprof, cpuprof string) (p *ProfInstance, err error) {
	p = &ProfInstance{
		memprofPath: memprof,
		cpuprofPath: cpuprof,
	}
	if cpuprof != "" {
		if p.cpuprof, err = os.Create(cpuprof); err != nil {
			p = nil
			return
		}
	}
	return
}

// startProfiling enables memory and/or CPU profiling if the
// appropriate command line flags have been set.
func (p *ProfInstance) Start() {

	// if we've passed in filenames to dump profiling data too,
	// start collecting profiling data.
	if p.memprofPath != "" {
		runtime.MemProfileRate = 1
	}
	if p.cpuprof != nil {
		pprof.StartCPUProfile(p.cpuprof)
	}
}

func (p *ProfInstance) Stop() {
	log.Printf("writing profiles")
	if p.memprofPath != "" {
		var err error
		var f *os.File
		if f, err = os.Create(p.memprofPath); err != nil {
			log.Printf("os.Create(%s): %s", p.memprofPath, err)
			return
		}
		defer f.Close()
		runtime.GC()
		pprof.WriteHeapProfile(f)
	}
	if p.cpuprof != nil {
		pprof.StopCPUProfile()
		p.cpuprof.Close()
	}
}
