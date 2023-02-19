package main

import (
	"flag"
	"github.com/dan-and-dna/dprof"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"log"
	"net/http"
)

var addr = flag.String("listen-address", ":8080", "The address to listen on for HTTP requests.")

func main() {
	flag.Parse()

	for i := 0; i < 10; i++ {
		f1()
	}
	dprof.DumpProfiles()
	sReg := dprof.GetStatRegistry()

	// Add go runtime metrics and process collectors.
	sReg.MustRegister(
		collectors.NewGoCollector(collectors.WithGoCollectorRuntimeMetrics()),
	)

	/*
		for i := 0; i < 13000; i++ {
			go func() {
				time.Sleep(time.Duration(2*(i+1)) * time.Millisecond)
				done := time.After(130 * time.Second)
				for {
					select {
					case <-done:
						return
					default:
						time.Sleep(1000 * time.Nanosecond)
					}

				}
			}()
		}

	*/

	// 100 m

	// Expose /metrics HTTP endpoint using the created custom registry.
	http.Handle("/metrics", promhttp.HandlerFor(sReg, promhttp.HandlerOpts{Registry: sReg}))
	log.Fatal(http.ListenAndServe(*addr, nil))
}

func f1() {
	b := make([]byte, 1024*1024)
	_ = b

	c := make([]byte, 1024*1024)
	_ = c

	for i := 0; i < len(c); i++ {
		c[i] = 3
	}
}
