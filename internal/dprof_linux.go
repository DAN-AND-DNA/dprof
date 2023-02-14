package internal

import (
	"os/signal"
	"syscall"
)

func (d *dProf) DumpWhenCpuThreshold() {
	// TODO 检查cpu是否超过阈值且距离上次到达阈值已经过去30秒
	d.dumpChan <- dumpMsg{DumpType: DumpCPU}
}

func (d *dProf) DumpWhenSignal() {
	signal.Notify(d.signalChan, syscall.SIGUSR1, syscall.SIGUSR2)
	go func() {
		for {
			select {
			case v := <-d.signalChan:
				switch v {
				case syscall.SIGUSR2:
					d.dumpChan <- dumpMsg{DumpType: DumpSignal}
				}
			case _ = <-d.done:
				return
			}
		}
	}()
}
