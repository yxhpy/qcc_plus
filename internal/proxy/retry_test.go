package proxy

import (
	"net/http"
	"testing"
	"time"
)

// TestCalculateBackoff 测试指数退避计算
func TestCalculateBackoff(t *testing.T) {
	cfg := RetryConfig{
		BackoffMin: 10 * time.Millisecond,
		BackoffMax: 100 * time.Millisecond,
	}

	tests := []struct {
		name        string
		attempt     int
		expectMin   time.Duration
		expectMax   time.Duration
		description string
	}{
		{
			name:        "first attempt",
			attempt:     0,
			expectMin:   10 * time.Millisecond,
			expectMax:   30 * time.Millisecond, // 10*2^0 + jitter(0-10ms)
			description: "第一次重试，基础退避 10ms",
		},
		{
			name:        "second attempt",
			attempt:     1,
			expectMin:   20 * time.Millisecond,
			expectMax:   50 * time.Millisecond, // 10*2^1 + jitter(0-10ms)
			description: "第二次重试，基础退避 20ms",
		},
		{
			name:        "third attempt",
			attempt:     2,
			expectMin:   40 * time.Millisecond,
			expectMax:   100 * time.Millisecond, // 10*2^2 + jitter，但不超过 100ms
			description: "第三次重试，基础退避 40ms",
		},
		{
			name:        "capped at max",
			attempt:     5,
			expectMin:   100 * time.Millisecond,
			expectMax:   100 * time.Millisecond, // 超过最大值，限制为 100ms
			description: "超过最大退避时间，限制为 100ms",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			backoff := calculateBackoff(tt.attempt, cfg)

			if backoff < tt.expectMin {
				t.Errorf("backoff %v < expected min %v", backoff, tt.expectMin)
			}
			if backoff > tt.expectMax {
				t.Errorf("backoff %v > expected max %v", backoff, tt.expectMax)
			}

			t.Logf("%s: attempt %d -> backoff %v", tt.description, tt.attempt, backoff)
		})
	}
}

// TestCalculateBackoffEdgeCases 测试边界情况
func TestCalculateBackoffEdgeCases(t *testing.T) {
	t.Run("negative attempt", func(t *testing.T) {
		cfg := RetryConfig{
			BackoffMin: 10 * time.Millisecond,
			BackoffMax: 100 * time.Millisecond,
		}

		backoff := calculateBackoff(-1, cfg)
		if backoff < 10*time.Millisecond || backoff > 30*time.Millisecond {
			t.Errorf("negative attempt should be treated as 0, got %v", backoff)
		}
	})

	t.Run("zero backoff min", func(t *testing.T) {
		cfg := RetryConfig{
			BackoffMin: 0,
			BackoffMax: 100 * time.Millisecond,
		}

		backoff := calculateBackoff(0, cfg)
		if backoff < defaultBackoffMin {
			t.Errorf("should fallback to default min, got %v", backoff)
		}
	})

	t.Run("backoff max < backoff min", func(t *testing.T) {
		cfg := RetryConfig{
			BackoffMin: 100 * time.Millisecond,
			BackoffMax: 10 * time.Millisecond, // 错误配置
		}

		backoff := calculateBackoff(0, cfg)
		// 应该立即返回最大值（即使它比最小值小）
		if backoff > cfg.BackoffMax {
			t.Errorf("backoff should be capped at max, got %v", backoff)
		}
	})
}

// TestShouldRetryStatus 测试状态码判断
func TestShouldRetryStatus(t *testing.T) {
	cfg := RetryConfig{
		RetryOnStatus: []int{http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout},
	}

	tests := []struct {
		status      int
		shouldRetry bool
		description string
	}{
		{http.StatusBadGateway, true, "502 Bad Gateway - 应该重试"},
		{http.StatusServiceUnavailable, true, "503 Service Unavailable - 应该重试"},
		{http.StatusGatewayTimeout, true, "504 Gateway Timeout - 应该重试"},
		{http.StatusInternalServerError, false, "500 Internal Server Error - 不在重试列表"},
		{http.StatusBadRequest, false, "400 Bad Request - 客户端错误，不重试"},
		{http.StatusUnauthorized, false, "401 Unauthorized - 认证错误，不重试"},
		{http.StatusNotFound, false, "404 Not Found - 不重试"},
		{http.StatusOK, false, "200 OK - 成功，不需要重试"},
	}

	for _, tt := range tests {
		t.Run(tt.description, func(t *testing.T) {
			result := shouldRetryStatus(tt.status, cfg)
			if result != tt.shouldRetry {
				t.Errorf("status %d: expected shouldRetry=%v, got %v", tt.status, tt.shouldRetry, result)
			}
		})
	}
}

// TestRetryConfigDefaults 测试默认配置
func TestRetryConfigDefaults(t *testing.T) {
	// 清空环境变量
	t.Setenv("RETRY_MAX_ATTEMPTS", "")
	t.Setenv("RETRY_BACKOFF_MIN_MS", "")
	t.Setenv("RETRY_BACKOFF_MAX_MS", "")
	t.Setenv("RETRY_ON_STATUS", "")

	cfg := loadRetryConfig()

	if cfg.MaxAttempts != defaultRetryMaxAttempts {
		t.Errorf("expected MaxAttempts=%d, got %d", defaultRetryMaxAttempts, cfg.MaxAttempts)
	}

	if cfg.BackoffMin != defaultBackoffMin {
		t.Errorf("expected BackoffMin=%v, got %v", defaultBackoffMin, cfg.BackoffMin)
	}

	if cfg.BackoffMax != defaultBackoffMax {
		t.Errorf("expected BackoffMax=%v, got %v", defaultBackoffMax, cfg.BackoffMax)
	}

	expectedStatus := []int{http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout}
	if len(cfg.RetryOnStatus) != len(expectedStatus) {
		t.Errorf("expected %d retry status codes, got %d", len(expectedStatus), len(cfg.RetryOnStatus))
	}

	for i, status := range expectedStatus {
		if cfg.RetryOnStatus[i] != status {
			t.Errorf("expected RetryOnStatus[%d]=%d, got %d", i, status, cfg.RetryOnStatus[i])
		}
	}
}

// TestRetryConfigFromEnv 测试从环境变量加载配置
func TestRetryConfigFromEnv(t *testing.T) {
	t.Setenv("RETRY_MAX_ATTEMPTS", "5")
	t.Setenv("RETRY_BACKOFF_MIN_MS", "20")
	t.Setenv("RETRY_BACKOFF_MAX_MS", "200")
	t.Setenv("RETRY_ON_STATUS", "500,502,503")
	t.Setenv("RETRY_PER_REQUEST_TIMEOUT_SEC", "60")

	cfg := loadRetryConfig()

	if cfg.MaxAttempts != 5 {
		t.Errorf("expected MaxAttempts=5, got %d", cfg.MaxAttempts)
	}

	if cfg.BackoffMin != 20*time.Millisecond {
		t.Errorf("expected BackoffMin=20ms, got %v", cfg.BackoffMin)
	}

	if cfg.BackoffMax != 200*time.Millisecond {
		t.Errorf("expected BackoffMax=200ms, got %v", cfg.BackoffMax)
	}

	if cfg.PerRequestTimeout != 60*time.Second {
		t.Errorf("expected PerRequestTimeout=60s, got %v", cfg.PerRequestTimeout)
	}

	expectedStatus := []int{500, 502, 503}
	if len(cfg.RetryOnStatus) != len(expectedStatus) {
		t.Errorf("expected %d retry status codes, got %d", len(expectedStatus), len(cfg.RetryOnStatus))
	}

	for i, status := range expectedStatus {
		if cfg.RetryOnStatus[i] != status {
			t.Errorf("expected RetryOnStatus[%d]=%d, got %d", i, status, cfg.RetryOnStatus[i])
		}
	}
}

// TestRetryConfigInvalidEnv 测试无效的环境变量
func TestRetryConfigInvalidEnv(t *testing.T) {
	t.Setenv("RETRY_MAX_ATTEMPTS", "invalid")
	t.Setenv("RETRY_BACKOFF_MIN_MS", "-1")
	t.Setenv("RETRY_ON_STATUS", "abc,xyz")

	cfg := loadRetryConfig()

	// 无效值应该回退到默认值
	if cfg.MaxAttempts != defaultRetryMaxAttempts {
		t.Errorf("invalid MaxAttempts should fallback to default, got %d", cfg.MaxAttempts)
	}

	if cfg.BackoffMin != defaultBackoffMin {
		t.Errorf("negative BackoffMin should fallback to default, got %v", cfg.BackoffMin)
	}

	// 无效的状态码列表应该回退到默认值
	expectedStatus := []int{http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout}
	if len(cfg.RetryOnStatus) != len(expectedStatus) {
		t.Errorf("invalid RetryOnStatus should fallback to default, got %v", cfg.RetryOnStatus)
	}
}

// TestRetryConfigBackoffMaxLessThanMin 测试 BackoffMax < BackoffMin 的情况
func TestRetryConfigBackoffMaxLessThanMin(t *testing.T) {
	t.Setenv("RETRY_BACKOFF_MIN_MS", "100")
	t.Setenv("RETRY_BACKOFF_MAX_MS", "50")

	cfg := loadRetryConfig()

	// BackoffMax 应该被调整为 >= BackoffMin
	if cfg.BackoffMax < cfg.BackoffMin {
		t.Errorf("BackoffMax (%v) should be >= BackoffMin (%v)", cfg.BackoffMax, cfg.BackoffMin)
	}
}
