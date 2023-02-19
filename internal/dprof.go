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

	intervals = map[int]int64{
		DumpCPU30: 10,
		DumpCPU40: 15,
		DumpCPU50: 30,
		DumpCPU60: 35,
		DumpCPU70: 35,
		DumpCPU80: 40,
		DumpCPU90: 40,
	}
)

const (
	DumpNone   = 0
	DumpSignal = iota + 1000
	DumpCPU10  = 100
	DumpCPU20  = 200
	DumpCPU30  = 300
	DumpCPU40  = 400
	DumpCPU50  = 500
	DumpCPU60  = 600
	DumpCPU70  = 700
	DumpCPU80  = 800
	DumpCPU90  = 900

	DumpEOF = 9999
)

type dProf struct {
	signalChan chan os.Signal
	done       chan struct{}

	stat *stat.Stat
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
		signalChan: make(chan os.Signal, 1),
		done:       make(chan struct{}),
		stat:       stat.New(),
	}

	// 开始监控进程性能
	d.stat.MonitorProcess()

	return d
}

// GetStatRegistry 返回当前使用的prometheus registry
func (d *dProf) GetStatRegistry() *prometheus.Registry {
	return d.stat.Registry
}

/*
DumpProfiles 输出pprof信息文件

	cpu:
	1. 处于不同的高度，记录一下
	2. 抖动超过100，记录一下
*/
func (d *dProf) DumpProfiles() {
	go func() {
		// TODO 原子
		isDoingProfile := false
		timers := make(map[int]int64)

		// 定时进行pprof
		onTimePProf := func(key int, interval, keepTime int64, startPProfFunc func() func()) {
			prevTime, ok := timers[key]
			currentTime := time.Now().Unix()
			canDump := false
			if !ok {
				prevTime = currentTime
				canDump = true
			} else if currentTime-prevTime >= interval {
				canDump = true
			}

			if canDump {
				log.Println("start pprof... ", key)

				// 避免再次启动
				for k, _ := range timers {
					if k < key {
						timers[k] = currentTime
					}
				}
				timers[key] = currentTime
				isDoingProfile = true

				stopPProfFunc := startPProfFunc()
				time.AfterFunc(time.Duration(keepTime)*time.Second, func() {
					log.Println("pprof stopped ", key)
					stopPProfFunc()
					isDoingProfile = false
				})
			}
		}

		for {
			time.Sleep(1 * time.Second)

			onTimePProf(DumpCPU20, 120, 5, func() func() { return d.dumpHeapProfile("normal") })
			// 跳过已经开始的采样
			if isDoingProfile == true {
				continue
			}

			// 抖动厉害需要跳过或记录
			if d.stat.Metrics.CpuUsageStdDeviation > 50 {

				// 当前cpu超过100，且抖动厉害，需要单独记录
				if d.stat.Metrics.CpuUsageStdDeviation >= 100 && d.stat.Metrics.CpuUsage >= 100 {
					onTimePProf(DumpCPU90, 20, 5, func() func() { return d.dumpCpuProfile("odd_gte100") })
				}

				continue
			}

			if d.stat.Metrics.CpuUsage <= 100 {
				// cpu <= 10%  (1次/60秒，持续5s)
				onTimePProf(DumpCPU10, 120, 5, func() func() { return d.dumpCpuProfile("normal_le100") })
			} else if d.stat.Metrics.CpuUsage <= 300 {
				// 10% < cpu <= 30%  (1次/35秒，持续5s)
				onTimePProf(DumpCPU30, 50, 5, func() func() { return d.dumpCpuProfile("normal_le300") })
			} else if d.stat.Metrics.CpuUsage <= 500 {
				// 30% < cpu <= 50%  (1次/15秒，持续5s)
				onTimePProf(DumpCPU50, 30, 5, func() func() { return d.dumpCpuProfile("normal_le500") })
			} else if d.stat.Metrics.CpuUsage <= 700 {
				// 50% < cpu <= 70%  (1次/10秒，持续5s)
				onTimePProf(DumpCPU70, 20, 5, func() func() { return d.dumpCpuProfile("normal_le700") })
			} else {
				//  70% < cpu (1次/6秒，持续5s)
				onTimePProf(DumpCPU80, 6, 5, func() func() { return d.dumpCpuProfile("normal_le1000") })
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

func recover1() {
	if err := recover(); err != nil {
		var buf = make([]byte, 1024)

		runtime.Stack(buf, false)
		log.Printf("error: %v \n%s", err, string(buf))
	}
}
