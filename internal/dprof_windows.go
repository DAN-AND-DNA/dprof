package internal

func (d *dProf) DumpWhenCpuThreshold() {
	// TODO 检查cpu是否超过阈值且距离上次到达阈值已经过去30秒
	d.dumpChan <- dumpMsg{DumpType: DumpCPU}
}

func (d *dProf) DumpWhenSignal() {

	d.dumpChan <- dumpMsg{DumpType: DumpSignal}

}
