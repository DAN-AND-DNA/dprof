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

	_gaugeProcessMemUsage = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "process_mem_usage",
		Help: "进程内存使用率 比例为(1/1000)",
	})
)

type Stat struct {
	currentProcess *process.Process
	Registry       *prometheus.Registry
}

func New() *Stat {
	s := &Stat{}

	s.currentProcess, _ = process.NewProcess(int32(os.Getpid()))
	s.Registry = prometheus.NewRegistry()
	s.Registry.MustRegister(_gaugeProcessCpuUsage, _gaugeProcessMemUsage)

	return s
}

// MonitorProcess 监控进程信息
func (stat *Stat) MonitorProcess() {
	if stat.currentProcess == nil {
		return
	}

	go func() {
		for {
			time.Sleep(250 * time.Millisecond)

			// 拿进程cpu站系统总体cpu的比例
			cpuUsage, _ := stat.currentProcess.Percent(0)
			_gaugeProcessCpuUsage.Set(cpuUsage) // 千分之几

			// 拿进程的内存
			memUsage, _ := stat.currentProcess.MemoryPercent()
			_gaugeProcessMemUsage.Set(float64(memUsage * 10)) // 千分之几
		}
	}()
}
