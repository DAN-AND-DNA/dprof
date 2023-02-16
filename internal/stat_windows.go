package internal

import (
	"errors"
	"github.com/shirou/gopsutil/process"
	"os"
)

var (
	ErrorBadCPUStat = errors.New("bad cpu stat of /proc/stat")
)

type StatImpl struct {
	p *process.Process
}

func NewStat() *StatImpl {
	stat := &StatImpl{}

	return stat
}

func (s *StatImpl) Init() error {
	var err error
	s.p, err = process.NewProcess(int32(os.Getpid()))
	if err != nil {
		return err
	}
	return nil
}

func (s *StatImpl) UpdateCpuUsage() uint64 {
	t, _ := s.p.Percent(0)
	return uint64(t)
}
