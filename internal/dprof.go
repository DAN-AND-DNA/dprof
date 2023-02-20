package internal

import (
	"fmt"
	"github.com/dan-and-dna/dprof/stat"
	"github.com/prometheus/client_golang/prometheus"
	"log"
	"os"
	"runtime"
	"runtime/pprof"
	"sync"
	"time"
)

var (
	singleInst *dProf = nil
	once       sync.Once
)

const (
	DumpNone   = 0
	DumpSignal = iota + 1000
	DumpCPU
	DumpMEM

	Dump100 = 100
	Dump200 = 200
	Dump300 = 300
	Dump400 = 400
	Dump500 = 500
	Dump600 = 600
	Dump700 = 700
	Dump800 = 800
	Dump900 = 900

	DumpEOF = 9999
)

type dProf struct {
	signalChan        chan os.Signal
	done              chan struct{}
	isDoingCpuProfile bool
	isDoingMemProfile bool
	timers            map[int]map[int]int64
	stat              *stat.Stat
}

func GetSingleInst() *dProf {
	if singleInst == nil {
		once.Do(func() {
			singleInst = newDProf()
		})
	}

	return singleInst
}
func newDProf() *dProf {
	d := &dProf{
		signalChan:        make(chan os.Signal, 1),
		done:              make(chan struct{}),
		isDoingCpuProfile: false,
		isDoingMemProfile: false,
		timers: map[int]map[int]int64{
			DumpCPU: make(map[int]int64),
			DumpMEM: make(map[int]int64),
		},
		stat: stat.New(),
	}

	// 监控进程指标
	d.stat.MonitorProcess()
	// 监控go运行时指标
	d.stat.MonitorGoRuntime()

	return d
}

// GetStatRegistry 返回当前使用的prometheus registry
func (d *dProf) GetStatRegistry() *prometheus.Registry {
	return d.stat.Registry
}

func (d *dProf) onTimePProf(pprofType, key int, interval, keepTime int64, startPProfFunc func() func()) {
	// 判断是否已经在执行
	if pprofType == DumpCPU && d.isDoingCpuProfile {
		return
	}

	if pprofType == DumpMEM && d.isDoingMemProfile {
		return
	}

	timers := d.timers[pprofType]
	prevTime, ok := timers[key]
	currentTime := time.Now().Unix()
	canDump := false
	if !ok {
		prevTime = currentTime
		canDump = true
	} else if currentTime-prevTime >= interval {
		canDump = true
	}

	// 判断时间是否运行
	if canDump {
		log.Println("start pprof... ", key)

		// 避免再次启动
		for k, _ := range timers {
			if k < key {
				timers[k] = currentTime
			}
		}

		timers[key] = currentTime
		if pprofType == DumpCPU {
			d.isDoingCpuProfile = true
		}

		if pprofType == DumpMEM {
			d.isDoingMemProfile = true
		}

		stopPProfFunc := startPProfFunc()
		time.AfterFunc(time.Duration(keepTime)*time.Second, func() {
			log.Println("pprof stopped ", key)
			stopPProfFunc()
			if pprofType == DumpCPU {
				d.isDoingCpuProfile = true
			}
			if pprofType == DumpMEM {
				d.isDoingMemProfile = true
			}
		})
	}
}

/*
DumpProfiles 输出pprof信息文件

	cpu:
	1. 处于不同的高度，记录一下
	2. 抖动超过100，记录一下
*/
func (d *dProf) DumpProfiles() {
	go func() {
		for {
			time.Sleep(1 * time.Second)

			// 进程的内存相关剖析 //TODO
			if d.stat.Metrics.MemUsage >= 100 {
				d.onTimePProf(DumpMEM, Dump100, 120, 5, func() func() { return d.dumpHeapProfile("normal_gte100") })
			}

			// 进程cpu相关剖析
			if d.stat.Metrics.CpuUsageStdDeviation > 50 {
				// 当前cpu超过100，且抖动厉害，需要单独记录
				if d.stat.Metrics.CpuUsageStdDeviation >= 100 && d.stat.Metrics.CpuUsage >= 100 {
					d.onTimePProf(DumpCPU, Dump900, 20, 5, func() func() { return d.dumpCpuProfile("odd_gte100") })
				}

			} else {
				// 定位负载
				if d.stat.Metrics.CpuUsage <= 100 {
					// cpu <= 10%  (1次/60秒，持续5s)
					d.onTimePProf(DumpCPU, Dump100, 120, 5, func() func() { return d.dumpCpuProfile("normal_le100") })
				} else if d.stat.Metrics.CpuUsage <= 300 {
					// 10% < cpu <= 30%  (1次/35秒，持续5s)
					d.onTimePProf(DumpCPU, Dump300, 50, 5, func() func() { return d.dumpCpuProfile("normal_le300") })
				} else if d.stat.Metrics.CpuUsage <= 500 {
					// 30% < cpu <= 50%  (1次/15秒，持续5s)
					d.onTimePProf(DumpCPU, Dump500, 30, 5, func() func() { return d.dumpCpuProfile("normal_le500") })
				} else if d.stat.Metrics.CpuUsage <= 700 {
					// 50% < cpu <= 70%  (1次/10秒，持续5s)
					d.onTimePProf(DumpCPU, Dump700, 20, 5, func() func() { return d.dumpCpuProfile("normal_le700") })
				} else {
					//  70% < cpu (1次/6秒，持续5s)
					d.onTimePProf(DumpCPU, Dump800, 6, 5, func() func() { return d.dumpCpuProfile("normal_le1000") })
				}
			}
		}
	}()
}

// dumpCpuProfile 输出cpu剖析文件
func (d *dProf) dumpCpuProfile(tag string) func() {
	nop := func() {}
	kind := "cpu"

	f, err := d.createDumpFile(fmt.Sprintf("%s-%s", kind, tag))
	if err != nil {
		log.Println(err)
		return nop
	}

	// 开始采样
	err = pprof.StartCPUProfile(f)
	if err != nil {
		log.Println(err)
		return nop
	}

	return func() {
		// 结束采样并写文件
		pprof.StopCPUProfile()
		_ = f.Sync()
		_ = f.Close()
	}
}

// dumpMemProfile 输出内存快照
func (d *dProf) dumpHeapProfile(tag string) func() {
	nop := func() {}
	kind := "heap"

	f, err := d.createDumpFile(fmt.Sprintf("%s-%s", kind, tag))
	if err != nil {
		log.Println(err)
		return nop
	}

	bakMemProfileRate := runtime.MemProfileRate
	// 尽量多
	runtime.MemProfileRate = 4096
	log.Println(bakMemProfileRate)

	return func() {
		err := pprof.Lookup(kind).WriteTo(f, 0)
		if err != nil {
			log.Println(err)
		}
		_ = f.Sync()
		_ = f.Close()
		runtime.MemProfileRate = bakMemProfileRate
	}
}
