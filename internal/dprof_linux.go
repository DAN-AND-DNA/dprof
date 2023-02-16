package internal

import (
	"fmt"
	"os"
	"os/signal"
	"path"
	"syscall"
	"time"
)

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

// createDumpFile 尝试创建dump文件
func (d *dProf) createDumpFile(kind string) (*os.File, error) {
	// 二进制路径
	appName := path.Base(os.Args[0])

	pprofPath := fmt.Sprintf("./%s-%d-%s-%s.pprof", appName, os.Getpid(), kind, time.Now().Format("2006-01-02_15-04-05"))
	f, err := os.Create(pprofPath)
	if err != nil {
		// 直接崩比较好，输出堆栈比较好
		return nil, err
	}

	return f, nil
}
