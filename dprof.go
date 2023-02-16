package dprof

import "github.com/dan-and-dna/dprof/internal"

func DumpWhenSignal() {
	internal.GetSingleInst().DumpWhenSignal()
}

func MonitorCpu() {
	internal.GetSingleInst().MonitorCpu()
}
