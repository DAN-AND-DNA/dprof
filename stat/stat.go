package stat

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/shirou/gopsutil/process"
	"log"
	"math"
	"os"
	"runtime"
	"time"
)

var (
	_gaugeProcessCpuUsage = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "process_cpu_usage",
		Help: "进程cpu使用率 比例为(1/1000)",
	})

	_gaugeProcessRecentCpuUsageLevel = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "process_recent_cpu_usageX",
		Help: "最近进程cpu使用率 比例为(1/1000)",
	}, []string{"cpu"})

	_gaugeProcessMemUsage = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "process_mem_usage",
		Help: "进程内存使用率 比例为(1/1000)",
	})

	_gaugeProcessRecentMemUsage = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "process_recent_mem_usage",
		Help: "最近进程内存使用率 比例为(1/1000)",
	}, []string{"mem"})

	_gaugeRuntimeMemInfo = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "runtime_mem_info",
		Help: "运行时内存信息",
	}, []string{"runtime_mem"})
)

type Metrics struct {
	// 进程级cpu
	CpuUsage             int64   // 当前
	PrevCpuUsage1        int64   // 250ms
	PrevCpuUsage2        int64   // 500ms
	CpuUsageStdDeviation float64 // cpu标准差

	// 进程级别内存
	MemUsage      int64
	PrevMemUsage1 int64
	PrevMemUsage2 int64

	// 运行时级别
	CpuNum       int    // 可用逻辑cpu核心数
	GoroutineNum int    // 当前协程数
	Alloc        uint64 // 当前分配对象所占的内存大小
	TotalAlloc   uint64 // 累计拿来进行对象分配的内存字节大小
	Sys          uint64 // 从操作系统申请到虚拟内存大小
	NumGC        uint32 // 已经完成的GC次数
	HeapInuse    uint64 // 使用中的堆大小
	HeapIdle     uint64 // 空闲的堆大小
	HeapAlloc    uint64 // 当前分配对象所占的内存大小
	HeapSys      uint64 // 从操作系统申请到堆虚拟内存大小
	HeapReleased uint64 // 释放返回给操作系统的堆的大小
}

type Stat struct {
	currentProcess *process.Process

	Registry *prometheus.Registry
	Metrics  Metrics
}

func New() *Stat {
	s := &Stat{}

	// 当前进程
	s.currentProcess, _ = process.NewProcess(int32(os.Getpid()))
	s.Registry = prometheus.NewRegistry()
	s.Registry.MustRegister(
		_gaugeProcessCpuUsage,
		_gaugeProcessRecentCpuUsageLevel,
		_gaugeProcessMemUsage,
		_gaugeProcessRecentMemUsage,
		_gaugeRuntimeMemInfo,
	)

	return s
}

// MonitorProcess 监控进程信息
func (stat *Stat) MonitorProcess() {
	if stat.currentProcess == nil {
		return
	}

	processCpuMemInterval := 250 * time.Millisecond

	// 进程 cpu 和 内存
	go func() {
		var c1, c2, c3 float64
		var m1, m2, m3 float64

		for {
			time.Sleep(processCpuMemInterval)

			// 拿进程cpu站系统总体cpu的比例
			cpuUsage, _ := stat.currentProcess.Percent(0)
			_gaugeProcessCpuUsage.Set(cpuUsage) // 千分之几

			c1, c2, c3 = cpuUsage, c1, c2
			// 平均值
			avgC := (c1 + c2 + c3) / 3
			// 方差
			varianceC := (math.Pow(c1-avgC, 2) + math.Pow(c2-avgC, 2) + math.Pow(c3-avgC, 2)) / 3
			stdDeviation := math.Sqrt(varianceC)
			stat.Metrics.CpuUsageStdDeviation = stdDeviation

			stat.Metrics.CpuUsage = int64(c1)
			stat.Metrics.PrevCpuUsage1 = int64(c2)
			stat.Metrics.PrevCpuUsage2 = int64(c3)

			_gaugeProcessRecentCpuUsageLevel.WithLabelValues("0ms").Set(c1)
			_gaugeProcessRecentCpuUsageLevel.WithLabelValues("250ms").Set(c2)
			_gaugeProcessRecentCpuUsageLevel.WithLabelValues("500ms").Set(c3)
			_gaugeProcessRecentCpuUsageLevel.WithLabelValues("std deviation").Set(stdDeviation)

			// 拿进程的内存
			memUsage, _ := stat.currentProcess.MemoryPercent()
			_gaugeProcessMemUsage.Set(float64(memUsage * 10)) // 千分之几

			m1, m2, m3 = float64(memUsage*10), m1, m2

			stat.Metrics.MemUsage = int64(m1)
			stat.Metrics.MemUsage = int64(m2)
			stat.Metrics.MemUsage = int64(m3)

			_gaugeProcessRecentMemUsage.WithLabelValues("0ms").Set(m1)
			_gaugeProcessRecentMemUsage.WithLabelValues("250ms").Set(m2)
			_gaugeProcessRecentMemUsage.WithLabelValues("500ms").Set(m3)
		}
	}()
}

func (stat *Stat) MonitorGoRuntime() {
	if stat.currentProcess == nil {
		return
	}

	runtimeInfoInterval := 1 * time.Second
	runtimeMemInterval := 5 * time.Second // 操作开销高

	// 开销不高的运行信息
	go func() {
		for {
			time.Sleep(runtimeInfoInterval)

			stat.Metrics.CpuNum = runtime.NumCPU()
			stat.Metrics.GoroutineNum = runtime.NumGoroutine()

			_gaugeRuntimeMemInfo.WithLabelValues("CpuNum").Set(float64(stat.Metrics.CpuNum))
			_gaugeRuntimeMemInfo.WithLabelValues("Goroutines").Set(float64(stat.Metrics.GoroutineNum))
		}
	}()

	// go运行时内存 开销高
	go func() {
		for {
			time.Sleep(runtimeMemInterval)

			var m runtime.MemStats
			runtime.ReadMemStats(&m)

			// gc相关
			stat.Metrics.NumGC = m.NumGC

			// 内存相关
			stat.Metrics.TotalAlloc = m.TotalAlloc
			stat.Metrics.Alloc = m.Alloc
			stat.Metrics.Sys = m.Sys
			stat.Metrics.HeapInuse = m.HeapInuse
			stat.Metrics.HeapAlloc = m.HeapAlloc
			stat.Metrics.HeapSys = m.HeapSys
			stat.Metrics.HeapIdle = m.HeapIdle
			stat.Metrics.HeapReleased = m.HeapReleased

			_gaugeRuntimeMemInfo.WithLabelValues("TotalAlloc").Set(float64(m.TotalAlloc) / (1024 * 1024))
			_gaugeRuntimeMemInfo.WithLabelValues("Alloc").Set(float64(m.Alloc) / (1024 * 1024))
			_gaugeRuntimeMemInfo.WithLabelValues("Sys").Set(float64(m.Sys) / (1024 * 1024))
			_gaugeRuntimeMemInfo.WithLabelValues("NumGC").Set(float64(m.NumGC))
			_gaugeRuntimeMemInfo.WithLabelValues("HeapInuse").Set(float64(m.HeapInuse) / (1024 * 1024))
			_gaugeRuntimeMemInfo.WithLabelValues("HeapAlloc").Set(float64(m.HeapAlloc) / (1024 * 1024))
			_gaugeRuntimeMemInfo.WithLabelValues("HeapIdle").Set(float64(m.HeapIdle) / (1024 * 1024))
			_gaugeRuntimeMemInfo.WithLabelValues("HeapReleased").Set(float64(m.HeapReleased) / (1024 * 1024))

			log.Printf("CpuUsage: %d, MemUsage: %d, Goroutines: %d, Alloc: %vm, TotalAlloc: %vm, Sys: %vm, HeapAlloc: %vm, HeapInuse: %vm, HeapIdle: %vm, HeapReleased: %vm, NumGC: %v\n",
				stat.Metrics.CpuUsage,
				stat.Metrics.MemUsage,
				stat.Metrics.GoroutineNum,
				m.Alloc/(1024*1024),
				m.TotalAlloc/(1024*1024),
				m.Sys/(1024*1024),
				m.HeapAlloc/(1024*1024),
				m.HeapInuse/(1024*1024),
				m.HeapIdle/(1024*1024),
				m.HeapReleased/(1024*1024),
				m.NumGC)
		}
	}()
}
