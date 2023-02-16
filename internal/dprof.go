package internal

import (
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
	stat         Stat
}

func GetSingleInst() *dProf {
	if singleInst == nil {
		once.Do(func() {
			singleInst = newDProf()
			err := singleInst.Init()
			if err != nil {
				panic(err)
			}
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
		stat:          NewStat(),
	}

	return d
}

func (d *dProf) Init() error {
	err := d.stat.Init()
	if err != nil {
		return err
	}

	go func() {
		for {
			select {
			case msg := <-d.dumpChan:
				switch msg.DumpType {
				case DumpSignal:
					log.Println("manual")
					// 手动模式，暂停之前全部启动的剖析
					if d.isCpuStarted {
						d.stopDumpCPU()
					}

					if d.isManualStarted {
						d.stopDumpCPU("manual_cpu")
						d.isManualStarted = false
					} else {
						d.dumpCPU("manual_cpu")
						d.isManualStarted = true
					}

				case DumpCPU30:
					fallthrough
				case DumpCPU40:
					fallthrough
				case DumpCPU50:
					fallthrough
				case DumpCPU60:
					fallthrough
				case DumpCPU70:
					fallthrough
				case DumpCPU80:
					fallthrough
				case DumpCPU90:
					if d.isManualStarted {
						continue
					}

					lastStartUnix, ok := d.lastStartUnix[msg.DumpType]
					interval := intervals[msg.DumpType]
					if ok {
						// 丢弃没时间的消息
						if msg.StartUnix <= 0 {
							log.Println("discard bad start unix")
							continue
						}

						if msg.StartUnix <= lastStartUnix {
							continue
						}

						// 是否满足执行的间隔时间
						if msg.StartUnix-lastStartUnix <= interval {
							continue
						}

						//log.Println(msg.DumpType, msg.StartUnix-lastStartUnix)
					}

					// 记录开始时间
					var pprofDuration int64 = 5
					d.lastStartUnix[msg.DumpType] = time.Now().Unix() + pprofDuration

					// 有已经在开始的cpu剖析，合并到上一次去
					// TODO 延长本次时间，避免剩余时间太短，无法捕获突发的cpu升高
					if d.isCpuStarted == true {
						continue
					}

					d.isCpuStarted = true
					log.Println("dump cpu pprof ", msg.DumpType)
					d.dumpCPU()

					go func() {
						onTime := time.After(time.Duration(pprofDuration) * time.Second)
						for {
							select {
							case _ = <-onTime:
								// 间隔时间到
								if !d.isCpuStarted {
									continue
								}

								log.Println("stop dump ..................")
								d.stopDumpCPU()

								d.isCpuStarted = false
							case _ = <-d.done:
								return
							}
						}
					}()
				}
			case _ = <-d.done:
				return
			}
		}
	}()

	return nil
}

func (d *dProf) MonitorCpu() {
	interval := time.Millisecond * 250
	go func() {
		cpuTicker := time.NewTicker(interval)
		defer cpuTicker.Stop()

		for {
			select {
			case <-cpuTicker.C:
				d.currCpuUsage = d.stat.UpdateCpuUsage()
				d.cleanCache()

				// 手动优先级高
				if d.isManualStarted {
					continue
				}

				// 超过总体cpu的30% 40% 60% 70% 80% 90%开始
				if d.currCpuUsage >= 900 {
					d.dumpChan <- dumpMsg{DumpType: DumpCPU90, StartUnix: time.Now().Unix()}
					continue
				}

				if d.currCpuUsage >= 800 {
					d.dumpChan <- dumpMsg{DumpType: DumpCPU80, StartUnix: time.Now().Unix()}
					continue
				}

				if d.currCpuUsage >= 700 {
					d.dumpChan <- dumpMsg{DumpType: DumpCPU70, StartUnix: time.Now().Unix()}
					continue
				}

				if d.currCpuUsage >= 600 {
					d.dumpChan <- dumpMsg{DumpType: DumpCPU60, StartUnix: time.Now().Unix()}
					continue
				}

				if d.currCpuUsage >= 500 {
					d.dumpChan <- dumpMsg{DumpType: DumpCPU50, StartUnix: time.Now().Unix()}
					continue
				}

				if d.currCpuUsage >= 400 {
					d.dumpChan <- dumpMsg{DumpType: DumpCPU40, StartUnix: time.Now().Unix()}
					continue
				}

				if d.currCpuUsage >= 300 {
					d.dumpChan <- dumpMsg{DumpType: DumpCPU30, StartUnix: time.Now().Unix()}
					continue
				}

			case <-d.done:
				return
			}
		}
	}()
}

// TryDumpCPU 尝试输出cpu pprof文件
func (d *dProf) dumpCPU(rawKind ...string) {
	defer recover1()

	kind := "cpu"
	if len(rawKind) != 0 {
		kind = rawKind[0]
	}

	f, err := d.createDumpFile(kind)
	if err != nil {
		panic(err)
	}

	// 开始采样
	err = pprof.StartCPUProfile(f)
	if err != nil {
		panic(err)
	}

	d.dumpClosers[kind] = func() {
		defer recover1()

		// 结束采样并写文件
		pprof.StopCPUProfile()
		f.Sync()
		err := f.Close()
		if err != nil {
			panic(err)
		}
	}
}

func (d *dProf) stopDumpCPU(rawKind ...string) {
	kind := "cpu"
	if len(rawKind) != 0 {
		kind = rawKind[0]
	}
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
		defer recover1()

		// 结束采样
		runtime.SetBlockProfileRate(0)
		// 写文件
		err := pprof.Lookup(kind).WriteTo(f, 0)
		if err != nil {
			panic(err)
		}

		err = f.Close()
		if err != nil {
			panic(err)
		}
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
		if currentUnix-lastStartUnix >= 120 {
			delete(d.lastStartUnix, cpuLevel)
		}
	}
}
