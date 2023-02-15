package internal

import (
	"errors"
	"strconv"
	"strings"
	"time"
)

var (
	ErrorBadCPUStat = errors.New("bad cpu stat of /proc/stat")
)

type Stat struct {
	cores              uint64
	coresQuota         float64
	prevSystemCpuUsage uint64
	prevCgroupCpuUsage uint64
	cgroup             *Cgroup
}

func NewStat() *Stat {
	stat := &Stat{
		cgroup: NewCgroup(),
	}

	return stat
}

func (s *Stat) Init() error {
	err := s.cgroup.Init()
	if err != nil {
		return err
	}

	// cgroup 限制的可用cpu数
	cpus, err := s.cgroup.GetCpus()
	if err != nil {
		return err
	}

	s.cores = uint64(len(cpus))
	s.coresQuota = float64(s.cores)

	quota, err := s.cgroup.GetQuotaUs()
	if err != nil {
		return err
	}

	// 无cpu时间片上的限制
	if quota != -1 {
		period, err := s.cgroup.GetPeriodUs()
		if err != nil {
			return err

		}

		// 计算时间片换算成的核心数（可能是一个核心的百分比，相当于一个cpu的时间片上限）
		cpuTimeCoresQuota := float64(quota) / float64(period)
		if cpuTimeCoresQuota < s.coresQuota {
			s.coresQuota = cpuTimeCoresQuota
		}
	}

	// 系统总的cpu使用情况
	s.prevSystemCpuUsage, err = GetSystemCpuUsage()
	if err != nil {
		return err
	}

	// 当前进程的cpu使用情况
	s.prevCgroupCpuUsage, err = s.cgroup.GetUsage()
	if err != nil {
		return err
	}

	return nil
}

func (s *Stat) UpdateCpuUsage() uint64 {
	total, err := s.cgroup.GetUsage()
	if err != nil {
		return 0
	}

	system, err := GetSystemCpuUsage()
	if err != nil {
		return 0
	}

	var usage uint64
	cpuDelta := total - s.prevCgroupCpuUsage
	systemDelta := system - s.prevSystemCpuUsage
	if cpuDelta > 0 && systemDelta > 0 {
		usage = uint64(float64(cpuDelta*s.cores*1e3) / (float64(systemDelta) * s.coresQuota))
	}

	s.prevSystemCpuUsage = system
	s.prevCgroupCpuUsage = total

	return usage
}

/*
GetCpuUsage 拿全局范围的cpu使用

/proc/stat的文件内容格式

cpu  771 1 3697 11320785 1117 0 751 0 0 0
cpu0 443 0 1047 3761325 58 0 358 0 0 0
cpu1 230 0 1157 3780358 1018 0 15 0 0 0
cpu2 96 0 1492 3779101 41 0 377 0 0 0
*/
func GetSystemCpuUsage() (uint64, error) {
	lines, err := ReadLines("/proc/stat")
	if err != nil {
		return 0, err
	}

	for _, line := range lines {
		fields := strings.Fields(line)
		if fields[0] == "cpu" {
			if len(fields) < 8 {
				return 0, ErrorBadCPUStat
			}

			var totalClockTicks uint64
			for _, strVal := range fields[1:] {
				val, err := strconv.ParseUint(strVal, 10, 64)
				if err != nil {
					return 0, err
				}

				totalClockTicks += val
			}

			return (totalClockTicks * uint64(time.Second)) / 100, nil
		}
	}

	return 0, ErrorBadCPUStat
}
