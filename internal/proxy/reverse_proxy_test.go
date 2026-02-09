package proxy

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestRetryTransport_RoundTrip_Success(t *testing.T) {
	// Create a test server that returns 200
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	}))
	defer server.Close()

	transport := &retryTransport{
		base:     http.DefaultTransport,
		attempts: 3,
	}

	req, _ := http.NewRequest("GET", server.URL, nil)
	resp, err := transport.RoundTrip(req)

	if err != nil {
		t.Fatalf("RoundTrip() error = %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("StatusCode = %d, want 200", resp.StatusCode)
	}
}

func TestRetryTransport_RoundTrip_Retry(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts < 2 {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("error"))
		} else {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("success"))
		}
	}))
	defer server.Close()

	transport := &retryTransport{
		base:     http.DefaultTransport,
		attempts: 3,
		logger:   log.New(io.Discard, "", 0), // Discard logs to avoid nil pointer
	}

	req, _ := http.NewRequest("GET", server.URL, nil)
	resp, err := transport.RoundTrip(req)

	if err != nil {
		t.Fatalf("RoundTrip() error = %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("StatusCode = %d, want 200", resp.StatusCode)
	}
	if attempts != 2 {
		t.Errorf("attempts = %d, want 2", attempts)
	}
}

func TestRetryTransport_RoundTrip_ContextCanceled(t *testing.T) {
	transport := &retryTransport{
		base:     http.DefaultTransport,
		attempts: 3,
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	req, _ := http.NewRequestWithContext(ctx, "GET", "http://example.com", nil)
	_, err := transport.RoundTrip(req)

	if !errors.Is(err, context.Canceled) {
		t.Errorf("RoundTrip() error = %v, want context.Canceled", err)
	}
}

func TestRetryTransport_RoundTrip_WithBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if string(body) != "test body" {
			t.Errorf("body = %s, want 'test body'", string(body))
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	transport := &retryTransport{
		base:     http.DefaultTransport,
		attempts: 1,
	}

	req, _ := http.NewRequest("POST", server.URL, strings.NewReader("test body"))
	_, err := transport.RoundTrip(req)

	if err != nil {
		t.Fatalf("RoundTrip() error = %v", err)
	}
}

func TestUsageReader_Read(t *testing.T) {
	data := []byte("test data")
	reader := &usageReader{
		ReadCloser: io.NopCloser(bytes.NewReader(data)),
		buf:        &bytes.Buffer{},
		tracker:    &usage{},
	}

	buf := make([]byte, 4)
	n, err := reader.Read(buf)

	if err != nil {
		t.Fatalf("Read() error = %v", err)
	}
	if n != 4 {
		t.Errorf("Read() n = %d, want 4", n)
	}
	if reader.buf.Len() != 4 {
		t.Errorf("buffer length = %d, want 4", reader.buf.Len())
	}
}

func TestUsageReader_Close(t *testing.T) {
	jsonData := `{"usage":{"input_tokens":100,"output_tokens":200}}`
	reader := &usageReader{
		ReadCloser: io.NopCloser(strings.NewReader(jsonData)),
		buf:        bytes.NewBufferString(jsonData),
		tracker:    &usage{},
	}

	err := reader.Close()
	if err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	if reader.tracker.input != 100 {
		t.Errorf("input tokens = %d, want 100", reader.tracker.input)
	}
	if reader.tracker.output != 200 {
		t.Errorf("output tokens = %d, want 200", reader.tracker.output)
	}
}

func TestStreamState(t *testing.T) {
	state := &streamState{}

	// Test initial state
	if state.enabled() {
		t.Error("initial state should be false")
	}

	// Test set true
	state.set(true)
	if !state.enabled() {
		t.Error("state should be true after set(true)")
	}

	// Test set false
	state.set(false)
	if state.enabled() {
		t.Error("state should be false after set(false)")
	}

	// Test nil state
	var nilState *streamState
	if nilState.enabled() {
		t.Error("nil state should return false")
	}
	nilState.set(true) // Should not panic
}

func TestBoolLike(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  bool
	}{
		{"1", "1", true},
		{"true", "true", true},
		{"TRUE", "TRUE", true},
		{"yes", "yes", true},
		{"on", "on", true},
		{"0", "0", false},
		{"false", "false", false},
		{"no", "no", false},
		{"off", "off", false},
		{"invalid", "invalid", false},
		{"empty", "", false},
		{"with spaces", " true ", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := boolLike(tt.value)
			if got != tt.want {
				t.Errorf("boolLike(%s) = %v, want %v", tt.value, got, tt.want)
			}
		})
	}
}

func TestStreamFlagEnabled(t *testing.T) {
	tests := []struct {
		name  string
		value interface{}
		want  bool
	}{
		{"bool true", true, true},
		{"bool false", false, false},
		{"string true", "true", true},
		{"string 1", "1", true},
		{"string false", "false", false},
		{"int", 123, false},
		{"nil", nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := streamFlagEnabled(tt.value)
			if got != tt.want {
				t.Errorf("streamFlagEnabled(%v) = %v, want %v", tt.value, got, tt.want)
			}
		})
	}
}

func TestIsStreamRequest(t *testing.T) {
	tests := []struct {
		name string
		req  *http.Request
		want bool
	}{
		{
			name: "nil request",
			req:  nil,
			want: false,
		},
		{
			name: "query param stream=true",
			req:  httptest.NewRequest("GET", "/?stream=true", nil),
			want: true,
		},
		{
			name: "header stream=1",
			req: func() *http.Request {
				req := httptest.NewRequest("GET", "/", nil)
				req.Header.Set("stream", "1")
				return req
			}(),
			want: true,
		},
		{
			name: "header x-stream=yes",
			req: func() *http.Request {
				req := httptest.NewRequest("GET", "/", nil)
				req.Header.Set("x-stream", "yes")
				return req
			}(),
			want: true,
		},
		{
			name: "accept text/event-stream",
			req: func() *http.Request {
				req := httptest.NewRequest("GET", "/", nil)
				req.Header.Set("Accept", "text/event-stream")
				return req
			}(),
			want: true,
		},
		{
			name: "no stream indicators",
			req:  httptest.NewRequest("GET", "/", nil),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isStreamRequest(tt.req)
			if got != tt.want {
				t.Errorf("isStreamRequest() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFirstByteFlusher_Write(t *testing.T) {
	recorder := httptest.NewRecorder()
	state := &streamState{}
	state.set(true)

	flusher := &firstByteFlusher{
		ResponseWriter: recorder,
		flusher:        recorder,
		state:          state,
	}

	n, err := flusher.Write([]byte("test"))
	if err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	if n != 4 {
		t.Errorf("Write() n = %d, want 4", n)
	}
	if !flusher.flushed {
		t.Error("flushed should be true after first write")
	}
}

func TestWrapFirstByteFlush(t *testing.T) {
	recorder := httptest.NewRecorder()
	state := &streamState{}

	// Test with valid flusher
	wrapped := wrapFirstByteFlush(recorder, state)
	if _, ok := wrapped.(*firstByteFlusher); !ok {
		t.Error("wrapFirstByteFlush should return *firstByteFlusher")
	}

	// Test with nil state
	wrapped = wrapFirstByteFlush(recorder, nil)
	if wrapped != recorder {
		t.Error("wrapFirstByteFlush with nil state should return original writer")
	}
}

func TestParseUsage(t *testing.T) {
	tests := []struct {
		name       string
		data       []byte
		wantInput  int64
		wantOutput int64
	}{
		{
			name:       "valid usage",
			data:       []byte(`{"usage":{"input_tokens":100,"output_tokens":200}}`),
			wantInput:  100,
			wantOutput: 200,
		},
		{
			name:       "no usage field",
			data:       []byte(`{"result":"success"}`),
			wantInput:  0,
			wantOutput: 0,
		},
		{
			name:       "invalid json",
			data:       []byte(`invalid json`),
			wantInput:  0,
			wantOutput: 0,
		},
		{
			name:       "empty data",
			data:       []byte(``),
			wantInput:  0,
			wantOutput: 0,
		},
		{
			name:       "usage in SSE format",
			data:       []byte(`data: {"usage":{"input_tokens":50,"output_tokens":75}}`),
			wantInput:  50,
			wantOutput: 75,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input, output := parseUsage(tt.data)
			if input != tt.wantInput {
				t.Errorf("parseUsage() input = %d, want %d", input, tt.wantInput)
			}
			if output != tt.wantOutput {
				t.Errorf("parseUsage() output = %d, want %d", output, tt.wantOutput)
			}
		})
	}
}

func TestServer_newReverseProxy(t *testing.T) {
	nodeURL, _ := url.Parse("https://api.example.com")
	node := &Node{
		ID:     "test-node",
		Name:   "Test Node",
		URL:    nodeURL,
		APIKey: "test-key",
	}

	srv := &Server{
		transport: http.DefaultTransport,
	}

	u := &usage{}
	proxy, state := srv.newReverseProxy(node, u)

	if proxy == nil {
		t.Fatal("newReverseProxy should return non-nil proxy")
	}
	if state == nil {
		t.Fatal("newReverseProxy should return non-nil state")
	}
}

func TestServer_newPassthroughProxy(t *testing.T) {
	nodeURL, _ := url.Parse("https://api.example.com")
	node := &Node{
		ID:     "test-node",
		Name:   "Test Node",
		URL:    nodeURL,
		APIKey: "test-key",
	}

	srv := &Server{
		transport: http.DefaultTransport,
	}

	proxy := srv.newPassthroughProxy(node)

	if proxy == nil {
		t.Fatal("newPassthroughProxy should return non-nil proxy")
	}
}

func TestParseModelFromRequest(t *testing.T) {
	tests := []struct {
		name string
		body []byte
		want string
	}{
		{
			name: "valid model",
			body: []byte(`{"model":"claude-3-opus-20240229"}`),
			want: "claude-3-opus-20240229",
		},
		{
			name: "no model field",
			body: []byte(`{"messages":[]}`),
			want: "",
		},
		{
			name: "invalid json",
			body: []byte(`invalid`),
			want: "",
		},
		{
			name: "empty body",
			body: []byte(``),
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseModelFromRequest(tt.body)
			if got != tt.want {
				t.Errorf("parseModelFromRequest() = %s, want %s", got, tt.want)
			}
		})
	}
}
