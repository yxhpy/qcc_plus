package proxy

import (
	"bytes"
	"context"
	"io"
	"sync/atomic"
	"testing"
	"time"
)

// slowReader 模拟一个每次 Read 之间有延迟的 reader，用于测试空闲超时。
type slowReader struct {
	chunks [][]byte
	delays []time.Duration
	idx    int
}

func (s *slowReader) Read(p []byte) (int, error) {
	if s.idx >= len(s.chunks) {
		return 0, io.EOF
	}
	if s.idx < len(s.delays) {
		time.Sleep(s.delays[s.idx])
	}
	n := copy(p, s.chunks[s.idx])
	s.idx++
	return n, nil
}

func (s *slowReader) Close() error { return nil }

func TestIdleTimeoutReader_NormalRead(t *testing.T) {
	// 数据持续到达，不应触发超时
	data := []byte("event: message\ndata: {\"text\":\"hello\"}\n\n")
	inner := io.NopCloser(bytes.NewReader(data))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	itr := newIdleTimeoutReader(inner, 500*time.Millisecond, cancel)
	defer itr.Close()

	buf := make([]byte, 1024)
	n, err := itr.Read(buf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != len(data) {
		t.Fatalf("expected %d bytes, got %d", len(data), n)
	}
	if !bytes.Equal(buf[:n], data) {
		t.Fatalf("data mismatch")
	}

	// context 不应被取消
	select {
	case <-ctx.Done():
		t.Fatal("context should not be cancelled after normal read")
	default:
	}
}

func TestIdleTimeoutReader_IdleTimeout(t *testing.T) {
	// 模拟上游停止发送数据，空闲超时应触发
	inner := &slowReader{
		chunks: [][]byte{
			[]byte("event: message\ndata: chunk1\n\n"),
			[]byte("event: message\ndata: chunk2\n\n"), // 这个 chunk 延迟超过空闲超时
		},
		delays: []time.Duration{
			0,
			500 * time.Millisecond, // 超过 100ms 的空闲超时
		},
	}

	var cancelled atomic.Bool
	ctx, cancel := context.WithCancel(context.Background())
	wrappedCancel := func() {
		cancelled.Store(true)
		cancel()
	}

	itr := newIdleTimeoutReader(inner, 100*time.Millisecond, wrappedCancel)
	defer itr.Close()

	// 第一次读取正常
	buf := make([]byte, 1024)
	n, err := itr.Read(buf)
	if err != nil {
		t.Fatalf("first read error: %v", err)
	}
	if n == 0 {
		t.Fatal("first read returned 0 bytes")
	}

	// 等待空闲超时触发
	time.Sleep(200 * time.Millisecond)

	// context 应该已被取消
	select {
	case <-ctx.Done():
		// 预期行为
	default:
		t.Fatal("context should be cancelled after idle timeout")
	}

	if !cancelled.Load() {
		t.Fatal("cancel function should have been called")
	}
}

func TestIdleTimeoutReader_ContinuousData(t *testing.T) {
	// 数据持续到达（每次间隔小于空闲超时），不应触发超时
	inner := &slowReader{
		chunks: [][]byte{
			[]byte("chunk1"),
			[]byte("chunk2"),
			[]byte("chunk3"),
		},
		delays: []time.Duration{
			0,
			30 * time.Millisecond,
			30 * time.Millisecond,
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	itr := newIdleTimeoutReader(inner, 100*time.Millisecond, cancel)
	defer itr.Close()

	// 读取所有数据
	all, err := io.ReadAll(itr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(all) != "chunk1chunk2chunk3" {
		t.Fatalf("data mismatch: got %q", string(all))
	}

	// context 不应被取消
	select {
	case <-ctx.Done():
		t.Fatal("context should not be cancelled when data flows continuously")
	default:
	}
}

func TestIdleTimeoutReader_CloseStopsTimer(t *testing.T) {
	// Close 后不应触发超时回调
	inner := io.NopCloser(bytes.NewReader([]byte("data")))

	var cancelled atomic.Bool
	_, cancel := context.WithCancel(context.Background())
	wrappedCancel := func() {
		cancelled.Store(true)
		cancel()
	}

	itr := newIdleTimeoutReader(inner, 50*time.Millisecond, wrappedCancel)

	// 立即关闭
	if err := itr.Close(); err != nil {
		t.Fatalf("close error: %v", err)
	}

	// 等待超过空闲超时
	time.Sleep(100 * time.Millisecond)

	if cancelled.Load() {
		t.Fatal("cancel should not be called after Close")
	}
}

func TestIdleTimeoutReader_ResetOnEachRead(t *testing.T) {
	// 每次 Read 都重置计时器，即使每次间隔接近超时也不应触发
	inner := &slowReader{
		chunks: [][]byte{
			[]byte("a"),
			[]byte("b"),
			[]byte("c"),
			[]byte("d"),
			[]byte("e"),
		},
		delays: []time.Duration{
			0,
			80 * time.Millisecond, // 接近 100ms 超时但未超过
			80 * time.Millisecond,
			80 * time.Millisecond,
			80 * time.Millisecond,
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	itr := newIdleTimeoutReader(inner, 100*time.Millisecond, cancel)
	defer itr.Close()

	all, err := io.ReadAll(itr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(all) != "abcde" {
		t.Fatalf("data mismatch: got %q", string(all))
	}

	select {
	case <-ctx.Done():
		t.Fatal("context should not be cancelled when each read resets the timer")
	default:
	}
}
