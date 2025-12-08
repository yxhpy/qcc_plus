//go:build windows

package proxy

import (
	"os/exec"
	"syscall"

	"golang.org/x/sys/windows"
)

// hideWindow 在 Windows 上设置进程属性以隐藏控制台窗口
// 使用增量设置，保留已有的 SysProcAttr 配置
func hideWindow(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	// 使用 windows.CREATE_NO_WINDOW 常量，避免魔法数字
	// CREATE_NO_WINDOW (0x08000000) 阻止创建控制台窗口
	cmd.SysProcAttr.CreationFlags |= windows.CREATE_NO_WINDOW
}
