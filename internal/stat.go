package internal

type Stat interface {
	Init() error            // 初始化
	UpdateCpuUsage() uint64 // 更新cpu使用率
}
