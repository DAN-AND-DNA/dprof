package main

import (
	"flag"
	"github.com/dan-and-dna/dprof/stat"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"log"
	"net/http"
)

var addr = flag.String("listen-address", ":8080", "The address to listen on for HTTP requests.")

func main() {
	flag.Parse()

	s := stat.New()

	// Add go runtime metrics and process collectors.
	s.Registry.MustRegister(
		collectors.NewGoCollector(collectors.WithGoCollectorRuntimeMetrics()),
	)

	s.MonitorProcess()

	t := make([]byte, 1024*1024*1024*21)
	_ = t

	// Expose /metrics HTTP endpoint using the created custom registry.
	http.Handle("/metrics", promhttp.HandlerFor(s.Registry, promhttp.HandlerOpts{Registry: s.Registry}))
	log.Fatal(http.ListenAndServe(*addr, nil))
}
