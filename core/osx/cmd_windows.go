//go:build windows

package osx

import (
	"os/exec"
	"syscall"
)

// SetCmdHideWindow 设置命令在 Windows 上隐藏窗口
func SetCmdHideWindow(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: 0x08000000, // CREATE_NO_WINDOW
	}
}

// OpenURL 在 Windows 上打开 URL，避免控制台闪烁
func OpenURL(urlStr string) error {
	// 使用 rundll32 而不是 cmd /c start 以避免弹窗
	cmd := exec.Command("rundll32", "url.dll,FileProtocolHandler", urlStr)
	SetCmdHideWindow(cmd)
	return cmd.Start()
}
