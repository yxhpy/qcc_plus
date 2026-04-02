package store

// OfficialPricing 返回内置的多提供商官方模型定价目录。
// 维护基准：
// - Anthropic: https://docs.anthropic.com/en/docs/about-claude/pricing (2026-03-27)
// - OpenAI: https://developers.openai.com/api/docs/pricing / models/compare (2026-04-01)
// - Gemini: https://ai.google.dev/gemini-api/docs/pricing (2026-04-01)
//
// 注意：
// - Gemini 部分模型存在上下文分档、音视频和缓存价差；此处记录基础文本档位，便于页面展示与默认成本估算。
// - 渠道价、代理价和私有结算规则仍建议在管理台中手动覆盖。
func OfficialPricing() []ModelPricingRecord {
	return []ModelPricingRecord{
		// Anthropic
		{ModelID: "claude-opus-4-1-20250805", ModelName: "Claude Opus 4.1", InputPriceMTok: 15.0, OutputPriceMTok: 75.0, IsActive: true},
		{ModelID: "claude-opus-4-20250514", ModelName: "Claude Opus 4", InputPriceMTok: 15.0, OutputPriceMTok: 75.0, IsActive: true},
		{ModelID: "claude-sonnet-4-5-20250929", ModelName: "Claude Sonnet 4.5", InputPriceMTok: 3.0, OutputPriceMTok: 15.0, IsActive: true},
		{ModelID: "claude-sonnet-4-20250514", ModelName: "Claude Sonnet 4", InputPriceMTok: 3.0, OutputPriceMTok: 15.0, IsActive: true},
		{ModelID: "claude-3-7-sonnet-20250219", ModelName: "Claude Sonnet 3.7", InputPriceMTok: 3.0, OutputPriceMTok: 15.0, IsActive: true},
		{ModelID: "claude-haiku-4-5-20251001", ModelName: "Claude Haiku 4.5", InputPriceMTok: 1.0, OutputPriceMTok: 5.0, IsActive: true},
		{ModelID: "claude-3-5-sonnet-20241022", ModelName: "Claude 3.5 Sonnet", InputPriceMTok: 3.0, OutputPriceMTok: 15.0, IsActive: true},
		{ModelID: "claude-3-5-haiku-20241022", ModelName: "Claude 3.5 Haiku", InputPriceMTok: 0.8, OutputPriceMTok: 4.0, IsActive: true},
		{ModelID: "claude-3-opus-20240229", ModelName: "Claude 3 Opus", InputPriceMTok: 15.0, OutputPriceMTok: 75.0, IsActive: true},
		{ModelID: "claude-3-sonnet-20240229", ModelName: "Claude 3 Sonnet", InputPriceMTok: 3.0, OutputPriceMTok: 15.0, IsActive: true},
		{ModelID: "claude-3-haiku-20240307", ModelName: "Claude 3 Haiku", InputPriceMTok: 0.25, OutputPriceMTok: 1.25, IsActive: true},

		// OpenAI
		{ModelID: "gpt-5.4", ModelName: "GPT-5.4", InputPriceMTok: 2.5, OutputPriceMTok: 15.0, IsActive: true},
		{ModelID: "gpt-5.4-mini", ModelName: "GPT-5.4 mini", InputPriceMTok: 0.4, OutputPriceMTok: 2.0, IsActive: true},
		{ModelID: "gpt-5.4-nano", ModelName: "GPT-5.4 nano", InputPriceMTok: 0.1, OutputPriceMTok: 0.4, IsActive: true},
		{ModelID: "gpt-5.3-codex", ModelName: "GPT-5.3 Codex", InputPriceMTok: 1.75, OutputPriceMTok: 14.0, IsActive: true},
		{ModelID: "gpt-5.2-codex", ModelName: "GPT-5.2 Codex", InputPriceMTok: 1.75, OutputPriceMTok: 14.0, IsActive: true},
		{ModelID: "gpt-5.1-codex", ModelName: "GPT-5.1 Codex", InputPriceMTok: 1.25, OutputPriceMTok: 10.0, IsActive: true},
		{ModelID: "gpt-5.1-codex-max", ModelName: "GPT-5.1 Codex Max", InputPriceMTok: 1.25, OutputPriceMTok: 10.0, IsActive: true},
		{ModelID: "gpt-5.1-codex-mini", ModelName: "GPT-5.1 Codex mini", InputPriceMTok: 0.25, OutputPriceMTok: 2.0, IsActive: true},
		{ModelID: "gpt-5-mini", ModelName: "GPT-5 mini", InputPriceMTok: 0.25, OutputPriceMTok: 2.0, IsActive: true},
		{ModelID: "gpt-5-nano", ModelName: "GPT-5 nano", InputPriceMTok: 0.05, OutputPriceMTok: 0.4, IsActive: true},

		// Gemini (基础文本档位)
		{ModelID: "gemini-2.5-pro", ModelName: "Gemini 2.5 Pro", InputPriceMTok: 1.25, OutputPriceMTok: 10.0, IsActive: true},
		{ModelID: "gemini-2.5-flash", ModelName: "Gemini 2.5 Flash", InputPriceMTok: 0.3, OutputPriceMTok: 2.5, IsActive: true},
		{ModelID: "gemini-2.5-flash-preview-09-2025", ModelName: "Gemini 2.5 Flash Preview", InputPriceMTok: 0.3, OutputPriceMTok: 2.5, IsActive: true},
		{ModelID: "gemini-2.5-flash-lite", ModelName: "Gemini 2.5 Flash-Lite", InputPriceMTok: 0.1, OutputPriceMTok: 0.4, IsActive: true},
		{ModelID: "gemini-2.5-flash-lite-preview-09-2025", ModelName: "Gemini 2.5 Flash-Lite Preview", InputPriceMTok: 0.1, OutputPriceMTok: 0.4, IsActive: true},
		{ModelID: "gemini-2.0-flash", ModelName: "Gemini 2.0 Flash", InputPriceMTok: 0.1, OutputPriceMTok: 0.4, IsActive: true},
	}
}
