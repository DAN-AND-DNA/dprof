package dprof

import (
	"errors"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"syscall"
	"time"
)

var (
	ErrorCallNewFirst = errors.New("need call function New() to create a new DProf")
)

/*
DProf

1. 根据信号量来手动输出pprof文件（需要手动关闭或杀死进程）

2. 根据cpu、内存的阈值自动输出pprof文件（持续5s - 10s）

3. io 请求超时，根据堆栈hash输出一份pprof文件

4. pprof可以自动告警或远程
*/
type DProf struct {
	isInit bool

	config      Config
	dumpClosers map[string]func()
}

type Config struct {
	BasePath string
}

func New(config ...Config) *DProf {
	// 默认配置
	defaultConfig := Config{
		BasePath: "./",
	}

	if len(config) != 0 {
		defaultConfig.BasePath = config[0].BasePath
	}

	return &DProf{
		isInit: true,
		config: defaultConfig,
	}
}

func DumpWhenSignal(dProf *DProf, signals ...os.Signal) {
	if dProf == nil {
		return
	}

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, signals...)

	// TODO windows

}

func (dProf *DProf) start() {
	defer recover1()

	// 剖析 cpu
	err := dProf.dumpCPU()
	if err != nil {
		panic(err)
	}

	// 剖析 io 阻塞
	err = dProf.dumpBlock()
	if err != nil {
		panic(err)
	}
}

func (dProf *DProf) stop() {
	defer recover1()

	err := validate(dProf)
	if err != nil {
		panic(err)
	}

	for kind, closer := range dProf.dumpClosers {
		log.Printf("stop dump %s \n", kind)
		closer()
	}
}

func validate(dProf *DProf) error {
	if dProf == nil || !dProf.isInit {
		return ErrorCallNewFirst
	}

	return nil
}

// TryDumpCPU 尝试输出cpu pprof文件
func (dProf *DProf) dumpCPU() error {
	kind := "cpu"
	f, err := dProf.createDumpFile(kind)
	if err != nil {
		return err
	}

	// 开始采样
	err = pprof.StartCPUProfile(f)
	if err != nil {
		return err
	}

	if dProf.dumpClosers == nil {
		dProf.dumpClosers = make(map[string]func())
	}

	dProf.dumpClosers[kind] = func() {
		defer recover1()

		// 结束采样并写文件
		pprof.StopCPUProfile()
		err := f.Close()
		if err != nil {
			panic(err)
		}
	}

	return nil
}

// TryDumpBlock 尝试输出io阻塞 pprof文件
func (dProf *DProf) dumpBlock() error {
	kind := "block"
	f, err := dProf.createDumpFile(kind)
	if err != nil {
		return err
	}

	// 开始采样
	runtime.SetBlockProfileRate(1)

	if dProf.dumpClosers == nil {
		dProf.dumpClosers = make(map[string]func())
	}

	dProf.dumpClosers[kind] = func() {
		defer recover1()

		// 结束采样
		runtime.SetBlockProfileRate(0)
		// 写文件
		err := pprof.Lookup(kind).WriteTo(f, 0)
		if err != nil {
			panic(err)
		}

		err = f.Close()
		if err != nil {
			panic(err)
		}
	}

	return nil
}

func recover1() {
	if err := recover(); err != nil {
		var buf = make([]byte, 1024)

		runtime.Stack(buf, false)
		log.Printf("error: %v \n%s", err, string(buf))
	}
}

// createDumpFile 尝试创建dump文件
func (dProf *DProf) createDumpFile(kind string) (*os.File, error) {
	err := validate(dProf)
	if err != nil {
		return nil, err
	}

	// 二进制路径
	var appName = "unknown_golang_app"
	if runtime.GOOS == "windows" {
		appName = path.Base(filepath.ToSlash(os.Args[0]))
	} else {
		appName = path.Base(os.Args[0])
	}

	basePath := dProf.config.BasePath
	pprofPath := path.Join(basePath, fmt.Sprintf("%s-%d-%s-%s.pprof", appName, syscall.Getpid(), kind, time.Now().Format("2006-01-02 15:04:05")))

	f, err := os.Create(pprofPath)
	if err != nil {
		// 直接崩比较好，输出堆栈比较好
		return nil, err
	}

	return f, nil
}
