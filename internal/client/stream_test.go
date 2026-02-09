package client

import (
	"bytes"
	"strings"
	"testing"
)

func TestStream_ContentBlockDelta(t *testing.T) {
	input := `event: content_block_delta
data: {"delta":{"text":"Hello"}}

event: content_block_delta
data: {"delta":{"text":" World"}}

`
	reader := strings.NewReader(input)
	err := stream(reader)
	if err != nil {
		t.Errorf("stream() error = %v", err)
	}
}

func TestStream_MessageDelta(t *testing.T) {
	input := `event: message_delta
data: {"usage":{"input_tokens":10,"output_tokens":20}}

`
	reader := strings.NewReader(input)
	err := stream(reader)
	if err != nil {
		t.Errorf("stream() error = %v", err)
	}
}

func TestStream_MixedEvents(t *testing.T) {
	input := `event: message_start
data: {"type":"message_start"}

event: content_block_start
data: {"type":"content_block_start"}

event: content_block_delta
data: {"delta":{"text":"Test"}}

event: content_block_stop
data: {"type":"content_block_stop"}

event: message_delta
data: {"usage":{"output_tokens":5}}

event: message_stop
data: {"type":"message_stop"}

`
	reader := strings.NewReader(input)
	err := stream(reader)
	if err != nil {
		t.Errorf("stream() error = %v", err)
	}
}

func TestStream_MultilineData(t *testing.T) {
	input := `event: content_block_delta
data: {"delta":{"text":"Line 1"}}
data: {"delta":{"text":"Line 2"}}

`
	reader := strings.NewReader(input)
	err := stream(reader)
	if err != nil {
		t.Errorf("stream() error = %v", err)
	}
}

func TestStream_EmptyInput(t *testing.T) {
	input := ""
	reader := strings.NewReader(input)
	err := stream(reader)
	if err != nil {
		t.Errorf("stream() error = %v", err)
	}
}

func TestStream_OnlyNewlines(t *testing.T) {
	input := "\n\n\n"
	reader := strings.NewReader(input)
	err := stream(reader)
	if err != nil {
		t.Errorf("stream() error = %v", err)
	}
}

func TestStream_InvalidJSON(t *testing.T) {
	// Invalid JSON should not cause error, just skip processing
	input := `event: content_block_delta
data: {invalid json}

`
	reader := strings.NewReader(input)
	err := stream(reader)
	if err != nil {
		t.Errorf("stream() error = %v", err)
	}
}

func TestStream_MissingDeltaField(t *testing.T) {
	input := `event: content_block_delta
data: {"other":"field"}

`
	reader := strings.NewReader(input)
	err := stream(reader)
	if err != nil {
		t.Errorf("stream() error = %v", err)
	}
}

func TestStream_MissingTextField(t *testing.T) {
	input := `event: content_block_delta
data: {"delta":{"other":"field"}}

`
	reader := strings.NewReader(input)
	err := stream(reader)
	if err != nil {
		t.Errorf("stream() error = %v", err)
	}
}

func TestStream_LargeBuffer(t *testing.T) {
	// Test with large content to ensure buffer handling works
	largeText := strings.Repeat("A", 100000)
	input := `event: content_block_delta
data: {"delta":{"text":"` + largeText + `"}}

`
	reader := strings.NewReader(input)
	err := stream(reader)
	if err != nil {
		t.Errorf("stream() error = %v", err)
	}
}

func TestStream_NoEventType(t *testing.T) {
	// Data without event type should be handled gracefully
	input := `data: {"delta":{"text":"Test"}}

`
	reader := strings.NewReader(input)
	err := stream(reader)
	if err != nil {
		t.Errorf("stream() error = %v", err)
	}
}

func TestStream_EventWithoutData(t *testing.T) {
	input := `event: content_block_delta

`
	reader := strings.NewReader(input)
	err := stream(reader)
	if err != nil {
		t.Errorf("stream() error = %v", err)
	}
}

func TestStream_ComplexSSESequence(t *testing.T) {
	input := `event: message_start
data: {"type":"message_start","message":{"id":"msg_123","type":"message","role":"assistant"}}

event: content_block_start
data: {"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}

event: ping
data: {"type":"ping"}

event: content_block_delta
data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Hello"}}

event: content_block_delta
data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":" there"}}

event: content_block_stop
data: {"type":"content_block_stop","index":0}

event: message_delta
data: {"type":"message_delta","delta":{"stop_reason":"end_turn","stop_sequence":null},"usage":{"output_tokens":2}}

event: message_stop
data: {"type":"message_stop"}

`
	reader := strings.NewReader(input)
	err := stream(reader)
	if err != nil {
		t.Errorf("stream() error = %v", err)
	}
}

func TestStream_ReadError(t *testing.T) {
	// Create a reader that will return an error
	reader := &errorReader{err: bytes.ErrTooLarge}
	err := stream(reader)
	if err != bytes.ErrTooLarge {
		t.Errorf("stream() error = %v, want %v", err, bytes.ErrTooLarge)
	}
}

// errorReader is a helper type that always returns an error
type errorReader struct {
	err error
}

func (e *errorReader) Read(p []byte) (n int, err error) {
	return 0, e.err
}
