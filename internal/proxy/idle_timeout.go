package proxy

import (
	"context"
	"io"
	"sync"
	"time"
)

// idleTimeoutReader 包装 io.ReadCloser，当连续 idleTimeout 时间没有读到任何数据时，
// 取消关联的 context，使上层感知到超时。
// 适用于 SSE 流式响应：只要上游持续发送数据就不会超时，
// 但如果上游停止发送（卡住），超过空闲时间后自动中断。
type idleTimeoutReader struct {
	inner       io.ReadCloser
	idleTimeout time.Duration
	cancel      context.CancelFunc
	timer       *time.Timer
	mu          sync.Mutex
	closed      bool
}

// newIdleTimeoutReader 创建一个带空闲超时的 Reader。
// cancel 函数会在空闲超时触发时被调用，用于取消请求 context。
func newIdleTimeoutReader(r io.ReadCloser, idleTimeout time.Duration, cancel context.CancelFunc) *idleTimeoutReader {
	itr := &idleTimeoutReader{
		inner:       r,
		idleTimeout: idleTimeout,
		cancel:      cancel,
	}
	itr.timer = time.AfterFunc(idleTimeout, itr.onTimeout)
	return itr
}

// onTimeout 空闲超时回调，取消 context。
func (itr *idleTimeoutReader) onTimeout() {
	itr.mu.Lock()
	defer itr.mu.Unlock()
	if !itr.closed && itr.cancel != nil {
		itr.cancel()
	}
}

// Read 每次成功读到数据后重置空闲计时器。
func (itr *idleTimeoutReader) Read(p []byte) (int, error) {
	n, err := itr.inner.Read(p)
	if n > 0 {
		itr.mu.Lock()
		if !itr.closed {
			itr.timer.Reset(itr.idleTimeout)
		}
		itr.mu.Unlock()
	}
	return n, err
}

// Close 关闭底层 reader 并停止计时器。
func (itr *idleTimeoutReader) Close() error {
	itr.mu.Lock()
	itr.closed = true
	itr.timer.Stop()
	itr.mu.Unlock()
	return itr.inner.Close()
}
