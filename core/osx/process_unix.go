//go:build !windows

package osx

import (
	"os/exec"
	"syscall"
	"time"

	"github.com/hankmor/vdd/core/logger"
)

// SuspendProcess 暂停进程 (发送 SIGSTOP 到进程组)
func SuspendProcess(cmd *exec.Cmd) error {
	if cmd == nil || cmd.Process == nil {
		return nil
	}
	// 发送信号给进程组 (PID 为负数表示进程组 ID)
	// 注意: 需要在启动命令时设置 Setpgid
	pgid, err := syscall.Getpgid(cmd.Process.Pid)
	if err == nil {
		return syscall.Kill(-pgid, syscall.SIGSTOP)
	}
	// 如果获取不到 PGID，尝试只挂起父进程
	return cmd.Process.Signal(syscall.SIGSTOP)
}

// ResumeProcess 恢复进程 (发送 SIGCONT 到进程组)
func ResumeProcess(cmd *exec.Cmd) error {
	if cmd == nil || cmd.Process == nil {
		return nil
	}
	pgid, err := syscall.Getpgid(cmd.Process.Pid)
	if err == nil {
		return syscall.Kill(-pgid, syscall.SIGCONT)
	}
	return cmd.Process.Signal(syscall.SIGCONT)
}

// SetProcessGroup 设置命令为新的进程组组长
func SetProcessGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

// KillProcessTree 终止进程及其所有子进程
// 使用分阶段终止策略：先 SIGTERM，等待后 SIGKILL
func KillProcessTree(cmd *exec.Cmd) error {
	if cmd == nil || cmd.Process == nil {
		return nil
	}

	pid := cmd.Process.Pid

	// 当 Setpgid=true 时，进程成为新进程组组长，pgid == pid
	// 使用负数发送信号到整个进程组
	pgid := pid

	// 第一阶段：发送 SIGTERM 到进程组（优雅终止）
	logger.Debugf("[进程] 发送 SIGTERM 到进程组: %d", pgid)
	if err := syscall.Kill(-pgid, syscall.SIGTERM); err != nil {
		logger.Debugf("[进程] SIGTERM 失败: %v, 尝试直接 Kill", err)
		// 如果进程组信号失败，直接杀死主进程
		return cmd.Process.Kill()
	}

	// 短暂等待进程响应（100ms 足够了）
	time.Sleep(100 * time.Millisecond)

	// 第二阶段：发送 SIGKILL 强制终止
	logger.Debugf("[进程] 发送 SIGKILL 到进程组: %d", pgid)
	if err := syscall.Kill(-pgid, syscall.SIGKILL); err != nil {
		// 进程可能已经退出，忽略错误
		logger.Debugf("[进程] SIGKILL 结果: %v (进程可能已退出)", err)
	}

	return nil
}
