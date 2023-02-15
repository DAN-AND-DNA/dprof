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
	ErrorBadCGroupV1File = errors.New("bad cgroup v1 file")
)

type CGroup struct {
	data map[string]string
}

/*
GetCGroup 获得当前进程所处的cgroup信息，目前只支持v1版本的cgroup

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
func GetCGroup() (*CGroup, error) {
	cgroupPath := fmt.Sprintf("/proc/%d/cgroup", os.Getpid())

	lines, err := ReadLines(cgroupPath)
	if err != nil {
		return nil, err
	}

	ret := &CGroup{
		data: make(map[string]string),
	}

	for _, line := range lines {
		cols := strings.Split(line, ":")
		if len(cols) != 3 {
			return nil, ErrorBadCGroupV1File
		}
		if !strings.HasPrefix(cols[1], "cpu") {
			continue
		}

		dirNames := strings.Split(cols[1], ",")
		for _, dirName := range dirNames {
			ret.data[dirName] = path.Join("/sys/fs/cgroup", dirName)
		}
	}

	return ret, nil
}

func (cgroup *CGroup) GetUsage() (uint64, error) {
	basePath, ok := cgroup.data["cpuacct"]
	if !ok {
		return 0, nil
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

// GetQuotaUs 获取当前进程所属的cgroup的每个时间周期可使用的cpu时间数，单位us
func (cgroup *CGroup) GetQuotaUs() (uint64, error) {
	basePath, ok := cgroup.data["cpu"]
	if !ok {
		return 0, nil
	}

	fullPath := path.Join(basePath, "cpu.cfs_quota_us")

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

// GetPeriodUs 获取当前进程所属的cgroup的时间周期，能使用的cpu核心数=cpu时间数/时间周期，单位us
func (cgroup *CGroup) GetPeriodUs() (uint64, error) {
	basePath, ok := cgroup.data["cpu"]
	if !ok {
		return 0, nil
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

// GetCpus 获取当前进程所属的cgroup可使用的cpu核心编号
func (cgroup *CGroup) GetCpus() (uint64, error) {
	basePath, ok := cgroup.data["cpuset"]
	if !ok {
		return 0, nil
	}

	fullPath := path.Join(basePath, "cpuset.cpus")

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
