package proxy

import "errors"

// ConfigError 表示构建配置问题。
type ConfigError struct{ msg string }

func (e *ConfigError) Error() string { return e.msg }

var (
	ErrUpstreamMissing = &ConfigError{"missing upstream base URL"}
	ErrNoActiveNode    = errors.New("no active upstream node")
)
