//go:build windows

package osx

import (
	"fmt"
	"os/exec"
	"strconv"
	"syscall"

	"github.com/hankmor/vdd/core/logger"
)

// SuspendProcess 暂停进程 (Windows 暂不支持原生 Suspend，返回错误触发 fallback)
func SuspendProcess(cmd *exec.Cmd) error {
	return fmt.Errorf("windows suspend not supported yet")
}

// ResumeProcess 恢复进程
func ResumeProcess(cmd *exec.Cmd) error {
	return fmt.Errorf("windows resume not supported yet")
}

// SetProcessGroup 设置命令创建新的进程组
// 同时包含隐藏窗口标志，避免与 SetCmdHideWindow 冲突
func SetProcessGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow: true,
		// CREATE_NEW_PROCESS_GROUP (0x200) | CREATE_NO_WINDOW (0x08000000)
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP | 0x08000000,
	}
}

// killProcessTree 终止进程及其所有子进程
// Windows 使用 taskkill /T /F 命令终止进程树
func KillProcessTree(cmd *exec.Cmd) error {
	if cmd == nil || cmd.Process == nil {
		return nil
	}

	pid := cmd.Process.Pid
	logger.Debugf("[解析] 使用 taskkill 终止进程树: %d", pid)

	// 使用 taskkill /T /F /PID <pid> 终止进程树
	// /T = 终止所有子进程
	// /F = 强制终止
	killCmd := exec.Command("taskkill", "/T", "/F", "/PID", strconv.Itoa(pid))
	if err := killCmd.Run(); err != nil {
		// taskkill 失败时回退到直接 Kill
		logger.Debugf("[解析] taskkill 失败: %v, 回退到 Process.Kill()", err)
		return cmd.Process.Kill()
	}

	return nil
}
