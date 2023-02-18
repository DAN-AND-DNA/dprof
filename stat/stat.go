package stat

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/shirou/gopsutil/process"
	"os"
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
)

type Metrics struct {
	CpuUsage      int64
	PrevCpuUsage1 int64
	PrevCpuUsage2 int64

	MemUsage      int64
	PrevMemUsage1 int64
	PrevMemUsage2 int64
}

type Stat struct {
	currentProcess *process.Process

	Registry *prometheus.Registry
	Metrics  Metrics
}

func New() *Stat {
	s := &Stat{}

	s.currentProcess, _ = process.NewProcess(int32(os.Getpid()))
	s.Registry = prometheus.NewRegistry()
	s.Registry.MustRegister(_gaugeProcessCpuUsage, _gaugeProcessRecentCpuUsageLevel, _gaugeProcessMemUsage, _gaugeProcessRecentMemUsage)

	return s
}

// MonitorProcess 监控进程信息
func (stat *Stat) MonitorProcess() {
	if stat.currentProcess == nil {
		return
	}

	var c1, c2, c3 float64
	var m1, m2, m3 float64

	go func() {
		for {
			time.Sleep(250 * time.Millisecond)

			// 拿进程cpu站系统总体cpu的比例
			cpuUsage, _ := stat.currentProcess.Percent(0)
			_gaugeProcessCpuUsage.Set(cpuUsage) // 千分之几

			c1, c2, c3 = cpuUsage, c1, c2
			stat.Metrics.CpuUsage = int64(c1)
			stat.Metrics.PrevCpuUsage1 = int64(c2)
			stat.Metrics.PrevCpuUsage2 = int64(c3)

			_gaugeProcessRecentCpuUsageLevel.WithLabelValues("0ms").Set(c1)
			_gaugeProcessRecentCpuUsageLevel.WithLabelValues("250ms").Set(c2)
			_gaugeProcessRecentCpuUsageLevel.WithLabelValues("500ms").Set(c3)

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
