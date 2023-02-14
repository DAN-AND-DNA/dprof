package dprof

import "github.com/dan-and-dna/dprof/internal"

type Config = internal.Config

func SetConfig(config ...Config) {
	internal.GetSingleInst().SetConfig(config...)
}

func DumpWhenSignal() {
	internal.GetSingleInst().DumpWhenSignal()
}

func DumpWhenCpuThreshold() {
	internal.GetSingleInst().DumpWhenCpuThreshold()
}
