//go:build !windows

package osx

import (
	"errors"
	"os/exec"
)

// SetCmdHideWindow 设置命令在非 Windows 平台上隐藏窗口 (no-op)
func SetCmdHideWindow(cmd *exec.Cmd) {
	// 非 Windows 系统不需要特殊处理
}

// OpenURL 在非 Windows 上打开 URL (stub)
func OpenURL(urlStr string) error {
	return errors.New("not supported on this platform, use fyne app.OpenURL")
}
