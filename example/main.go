package main

import (
	"fmt"
	"github.com/dan-and-dna/dprof"
	"github.com/dan-and-dna/dprof/stackerr"
	"runtime/debug"
	"time"
)

func main() {
	//g()
	dprof.MonitorCpu()

	time.Sleep(4 * time.Second)

	go func() {
		for {

		}
	}()

	time.Sleep(4 * time.Second)

	go func() {
		for {

		}
	}()

	time.Sleep(4 * time.Second)

	go func() {
		for {

		}
	}()

	time.Sleep(4 * time.Second)

	go func() {
		for {

		}
	}()

	time.Sleep(4 * time.Second)

	go func() {
		for {

		}
	}()

	time.Sleep(4 * time.Second)

	go func() {
		for {

		}
	}()

	time.Sleep(100 * time.Second)
}

func g() {
	f()
}

func f() {
	err := stackerr.New()
	fmt.Println(err)
	debug.PrintStack()
}
