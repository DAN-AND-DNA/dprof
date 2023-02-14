package main

import (
	"github.com/dan-and-dna/dprof"
	"time"
)

func main() {
	//dprof.DumpWhenSignal()
	dprof.DumpWhenCpuThreshold()
	//time.Sleep(5 * time.Second)
	for i := 0; i < 1000; i++ {
		f()
	}

	//dprof.DumpWhenSignal()

	time.Sleep(10 * time.Second)
}

func f() {
	count := 0
	for i := 0; i < 10000000; i++ {
		count += i
	}
}
