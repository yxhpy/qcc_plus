package proxy

import (
	"math"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

// RetryConfig 重试配置
type RetryConfig struct {
	MaxAttempts        int             // 最大重试次数（包含首次），默认 3
	BackoffMin         time.Duration   // 最小退避时间，默认 10ms
	BackoffMax         time.Duration   // 最大退避时间，默认 100ms
	RetryOnStatus      []int           // 需要重试的状态码，默认 [502, 503, 504]
	PerRequestTimeout  time.Duration   // 单次请求超时，默认 30s
	TotalTimeout       time.Duration   // 所有重试的总超时，0 表示关闭
	PerAttemptTimeouts []time.Duration // 覆盖每次尝试的超时，缺省使用 PerRequestTimeout
}

const (
	defaultRetryMaxAttempts = 3
	defaultBackoffMin       = 10 * time.Millisecond
	defaultBackoffMax       = 100 * time.Millisecond
	defaultPerRequestTO     = 30 * time.Second
)

// 从环境变量加载 RetryConfig
func loadRetryConfig() RetryConfig {
	cfg := RetryConfig{
		MaxAttempts:       defaultRetryMaxAttempts,
		BackoffMin:        defaultBackoffMin,
		BackoffMax:        defaultBackoffMax,
		RetryOnStatus:     []int{http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout},
		PerRequestTimeout: defaultPerRequestTO,
		TotalTimeout:      0,
	}

	if v := os.Getenv("RETRY_MAX_ATTEMPTS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			cfg.MaxAttempts = n
		}
	}

	if v := os.Getenv("RETRY_BACKOFF_MIN_MS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			cfg.BackoffMin = time.Duration(n) * time.Millisecond
		}
	}

	if v := os.Getenv("RETRY_BACKOFF_MAX_MS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			cfg.BackoffMax = time.Duration(n) * time.Millisecond
		}
	}

	if v := os.Getenv("RETRY_ON_STATUS"); v != "" {
		parts := strings.Split(v, ",")
		var codes []int
		for _, p := range parts {
			p = strings.TrimSpace(p)
			if p == "" {
				continue
			}
			if n, err := strconv.Atoi(p); err == nil {
				codes = append(codes, n)
			}
		}
		if len(codes) > 0 {
			cfg.RetryOnStatus = codes
		}
	}

	if v := os.Getenv("RETRY_PER_REQUEST_TIMEOUT_SEC"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			cfg.PerRequestTimeout = time.Duration(n) * time.Second
		}
	}

	if v := os.Getenv("RETRY_TOTAL_TIMEOUT_SEC"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			cfg.TotalTimeout = time.Duration(n) * time.Second
		}
	}

	if v := os.Getenv("RETRY_PER_ATTEMPT_TIMEOUTS_SEC"); v != "" {
		parts := strings.Split(v, ",")
		for _, p := range parts {
			p = strings.TrimSpace(p)
			if p == "" {
				continue
			}
			if n, err := strconv.Atoi(p); err == nil && n > 0 {
				cfg.PerAttemptTimeouts = append(cfg.PerAttemptTimeouts, time.Duration(n)*time.Second)
			}
		}
	}

	if cfg.BackoffMax < cfg.BackoffMin {
		cfg.BackoffMax = cfg.BackoffMin
	}

	return cfg
}

// calculateBackoff 计算带抖动的指数退避时间
// 公式：min(BackoffMin * 2^attempt + random jitter, BackoffMax)
func calculateBackoff(attempt int, cfg RetryConfig) time.Duration {
	if attempt < 0 {
		attempt = 0
	}
	if cfg.BackoffMin <= 0 {
		cfg.BackoffMin = defaultBackoffMin
	}
	if cfg.BackoffMax <= 0 {
		cfg.BackoffMax = defaultBackoffMax
	}

	base := float64(cfg.BackoffMin) * math.Pow(2, float64(attempt))
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	jitter := r.Float64() * float64(cfg.BackoffMin)

	backoff := time.Duration(base + jitter)
	if backoff > cfg.BackoffMax {
		return cfg.BackoffMax
	}
	return backoff
}

// shouldRetryStatus 判断 HTTP 状态码是否应该重试
func shouldRetryStatus(status int, cfg RetryConfig) bool {
	for _, s := range cfg.RetryOnStatus {
		if status == s {
			return true
		}
	}
	return false
}
