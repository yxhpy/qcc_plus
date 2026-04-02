package protocol

// CanonicalRequest is the protocol-agnostic request contract used internally.
type CanonicalRequest struct {
	RequestID  string
	AccountID  string
	ModelAlias string
	Stream     bool
	Messages   []CanonicalMessage
	Tools      []CanonicalTool
	ToolChoice *CanonicalToolChoice
	Options    CanonicalOptions
	Metadata   map[string]string
}

// CanonicalMessage is a role-based message with typed parts.
type CanonicalMessage struct {
	Role  string
	Parts []CanonicalPart
}

// CanonicalPart supports text/tool and future multimodal extensions.
type CanonicalPart struct {
	Type       string
	Text       string
	MIMEType   string
	Data       []byte
	ToolCall   *CanonicalToolCall
	ToolResult *CanonicalToolResult
}

// CanonicalTool is a unified tool schema.
type CanonicalTool struct {
	Name        string
	Description string
	InputSchema map[string]any
}

// CanonicalToolChoice represents routing policy for tool invocation.
type CanonicalToolChoice struct {
	Type string
	Name string
}

// CanonicalToolCall is a normalized function invocation.
type CanonicalToolCall struct {
	ID    string
	Name  string
	Input map[string]any
}

// CanonicalToolResult is tool execution output.
type CanonicalToolResult struct {
	ToolCallID string
	Output     string
	IsError    bool
}

// CanonicalUsage is normalized token accounting.
type CanonicalUsage struct {
	InputTokens         int64
	OutputTokens        int64
	CacheCreationTokens int64
	CacheReadTokens     int64
}

// CanonicalResponse is the final non-streaming output contract.
type CanonicalResponse struct {
	Content      []CanonicalPart
	ToolCalls    []CanonicalToolCall
	Usage        CanonicalUsage
	FinishReason string
}

// CanonicalStreamEventType defines normalized stream event labels.
type CanonicalStreamEventType string

const (
	EventContentStart CanonicalStreamEventType = "content_start"
	EventContentDelta CanonicalStreamEventType = "content_delta"
	EventContentStop  CanonicalStreamEventType = "content_stop"

	EventToolUseStart CanonicalStreamEventType = "tool_use_start"
	EventToolUseDelta CanonicalStreamEventType = "tool_use_delta"
	EventToolUseStop  CanonicalStreamEventType = "tool_use_stop"

	EventComplete CanonicalStreamEventType = "complete"
	EventError    CanonicalStreamEventType = "error"
)

// CanonicalToolCallDelta is incremental tool-call stream payload.
type CanonicalToolCallDelta struct {
	ID          string
	Name        string
	InputChunk  string
	InputObject map[string]any
}

// CanonicalStreamEvent is normalized stream payload.
type CanonicalStreamEvent struct {
	Type          CanonicalStreamEventType
	Text          string
	ToolCallDelta *CanonicalToolCallDelta
	Usage         *CanonicalUsage
	Error         *NormalizedError
}

// CanonicalOptions keeps a protocol-neutral option subset for MVP.
type CanonicalOptions struct {
	MaxTokens   int64
	Temperature *float64
	TopP        *float64
	Stop        []string
	JSONMode    bool
}
