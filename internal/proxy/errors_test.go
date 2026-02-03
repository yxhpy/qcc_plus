package proxy

import (
	"errors"
	"testing"
)

func TestConfigError(t *testing.T) {
	t.Run("Error returns message", func(t *testing.T) {
		err := &ConfigError{msg: "test error message"}
		if err.Error() != "test error message" {
			t.Errorf("expected 'test error message', got '%s'", err.Error())
		}
	})

	t.Run("ErrUpstreamMissing is ConfigError", func(t *testing.T) {
		if ErrUpstreamMissing.Error() != "missing upstream base URL" {
			t.Errorf("expected 'missing upstream base URL', got '%s'", ErrUpstreamMissing.Error())
		}
	})

	t.Run("ErrNoActiveNode is standard error", func(t *testing.T) {
		if ErrNoActiveNode.Error() != "no active upstream node" {
			t.Errorf("expected 'no active upstream node', got '%s'", ErrNoActiveNode.Error())
		}
	})

	t.Run("ConfigError can be compared with errors.As", func(t *testing.T) {
		err := &ConfigError{msg: "test"}
		var target *ConfigError
		if !errors.As(err, &target) {
			t.Error("ConfigError should be identifiable with errors.As")
		}
		if target.msg != "test" {
			t.Errorf("expected message 'test', got '%s'", target.msg)
		}
	})
}
