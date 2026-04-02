package proxy

import "qcc_plus/internal/store"

// RuntimeSettingApplyMode 描述配置项在运行中的生效方式。
type RuntimeSettingApplyMode string

const (
	RuntimeSettingApplyOnChange    RuntimeSettingApplyMode = "on_change"
	RuntimeSettingApplyReadThrough RuntimeSettingApplyMode = "read_through"
	RuntimeSettingApplyFrontend    RuntimeSettingApplyMode = "frontend_poll"
)

// RuntimeSettingDefinition 是系统级运行时配置的权威目录。
//
// 这里故意只收录“前端允许编辑且已经有明确生效语义”的配置：
//   - OnChange: 更新后通过 SettingsCache 回调立即推送到后端运行时。
//   - ReadThrough: 不需要主动推送，业务逻辑在每次读取时直接查缓存。
//   - Frontend: 由前端上下文消费，当前标签页立即更新，其他实例/标签页依赖轮询同步。
//
// 其他仅启动时读取的参数仍然通过环境变量管理，避免在 UI 中暴露出“能改但不生效”的伪配置。
type RuntimeSettingDefinition struct {
	Key            string                  `json:"key"`
	Label          string                  `json:"label"`
	Category       string                  `json:"category"`
	CategoryLabel  string                  `json:"category_label"`
	Description    string                  `json:"description"`
	DataType       string                  `json:"data_type"`
	DefaultValue   any                     `json:"default_value"`
	Unit           string                  `json:"unit,omitempty"`
	Min            *float64                `json:"min,omitempty"`
	Max            *float64                `json:"max,omitempty"`
	Step           *float64                `json:"step,omitempty"`
	Persisted      bool                    `json:"persisted"`
	HotReload      bool                    `json:"hot_reload"`
	ApplyMode      RuntimeSettingApplyMode `json:"apply_mode"`
	ApplyModeLabel string                  `json:"apply_mode_label"`
	SyncNote       string                  `json:"sync_note"`
}

var runtimeSettingDefinitions = []RuntimeSettingDefinition{
	{
		Key:            "proxy.retry_max",
		Label:          "代理失败重试次数",
		Category:       "performance",
		CategoryLabel:  "代理转发",
		Description:    "非 200 响应的节点切换重试上限。修改后立即影响新的转发请求。",
		DataType:       "number",
		DefaultValue:   3,
		Unit:           "次",
		Min:            floatPtr(1),
		Max:            floatPtr(10),
		Step:           floatPtr(1),
		Persisted:      true,
		HotReload:      true,
		ApplyMode:      RuntimeSettingApplyOnChange,
		ApplyModeLabel: "后端即时生效",
		SyncNote:       "当前实例立即更新；多实例部署时，其他实例会在 settings watcher 下一次刷新时同步。",
	},
	{
		Key:            "health.fail_threshold",
		Label:          "节点失败阈值",
		Category:       "health",
		CategoryLabel:  "健康检查",
		Description:    "节点连续失败达到该阈值后会进入故障状态，并参与恢复逻辑。",
		DataType:       "number",
		DefaultValue:   3,
		Unit:           "次",
		Min:            floatPtr(1),
		Max:            floatPtr(10),
		Step:           floatPtr(1),
		Persisted:      true,
		HotReload:      true,
		ApplyMode:      RuntimeSettingApplyOnChange,
		ApplyModeLabel: "后端即时生效",
		SyncNote:       "当前实例立即更新；多实例部署时，其他实例会在 settings watcher 下一次刷新时同步。",
	},
	{
		Key:            "health.check_interval_sec",
		Label:          "健康检查间隔",
		Category:       "health",
		CategoryLabel:  "健康检查",
		Description:    "后台定时健康检查的轮询间隔。修改后会直接影响后续调度周期。",
		DataType:       "number",
		DefaultValue:   30,
		Unit:           "秒",
		Min:            floatPtr(5),
		Max:            floatPtr(300),
		Step:           floatPtr(1),
		Persisted:      true,
		HotReload:      true,
		ApplyMode:      RuntimeSettingApplyOnChange,
		ApplyModeLabel: "后端即时生效",
		SyncNote:       "当前实例立即更新；多实例部署时，其他实例会在 settings watcher 下一次刷新时同步。",
	},
	{
		Key:            "health.skip_disabled_nodes",
		Label:          "跳过已禁用节点的健康检查",
		Category:       "health",
		CategoryLabel:  "健康检查",
		Description:    "开启后，手动禁用的节点不会再进入自动健康检查队列。",
		DataType:       "boolean",
		DefaultValue:   true,
		Persisted:      true,
		HotReload:      true,
		ApplyMode:      RuntimeSettingApplyReadThrough,
		ApplyModeLabel: "按次读取缓存",
		SyncNote:       "不需要额外推送，健康检查逻辑每次运行时都会重新读取当前值。",
	},
	{
		Key:            "monitor.hide_disabled_nodes",
		Label:          "监控页隐藏已禁用节点",
		Category:       "monitor",
		CategoryLabel:  "监控展示",
		Description:    "控制监控大屏默认是否展示已手动禁用的节点。",
		DataType:       "boolean",
		DefaultValue:   true,
		Persisted:      true,
		HotReload:      true,
		ApplyMode:      RuntimeSettingApplyReadThrough,
		ApplyModeLabel: "按次读取缓存",
		SyncNote:       "监控接口会直接读取缓存值，刷新监控页即可看到结果。",
	},
	{
		Key:            "monitor.refresh_interval_ms",
		Label:          "监控页自动刷新间隔",
		Category:       "monitor",
		CategoryLabel:  "监控展示",
		Description:    "前端监控大屏的自动刷新周期。当前标签页保存后立即生效，其他标签页会在 settings 轮询后同步。",
		DataType:       "number",
		DefaultValue:   30000,
		Unit:           "毫秒",
		Min:            floatPtr(5000),
		Max:            floatPtr(120000),
		Step:           floatPtr(1000),
		Persisted:      true,
		HotReload:      true,
		ApplyMode:      RuntimeSettingApplyFrontend,
		ApplyModeLabel: "前端轮询同步",
		SyncNote:       "当前标签页通过 SettingsContext 立即更新；其他标签页或实例会在下次轮询后同步。",
	},
}

func RuntimeSettingDefinitions() []RuntimeSettingDefinition {
	defs := make([]RuntimeSettingDefinition, len(runtimeSettingDefinitions))
	copy(defs, runtimeSettingDefinitions)
	return defs
}

func LookupRuntimeSettingDefinition(key string) (RuntimeSettingDefinition, bool) {
	for _, def := range runtimeSettingDefinitions {
		if def.Key == key {
			return def, true
		}
	}
	return RuntimeSettingDefinition{}, false
}

func RuntimeSettingsAppliedOnChange() []string {
	var keys []string
	for _, def := range runtimeSettingDefinitions {
		if def.ApplyMode == RuntimeSettingApplyOnChange {
			keys = append(keys, def.Key)
		}
	}
	return keys
}

func buildRuntimeSettingRecord(def RuntimeSettingDefinition, value any) *store.Setting {
	description := def.Description
	return &store.Setting{
		Key:         def.Key,
		Scope:       "system",
		Value:       value,
		DataType:    def.DataType,
		Category:    def.Category,
		Description: &description,
	}
}

func floatPtr(v float64) *float64 {
	return &v
}
