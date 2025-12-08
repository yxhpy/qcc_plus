//go:build !windows

package proxy

import "os/exec"

// hideWindow 在非 Windows 平台上是空操作
func hideWindow(cmd *exec.Cmd) {
	// 非 Windows 平台不需要特殊处理
}
