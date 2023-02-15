package internal

import (
	"errors"
	"fmt"
	"os"
	"path"
	"strconv"
	"strings"
)

var (
	ErrorBadCgroupInfo = errors.New("bad cgroup info")
	ErrorNoCpusetDir   = errors.New("no cpuset dir")
	ErrorNoCpuacctDir  = errors.New("no cpuacct dir")
	ErrorNoCpuDir      = errors.New("no cpu dir")
)

type Cgroup struct {
	cgroupProcPath string
	cgroupFSPath   string

	data map[string]string
}

func NewCgroup() *Cgroup {
	return &Cgroup{
		cgroupProcPath: fmt.Sprintf("/proc/%d/cgroup", os.Getpid()),
		cgroupFSPath:   "/sys/fs/cgroup",
	}
}

/*
Init 获得当前进程所处的cgroup信息，目前只支持v1版本的cgroup

	cgroup 文件的一般格式

	10:cpuset:/
	9:pids:/system.slice/tuned.service
	8:blkio:/system.slice/tuned.service
	7:hugetlb:/
	6:devices:/system.slice/tuned.service
	5:net_prio,net_cls:/
	4:perf_event:/
	3:cpuacct,cpu:/system.slice/tuned.service
	2:freezer:/
	1:name=systemd:/system.slice/tuned.service
*/
func (c *Cgroup) Init() error {
	lines, err := ReadLines(c.cgroupProcPath)
	if err != nil {
		return err
	}

	if c.data == nil {
		c.data = make(map[string]string)
	}

	for _, line := range lines {
		cols := strings.Split(line, ":")
		if len(cols) != 3 {
			return ErrorBadCgroupInfo
		}
		if !strings.HasPrefix(cols[1], "cpu") {
			continue
		}

		dirNames := strings.Split(cols[1], ",")
		for _, dirName := range dirNames {
			c.data[dirName] = path.Join(c.cgroupFSPath, dirName)
		}
	}

	return nil
}

func (cgroup *Cgroup) GetUsage() (uint64, error) {
	basePath, ok := cgroup.data["cpuacct"]
	if !ok {
		return 0, ErrorNoCpuacctDir
	}

	fullPath := path.Join(basePath, "cpuacct.usage")

	line, err := ReadLine(fullPath)
	if err != nil {
		return 0, err
	}

	val, err := strconv.ParseUint(line, 10, 64)
	if err != nil {
		return 0, err
	}

	return val, nil
}

// GetQuotaUs 获取当前进程所属的cgroup的每个时间周期可使用的cpu时间数，单位us，-1代表全部cpu时间数
func (cgroup *Cgroup) GetQuotaUs() (int64, error) {
	basePath, ok := cgroup.data["cpu"]
	if !ok {
		return 0, ErrorNoCpuDir
	}

	fullPath := path.Join(basePath, "cpu.cfs_quota_us")

	line, err := ReadLine(fullPath)
	if err != nil {
		return 0, err
	}

	val, err := strconv.ParseInt(line, 10, 64)
	if err != nil {
		return 0, err
	}

	return val, nil
}

// GetPeriodUs 获取当前进程所属的cgroup的时间周期，能使用的cpu核心数=cpu时间数/时间周期，单位us
func (cgroup *Cgroup) GetPeriodUs() (uint64, error) {
	basePath, ok := cgroup.data["cpu"]
	if !ok {
		return 0, ErrorNoCpuDir
	}

	fullPath := path.Join(basePath, "cpu.cfs_period_us")

	line, err := ReadLine(fullPath)
	if err != nil {
		return 0, err
	}

	val, err := strconv.ParseUint(line, 10, 64)
	if err != nil {
		return 0, err
	}

	return val, nil
}

/*
GetCpus 获取当前进程所属的cgroup可使用的cpu核心编号，所谓亲和
格式为 0-3,6
*/
func (cgroup *Cgroup) GetCpus() ([]uint64, error) {
	basePath, ok := cgroup.data["cpuset"]
	if !ok {
		return nil, ErrorBadCPUStat
	}

	fullPath := path.Join(basePath, "cpuset.cpus")

	line, err := ReadLine(fullPath)
	if err != nil {
		return nil, err
	}

	list := make(map[uint64]struct{})
	cpuIdRanges := strings.Split(line, ",")
	for _, cpuIdRange := range cpuIdRanges {
		if strings.Contains(cpuIdRange, "-") {
			// cpu编号范围
			cpuIds := strings.SplitN(cpuIdRange, "-", 2)
			min, err := strconv.ParseUint(cpuIds[0], 10, 64)
			if err != nil {
				return nil, err
			}

			max, err := strconv.ParseUint(cpuIds[1], 10, 64)
			if err != nil {
				return nil, err
			}

			for i := min; i <= max; i++ {
				list[i] = struct{}{}
			}
		} else {
			// 单独的cpu编号
			v, err := strconv.ParseUint(cpuIdRange, 10, 64)
			if err != nil {
				return nil, err
			}

			list[v] = struct{}{}
		}
	}

	var cpus []uint64
	for k := range list {
		cpus = append(cpus, k)
	}

	return cpus, nil
}
