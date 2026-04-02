package proxy

import (
	"bytes"
	"net/http"
	"sync"
)

// retryBufferWriter 在重试场景下缓冲响应，避免将中间节点的错误返回给客户端。
// 当请求成功或确定为最终失败时，调用 Flush() 将缓冲内容写入真正的 ResponseWriter。
// 如果需要重试，调用 Reset() 丢弃缓冲区。
//
// 流式模式（streaming=true）：当上游返回 200 + SSE 数据时，第一次 Flush() 会自动
// 触发 FlushToReal()，将数据立即透传给客户端。这避免了流式请求被无限缓冲导致客户端
// 看不到任何输出而"卡住"的问题。
type retryBufferWriter struct {
	real      http.ResponseWriter
	header    http.Header
	body      bytes.Buffer
	status    int
	flushed   bool // 是否已经 flush 到真正的 writer
	streaming bool // 是否为流式模式（SSE）
	mu        sync.Mutex
}

func newRetryBufferWriter(w http.ResponseWriter) *retryBufferWriter {
	return &retryBufferWriter{
		real:   w,
		header: make(http.Header),
		status: http.StatusOK,
	}
}

// SetStreaming 标记此 buffer 为流式模式。
// 流式模式下，当收到成功响应（200）的第一次 Flush 时，自动 FlushToReal，
// 让数据立即透传给客户端，避免客户端"卡住"。
func (rb *retryBufferWriter) SetStreaming(streaming bool) {
	rb.mu.Lock()
	defer rb.mu.Unlock()
	rb.streaming = streaming
}

func (rb *retryBufferWriter) Header() http.Header {
	rb.mu.Lock()
	defer rb.mu.Unlock()
	if rb.flushed {
		return rb.real.Header()
	}
	return rb.header
}

func (rb *retryBufferWriter) WriteHeader(code int) {
	rb.mu.Lock()
	defer rb.mu.Unlock()
	if rb.flushed {
		rb.real.WriteHeader(code)
		return
	}
	rb.status = code
}

func (rb *retryBufferWriter) Write(b []byte) (int, error) {
	rb.mu.Lock()
	defer rb.mu.Unlock()
	if rb.flushed {
		return rb.real.Write(b)
	}
	return rb.body.Write(b)
}

// Flush implements http.Flusher - needed for SSE streaming.
// 在流式模式下，如果响应状态码为 200 且有缓冲数据，自动触发 FlushToReal，
// 将数据立即透传给客户端。这是解决"流式请求卡住"问题的关键：
// 没有这个机制，SSE 数据会被无限缓冲在内存中，客户端什么都收不到。
func (rb *retryBufferWriter) Flush() {
	rb.mu.Lock()
	defer rb.mu.Unlock()
	if rb.flushed {
		if f, ok := rb.real.(http.Flusher); ok {
			f.Flush()
		}
		return
	}
	// 流式模式：收到成功响应的数据后，自动透传给客户端
	if rb.streaming && rb.status == http.StatusOK && rb.body.Len() > 0 {
		rb.flushToRealLocked()
		return
	}
}

// FlushToReal 将缓冲的响应写入真正的 ResponseWriter，之后所有写入直接透传。
func (rb *retryBufferWriter) FlushToReal() {
	rb.mu.Lock()
	defer rb.mu.Unlock()
	rb.flushToRealLocked()
}

// flushToRealLocked 是 FlushToReal 的内部实现，调用方必须持有 rb.mu 锁。
func (rb *retryBufferWriter) flushToRealLocked() {
	if rb.flushed {
		return
	}
	rb.flushed = true

	// 复制 header
	for k, vs := range rb.header {
		for _, v := range vs {
			rb.real.Header().Add(k, v)
		}
	}
	rb.real.WriteHeader(rb.status)
	if rb.body.Len() > 0 {
		rb.real.Write(rb.body.Bytes())
	}
	// 立即 flush 确保客户端收到
	if f, ok := rb.real.(http.Flusher); ok {
		f.Flush()
	}
	rb.body.Reset()
}

// Reset 丢弃缓冲区，准备下一次重试。仅在未 flush 时有效。
func (rb *retryBufferWriter) Reset() {
	rb.mu.Lock()
	defer rb.mu.Unlock()
	if rb.flushed {
		return // 已经 flush 了，无法重置
	}
	rb.header = make(http.Header)
	rb.body.Reset()
	rb.status = http.StatusOK
}

// IsFlushed 返回是否已经 flush 到真正的 writer
func (rb *retryBufferWriter) IsFlushed() bool {
	rb.mu.Lock()
	defer rb.mu.Unlock()
	return rb.flushed
}
