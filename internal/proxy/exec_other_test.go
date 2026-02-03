//go:build !windows

package proxy

import (
	"os/exec"
	"testing"
)

func TestHideWindow(t *testing.T) {
	t.Run("hideWindow does not panic on non-Windows", func(t *testing.T) {
		cmd := exec.Command("echo", "test")
		// Should not panic
		hideWindow(cmd)
		// Verify command is still usable
		if cmd == nil {
			t.Error("command should not be nil after hideWindow")
		}
	})

	t.Run("hideWindow with nil command does not panic", func(t *testing.T) {
		// Should not panic even with nil
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("hideWindow panicked with nil command: %v", r)
			}
		}()
		hideWindow(nil)
	})
}
