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

/*
GetCpuUsage 拿全局范围的cpu使用

/proc/stat的文件内容格式

cpu  771 1 3697 11320785 1117 0 751 0 0 0
cpu0 443 0 1047 3761325 58 0 358 0 0 0
cpu1 230 0 1157 3780358 1018 0 15 0 0 0
cpu2 96 0 1492 3779101 41 0 377 0 0 0
*/
func GetCpuUsage() (uint64, error) {
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
