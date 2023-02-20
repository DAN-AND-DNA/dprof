package main

import (
	"flag"
	"github.com/dan-and-dna/dprof"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"log"
	"net/http"
)

var addr = flag.String("listen-address", ":8080", "The address to listen on for HTTP requests.")

func main() {
	flag.Parse()

	dprof.DumpProfiles()

	// 37 mb
	d := make([]byte, 1024*1024*37)
	_ = d

	for i := 0; i < len(d); i++ {
		d[i] = 2
	}

	sReg := dprof.GetStatRegistry()
	http.Handle("/metrics", promhttp.HandlerFor(sReg, promhttp.HandlerOpts{Registry: sReg}))
	log.Fatal(http.ListenAndServe(*addr, nil))
}
