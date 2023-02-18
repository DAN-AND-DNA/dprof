package internal

import (
	"fmt"
	"os"
	"path"
	"time"
)

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
