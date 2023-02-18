package internal

import (
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

type dumpMsg struct {
	DumpType  int
	StartUnix int64
}

/*
dProf

1. 根据信号量来手动输出pprof文件

2. 根据cpu、内存、网络超时的阈值自动输出pprof文件（持续5s），已经开始的剖析不可中断，新来的请求会被忽略（特殊情况是手动剖析会中断全部自动剖析）

	cpu: 处在不同的档位，以不同的速度输出pprof，首次到达新的档位，直接输出pprof，清理比当前档位高的过期档位

	cpu：输出持续5秒的（cpu）剖析记录，cd时间为10秒，最大保持100个cpu pprof文件(平均1k、2k的文件)

	内存：输出持续5秒的（内存）剖析记录，cd时间为10秒，最大保持100个内存 pprof文件(平均1k、2k的文件)

	网络：超时请求统计比例上升，输出（io阻塞）剖析记录

3. io 请求超时，超时错误，输出堆栈消息

5. 自动分析pprof文件，以日志形式输出pprof关键信息

6. pprof可以自动告警或远程
*/
type dProf struct {
	isManualStarted bool
	isCpuStarted    bool
	dumpClosers     map[string]func()
	signalChan      chan os.Signal
	dumpChan        chan dumpMsg
	lastStartUnix   map[int]int64
	nextInterval    map[int]int64
	done            chan struct{}

	currCpuUsage uint64
	//stat         Stat

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
		signalChan:    make(chan os.Signal, 1),
		dumpChan:      make(chan dumpMsg, 2000),
		lastStartUnix: make(map[int]int64),
		nextInterval:  make(map[int]int64),
		done:          make(chan struct{}),
		dumpClosers:   make(map[string]func()),
		stat:          stat.New(),
		//stat:          NewStat(),
	}

	// 监控进程性能
	d.stat.MonitorProcess()

	return d
}

func (d *dProf) GetStatRegistry() *prometheus.Registry {
	return d.stat.Registry
}

// DumpProfiles 输出pprof信息文件
func (d *dProf) DumpProfiles() {
	go func() {
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

			if isDoingProfile == true {
				continue
			}

			if d.stat.Metrics.CpuUsage <= 100 {
				// cpu <= 10%  (1次/60秒，持续5s)
				onTimePProf(DumpCPU10, 80, 5, d.dumpCpuPProf)
			} else if d.stat.Metrics.CpuUsage <= 300 {
				// 10% < cpu <= 30%  (1次/35秒，持续5s)
				onTimePProf(DumpCPU30, 50, 5, d.dumpCpuPProf)
			} else if d.stat.Metrics.CpuUsage <= 500 {
				// 30% < cpu <= 50%  (1次/15秒，持续5s)
				onTimePProf(DumpCPU50, 30, 5, d.dumpCpuPProf)
			} else if d.stat.Metrics.CpuUsage <= 700 {
				// 50% < cpu <= 70%  (1次/10秒，持续5s)
				onTimePProf(DumpCPU70, 20, 5, d.dumpCpuPProf)
			} else {
				//  70% < cpu (1次/6秒，持续5s)
				onTimePProf(DumpCPU80, 6, 5, d.dumpCpuPProf)
			}
		}
	}()
}

// TryDumpCPU 尝试输出cpu pprof文件
func (d *dProf) dumpCpuPProf() func() {
	kind := "cpu"

	f, err := d.createDumpFile(kind)
	if err != nil {
		log.Println(err)
	}

	// 开始采样
	err = pprof.StartCPUProfile(f)
	if err != nil {
		log.Println(err)
	}

	return func() {
		// 结束采样并写文件
		pprof.StopCPUProfile()
		_ = f.Sync()
		_ = f.Close()
	}
}

func (d *dProf) stopDumpCPU() {
	kind := "cpu"

	f, ok := d.dumpClosers[kind]
	if !ok {
		return
	}
	delete(d.dumpClosers, kind)
	f()
}

// TryDumpBlock 尝试输出io阻塞 pprof文件
func (d *dProf) dumpBlock() error {
	kind := "block"
	f, err := d.createDumpFile(kind)
	if err != nil {
		return err
	}

	// 开始采样
	runtime.SetBlockProfileRate(1)

	d.dumpClosers[kind] = func() {
		// 结束采样
		runtime.SetBlockProfileRate(0)
		// 写文件
		_ = pprof.Lookup(kind).WriteTo(f, 0)

		_ = f.Close()
	}

	return nil
}

func recover1() {
	if err := recover(); err != nil {
		var buf = make([]byte, 1024)

		runtime.Stack(buf, false)
		log.Printf("error: %v \n%s", err, string(buf))
	}
}

// 下降清理之前高于的记录
func (d *dProf) cleanCache() {
	currentUnix := time.Now().Unix()
	for cpuLevel, lastStartUnix := range d.lastStartUnix {
		// 两分钟没变化
		if currentUnix-lastStartUnix >= 30 {
			delete(d.lastStartUnix, cpuLevel)
		}
	}
}
