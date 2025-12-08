package proxy

import (
	"os"
	"strconv"
	"strings"
)

// EnvVarCategory 环境变量分类
type EnvVarCategory string

const (
	EnvCategoryBasic       EnvVarCategory = "basic"       // 基础配置
	EnvCategoryMultiTenant EnvVarCategory = "multiTenant" // 多租户与安全
	EnvCategoryCLI         EnvVarCategory = "cli"         // CLI 凭证
	EnvCategoryHealth      EnvVarCategory = "health"      // 健康检查
	EnvCategoryWarmup      EnvVarCategory = "warmup"      // 预热配置
	EnvCategoryRetry       EnvVarCategory = "retry"       // HTTP 重试
	EnvCategoryTransport   EnvVarCategory = "transport"   // 传输层连接池
	EnvCategoryCircuit     EnvVarCategory = "circuit"     // 熔断保护
	EnvCategoryMetrics     EnvVarCategory = "metrics"     // 指标调度
	EnvCategoryMySQL       EnvVarCategory = "mysql"       // MySQL 持久化
	EnvCategoryTunnel      EnvVarCategory = "tunnel"      // Cloudflare Tunnel
)

// EnvVarDefinition 环境变量定义
type EnvVarDefinition struct {
	Name         string         `json:"name"`          // 变量名
	Category     EnvVarCategory `json:"category"`      // 分类
	DefaultValue string         `json:"default_value"` // 默认值
	Description  string         `json:"description"`   // 中文说明
	IsSecret     bool           `json:"is_secret"`     // 是否敏感（脱敏显示）
	CurrentValue string         `json:"current_value"` // 当前值
	IsSet        bool           `json:"is_set"`        // 是否已设置
}

// EnvVarCategoryInfo 分类信息
type EnvVarCategoryInfo struct {
	Key         EnvVarCategory `json:"key"`
	Label       string         `json:"label"`
	Description string         `json:"description"`
}

// GetEnvVarCategories 获取所有分类信息
func GetEnvVarCategories() []EnvVarCategoryInfo {
	return []EnvVarCategoryInfo{
		{Key: EnvCategoryBasic, Label: "基础配置", Description: "代理服务器的基本配置，包括监听地址、上游地址等"},
		{Key: EnvCategoryMultiTenant, Label: "多租户与安全", Description: "多租户架构相关的安全配置"},
		{Key: EnvCategoryCLI, Label: "CLI 凭证", Description: "Claude CLI 相关的凭证和模型配置"},
		{Key: EnvCategoryHealth, Label: "健康检查", Description: "节点健康检查和故障检测配置"},
		{Key: EnvCategoryWarmup, Label: "预热配置", Description: "服务预热相关的配置"},
		{Key: EnvCategoryRetry, Label: "HTTP 重试", Description: "HTTP 请求重试策略配置"},
		{Key: EnvCategoryTransport, Label: "传输层连接池", Description: "HTTP 传输层连接池配置"},
		{Key: EnvCategoryCircuit, Label: "熔断保护", Description: "熔断器相关的保护配置"},
		{Key: EnvCategoryMetrics, Label: "指标调度", Description: "监控指标聚合和清理配置"},
		{Key: EnvCategoryMySQL, Label: "MySQL 持久化", Description: "MySQL 数据库连接和连接池配置"},
		{Key: EnvCategoryTunnel, Label: "Cloudflare Tunnel", Description: "内网穿透隧道配置"},
	}
}

// GetAllEnvVarDefinitions 获取所有环境变量定义
func GetAllEnvVarDefinitions() []EnvVarDefinition {
	definitions := []EnvVarDefinition{
		// ========== 基础配置 ==========
		{Name: "LISTEN_ADDR", Category: EnvCategoryBasic, DefaultValue: ":8000", Description: "代理服务器监听地址"},
		{Name: "UPSTREAM_BASE_URL", Category: EnvCategoryBasic, DefaultValue: "https://api.anthropic.com", Description: "上游 API 基础地址"},
		{Name: "UPSTREAM_API_KEY", Category: EnvCategoryBasic, DefaultValue: "", Description: "默认上游 API Key（可选）", IsSecret: true},
		{Name: "UPSTREAM_NAME", Category: EnvCategoryBasic, DefaultValue: "default", Description: "默认节点名称"},
		{Name: "PROXY_HEALTH_CHECK_MODE", Category: EnvCategoryBasic, DefaultValue: "cli", Description: "全局健康检查方式（cli/api/head）"},

		// ========== 多租户与安全 ==========
		{Name: "ADMIN_API_KEY", Category: EnvCategoryMultiTenant, DefaultValue: "admin", Description: "管理员访问密钥（⚠️ 生产必改）", IsSecret: true},
		{Name: "DEFAULT_ACCOUNT_NAME", Category: EnvCategoryMultiTenant, DefaultValue: "default", Description: "默认账号名称"},
		{Name: "DEFAULT_PROXY_API_KEY", Category: EnvCategoryMultiTenant, DefaultValue: "default-proxy-key", Description: "默认代理 API Key（⚠️ 生产必改）", IsSecret: true},

		// ========== CLI 凭证 ==========
		{Name: "ANTHROPIC_API_KEY", Category: EnvCategoryCLI, DefaultValue: "", Description: "Anthropic API Key（可与 UPSTREAM_API_KEY 二选一）", IsSecret: true},
		{Name: "ANTHROPIC_AUTH_TOKEN", Category: EnvCategoryCLI, DefaultValue: "", Description: "Anthropic Auth Token（可替代 API Key）", IsSecret: true},
		{Name: "ANTHROPIC_BASE_URL", Category: EnvCategoryCLI, DefaultValue: "https://api.anthropic.com", Description: "Anthropic Base URL"},
		{Name: "OPENAI_API_KEY", Category: EnvCategoryCLI, DefaultValue: "", Description: "OpenAI SDK 兼容 Key（可选）", IsSecret: true},
		{Name: "MODEL", Category: EnvCategoryCLI, DefaultValue: "claude-sonnet-4-5-20250929", Description: "CLI 默认模型"},
		{Name: "WARMUP_MODEL", Category: EnvCategoryCLI, DefaultValue: "claude-haiku-4-5-20251001", Description: "预热使用的模型"},
		{Name: "NO_WARMUP", Category: EnvCategoryCLI, DefaultValue: "0", Description: "关闭 CLI 预热（1=跳过，0=启用）"},
		{Name: "MINIMAL_SYSTEM", Category: EnvCategoryCLI, DefaultValue: "1", Description: "使用精简系统提示（1=精简，0=完整）"},
		{Name: "USER_HASH", Category: EnvCategoryCLI, DefaultValue: "", Description: "自定义用户哈希（不填则基于 Token 计算）"},

		// ========== 健康检查 ==========
		{Name: "PROXY_RETRY_MAX", Category: EnvCategoryHealth, DefaultValue: "3", Description: "非 200 自动重试次数"},
		{Name: "PROXY_FAIL_THRESHOLD", Category: EnvCategoryHealth, DefaultValue: "3", Description: "连续失败多少次标记节点不可用"},
		{Name: "PROXY_HEALTH_INTERVAL_SEC", Category: EnvCategoryHealth, DefaultValue: "30", Description: "失败节点探活间隔（秒）"},
		{Name: "PROXY_HEALTH_CHECK_ALL_INTERVAL", Category: EnvCategoryHealth, DefaultValue: "10m", Description: "全量健康检查间隔（优先使用）"},
		{Name: "HEALTH_ALL_INTERVAL_MIN", Category: EnvCategoryHealth, DefaultValue: "10", Description: "全量健康检查间隔（分钟，备选）"},
		{Name: "HEALTH_CHECK_CONCURRENCY", Category: EnvCategoryHealth, DefaultValue: "2", Description: "全量健康检查并发数（HEAD/API，自动限制 1~4）"},
		{Name: "HEALTH_CHECK_CONCURRENCY_CLI", Category: EnvCategoryHealth, DefaultValue: "1", Description: "CLI 健康检查并发数（建议 1~2）"},

		// ========== 预热配置 ==========
		{Name: "WARMUP_ENABLED", Category: EnvCategoryWarmup, DefaultValue: "1", Description: "预热开关（1=启用，0=关闭）"},
		{Name: "WARMUP_ATTEMPTS", Category: EnvCategoryWarmup, DefaultValue: "2", Description: "预热尝试次数"},
		{Name: "WARMUP_TIMEOUT_MS", Category: EnvCategoryWarmup, DefaultValue: "17000", Description: "单次预热超时（毫秒）"},
		{Name: "WARMUP_REQUIRED_SUCCESS", Category: EnvCategoryWarmup, DefaultValue: "1", Description: "至少成功次数"},
		{Name: "WARMUP_CONCURRENCY", Category: EnvCategoryWarmup, DefaultValue: "1", Description: "预热并发数（最大 2）"},

		// ========== HTTP 重试 ==========
		{Name: "RETRY_MAX_ATTEMPTS", Category: EnvCategoryRetry, DefaultValue: "3", Description: "最大重试次数（包含首次）"},
		{Name: "RETRY_PER_REQUEST_TIMEOUT_SEC", Category: EnvCategoryRetry, DefaultValue: "30", Description: "单次请求超时（秒）"},
		{Name: "RETRY_TOTAL_TIMEOUT_SEC", Category: EnvCategoryRetry, DefaultValue: "0", Description: "总超时时间（秒，0=关闭）"},
		{Name: "RETRY_PER_ATTEMPT_TIMEOUTS_SEC", Category: EnvCategoryRetry, DefaultValue: "", Description: "每次尝试超时（逗号分隔，如 12,6,3）"},
		{Name: "RETRY_BACKOFF_MIN_MS", Category: EnvCategoryRetry, DefaultValue: "10", Description: "最小退避时间（毫秒）"},
		{Name: "RETRY_BACKOFF_MAX_MS", Category: EnvCategoryRetry, DefaultValue: "100", Description: "最大退避时间（毫秒）"},
		{Name: "RETRY_ON_STATUS", Category: EnvCategoryRetry, DefaultValue: "502,503,504", Description: "需要重试的 HTTP 状态码"},

		// ========== 传输层连接池 ==========
		{Name: "PROXY_TRANSPORT_MAX_IDLE_CONNS", Category: EnvCategoryTransport, DefaultValue: "200", Description: "最大空闲连接数"},
		{Name: "PROXY_TRANSPORT_MAX_IDLE_CONNS_PER_HOST", Category: EnvCategoryTransport, DefaultValue: "50", Description: "每个主机最大空闲连接数"},
		{Name: "PROXY_TRANSPORT_MAX_CONNS_PER_HOST", Category: EnvCategoryTransport, DefaultValue: "100", Description: "每个主机最大连接数"},
		{Name: "PROXY_TRANSPORT_IDLE_CONN_TIMEOUT", Category: EnvCategoryTransport, DefaultValue: "90s", Description: "空闲连接超时"},
		{Name: "PROXY_TRANSPORT_TLS_HANDSHAKE_TIMEOUT", Category: EnvCategoryTransport, DefaultValue: "10s", Description: "TLS 握手超时"},
		{Name: "PROXY_TRANSPORT_RESPONSE_HEADER_TIMEOUT", Category: EnvCategoryTransport, DefaultValue: "30s", Description: "响应头超时"},
		{Name: "PROXY_TRANSPORT_EXPECT_CONTINUE_TIMEOUT", Category: EnvCategoryTransport, DefaultValue: "1s", Description: "Expect Continue 超时"},
		{Name: "PROXY_TRANSPORT_DISABLE_COMPRESSION", Category: EnvCategoryTransport, DefaultValue: "false", Description: "禁用压缩"},
		{Name: "PROXY_TRANSPORT_FORCE_HTTP2", Category: EnvCategoryTransport, DefaultValue: "true", Description: "强制使用 HTTP/2"},

		// ========== 熔断保护 ==========
		{Name: "CB_ENABLED", Category: EnvCategoryCircuit, DefaultValue: "1", Description: "启用熔断器（1=启用，0=关闭）"},
		{Name: "CB_WINDOW_SECONDS", Category: EnvCategoryCircuit, DefaultValue: "60", Description: "滑动窗口大小（秒）"},
		{Name: "CB_FAILURE_RATE", Category: EnvCategoryCircuit, DefaultValue: "0.5", Description: "失败率阈值（0-1）"},
		{Name: "CB_CONSECUTIVE_FAILS", Category: EnvCategoryCircuit, DefaultValue: "5", Description: "连续失败次数阈值"},
		{Name: "CB_COOLDOWN_SECONDS", Category: EnvCategoryCircuit, DefaultValue: "30", Description: "冷却时间（秒）"},
		{Name: "CB_HALFOPEN_MAX_CALLS", Category: EnvCategoryCircuit, DefaultValue: "3", Description: "半开状态最大试探次数"},

		// ========== 指标调度 ==========
		{Name: "METRICS_SCHEDULER_ENABLED", Category: EnvCategoryMetrics, DefaultValue: "1", Description: "启用指标调度器（需持久化）"},
		{Name: "METRICS_AGGREGATE_INTERVAL", Category: EnvCategoryMetrics, DefaultValue: "1h", Description: "指标聚合间隔"},
		{Name: "METRICS_CLEANUP_INTERVAL", Category: EnvCategoryMetrics, DefaultValue: "24h", Description: "指标清理间隔"},

		// ========== MySQL 持久化 ==========
		{Name: "PROXY_MYSQL_DSN", Category: EnvCategoryMySQL, DefaultValue: "", Description: "MySQL 连接字符串（启用持久化）", IsSecret: true},
		{Name: "MYSQL_MAX_OPEN_CONNS", Category: EnvCategoryMySQL, DefaultValue: "25", Description: "最大打开连接数"},
		{Name: "MYSQL_MAX_IDLE_CONNS", Category: EnvCategoryMySQL, DefaultValue: "10", Description: "最大空闲连接数"},
		{Name: "MYSQL_CONN_MAX_LIFETIME", Category: EnvCategoryMySQL, DefaultValue: "300", Description: "连接最大生命周期（秒）"},
		{Name: "MYSQL_CONN_MAX_IDLE_TIME", Category: EnvCategoryMySQL, DefaultValue: "180", Description: "连接最大空闲时间（秒）"},

		// ========== Cloudflare Tunnel ==========
		{Name: "CF_API_TOKEN", Category: EnvCategoryTunnel, DefaultValue: "", Description: "Cloudflare API Token", IsSecret: true},
		{Name: "TUNNEL_SUBDOMAIN", Category: EnvCategoryTunnel, DefaultValue: "", Description: "隧道子域名"},
		{Name: "TUNNEL_ZONE", Category: EnvCategoryTunnel, DefaultValue: "", Description: "Cloudflare Zone（域名）"},
		{Name: "TUNNEL_ENABLED", Category: EnvCategoryTunnel, DefaultValue: "0", Description: "启用隧道功能（1=启用，0=关闭）"},
	}

	// 填充当前值，并对敏感变量进行脱敏
	for i := range definitions {
		def := &definitions[i]
		val, exists := os.LookupEnv(def.Name)
		def.IsSet = exists

		// 敏感变量的默认值也需要隐藏
		if def.IsSecret && def.DefaultValue != "" {
			def.DefaultValue = "********"
		}

		if exists {
			if def.IsSecret && val != "" {
				def.CurrentValue = maskSecret(val)
			} else {
				def.CurrentValue = val
			}
		} else {
			// 未设置时，敏感变量隐藏默认值
			if def.IsSecret {
				def.CurrentValue = "(未设置)"
			} else {
				def.CurrentValue = def.DefaultValue
			}
		}
	}

	return definitions
}

// GetEnvVarsByCategory 按分类获取环境变量
func GetEnvVarsByCategory(category EnvVarCategory) []EnvVarDefinition {
	all := GetAllEnvVarDefinitions()
	var result []EnvVarDefinition
	for _, def := range all {
		if def.Category == category {
			result = append(result, def)
		}
	}
	return result
}

// maskSecret 脱敏显示敏感值
func maskSecret(value string) string {
	if len(value) <= 8 {
		return "********"
	}
	return value[:4] + "****" + value[len(value)-4:]
}

// GetEnvString 获取环境变量字符串值
func GetEnvString(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

// GetEnvInt 获取环境变量整数值
func GetEnvInt(key string, defaultVal int) int {
	if val := os.Getenv(key); val != "" {
		if i, err := strconv.Atoi(val); err == nil {
			return i
		}
	}
	return defaultVal
}

// GetEnvBool 获取环境变量布尔值
func GetEnvBool(key string, defaultVal bool) bool {
	if val := os.Getenv(key); val != "" {
		val = strings.ToLower(val)
		return val == "1" || val == "true" || val == "yes"
	}
	return defaultVal
}
