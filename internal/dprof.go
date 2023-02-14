package internal

import (
	"log"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"sync"
	"time"
)

var (
	singleInst *dProf = nil
	once       sync.Once
)

const (
	DumpNone   = 0
	DumpSignal = iota + 1000
	DumpCPU

	DumpEOF = 9999
)

type dumpMsg struct {
	DumpType  int
	StartUnix int64
}

/*
dProf

1. 根据信号量来手动输出pprof文件（需要手动关闭或杀死进程）

2. 根据cpu、内存的阈值自动输出pprof文件（持续5s）

3. io 请求超时，根据堆栈hash输出一份pprof文件

4. pprof可以自动告警或远程
*/
type dProf struct {
	isStarted     bool
	config        Config
	dumpClosers   map[string]func()
	signalChan    chan os.Signal
	dumpChan      chan dumpMsg
	lastStartUnix map[int]int64
	done          chan struct{}
}

type Config struct {
	BaseDir string
}

func GetSingleInst() *dProf {
	if singleInst == nil {
		once.Do(func() {
			singleInst = New().SetConfig()
		})
	}

	return singleInst
}

func New() *dProf {
	d := &dProf{
		signalChan:  make(chan os.Signal, 1),
		dumpChan:    make(chan dumpMsg, 2000),
		done:        make(chan struct{}),
		dumpClosers: make(map[string]func()),
	}

	var next <-chan time.Time
	go func() {
		for {
			select {
			case msg := <-d.dumpChan:
				switch msg.DumpType {
				case DumpSignal:
					// 手动 暂停全部
					if d.isStarted {
						d.stopDumpCPU()
					} else {
						d.dumpCPU()
						d.isStarted = true
					}

				case DumpCPU:
					next = time.After(5 * time.Second)
					d.dumpCPU()
				}
			case _ = <-next:
				// 暂停上次cpu
				d.stopDumpCPU()
			case _ = <-d.done:
				return
			}
		}
	}()

	return d
}
func (d *dProf) SetConfig(config ...Config) *dProf {
	// 默认配置
	defaultConfig := Config{
		BaseDir: "./",
	}

	// 用户配置覆盖原有的配置
	if len(config) != 0 {
		// 文件目录
		if config[0].BaseDir != "" {
			defaultConfig.BaseDir = config[0].BaseDir
		}
	}

	// TODO 改成atomic配置
	d.config = defaultConfig

	return d
}

// TryDumpCPU 尝试输出cpu pprof文件
func (d *dProf) dumpCPU() {
	defer recover1()

	kind := "cpu"
	f, err := d.createDumpFile(kind)
	if err != nil {
		panic(err)
	}

	// 开始采样
	err = pprof.StartCPUProfile(f)
	if err != nil {
		panic(err)
	}

	d.dumpClosers[kind] = func() {
		defer recover1()

		// 结束采样并写文件
		pprof.StopCPUProfile()
		f.Sync()
		err := f.Close()
		if err != nil {
			panic(err)
		}
	}
}

func (d *dProf) stopDumpCPU() {
	kind := "cpu"

	f, ok := d.dumpClosers[kind]
	if !ok {
		return
	}
	f()
}

// TryDumpBlock 尝试输出io阻塞 pprof文件
func (d *dProf) dumpBlock() error {
	kind := "block"
	f, err := d.createDumpFile(kind)
	if err != nil {
		return err
	}

	// 开始采样
	runtime.SetBlockProfileRate(1)

	d.dumpClosers[kind] = func() {
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
func (d *dProf) createDumpFile(kind string) (*os.File, error) {
	// 二进制路径
	var appName = "unknown_golang_app"
	if runtime.GOOS == "windows" {
		appName = path.Base(filepath.ToSlash(os.Args[0]))
	} else {
		appName = path.Base(os.Args[0])
	}

	basePath := d.config.BaseDir
	_ = basePath
	_ = appName
	//pprofPath := path.Join(basePath, fmt.Sprintf("%s-%d-%s-%s.pprof", appName, syscall.Getpid(), kind, time.Now().Format("2006-01-02 15:04:05")))
	pprofPath := "./ddd.prof"
	f, err := os.Create(pprofPath)
	if err != nil {
		// 直接崩比较好，输出堆栈比较好
		return nil, err
	}

	return f, nil
}
