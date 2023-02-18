package dprof

import (
	"github.com/dan-and-dna/dprof/internal"
	"github.com/prometheus/client_golang/prometheus"
)

func GetStatRegistry() *prometheus.Registry {
	return internal.GetSingleInst().GetStatRegistry()
}

func DumpProfiles() {
	internal.GetSingleInst().DumpProfiles()
}
