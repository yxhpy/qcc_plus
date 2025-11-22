package client

// Config holds runtime options for the CLI request flow.
type Config struct {
	Token       string
	BaseURL     string
	Model       string
	WarmupModel string
	NoWarmup    bool
	Minimal     bool
	UserHash    string
	Message     string
}

type Body struct {
	Model     string         `json:"model"`
	Messages  []Message      `json:"messages"`
	System    []SystemBlock  `json:"system,omitempty"`
	Tools     any            `json:"tools,omitempty"`
	Metadata  map[string]any `json:"metadata,omitempty"`
	MaxTokens int            `json:"max_tokens,omitempty"`
	Stream    bool           `json:"stream"`
}

type Message struct {
	Role    string        `json:"role"`
	Content []ContentItem `json:"content"`
}

type ContentItem struct {
	Type         string        `json:"type"`
	Text         string        `json:"text,omitempty"`
	CacheControl *CacheControl `json:"cache_control,omitempty"`
}

type CacheControl struct {
	Type string `json:"type"`
}

type SystemBlock struct {
	Type         string        `json:"type"`
	Text         string        `json:"text"`
	CacheControl *CacheControl `json:"cache_control,omitempty"`
}
