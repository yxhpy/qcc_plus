package proxy

import (
	"bytes"
	"io"
)

// sseFixReader 修复上游返回的格式错误的 SSE 流。
// 确保每个 SSE 事件之间有正确的 \n\n 分隔。
// 某些逆向节点会在 JSON data 行未正确结束时就开始下一个 event: 行，
// 导致客户端 JSON 解析失败。
type sseFixReader struct {
	inner   io.ReadCloser
	pending bytes.Buffer // 待输出的修复后数据
	carry   []byte       // 上次 Read 未处理完的尾部（可能是不完整的行）
}

func newSSEFixReader(r io.ReadCloser) *sseFixReader {
	return &sseFixReader{inner: r}
}

func (s *sseFixReader) Read(p []byte) (int, error) {
	// 如果 pending 中有数据，先输出
	if s.pending.Len() > 0 {
		return s.pending.Read(p)
	}

	// 从上游读取
	buf := make([]byte, len(p)+4096) // 多读一些以便处理
	n, err := s.inner.Read(buf)
	if n == 0 {
		return 0, err
	}

	raw := buf[:n]

	// 将上次的 carry 拼接到前面
	if len(s.carry) > 0 {
		raw = append(s.carry, raw...)
		s.carry = nil
	}

	// 按行扫描，修复缺失的 \n\n 分隔
	s.fixSSEFraming(raw)

	// 从 pending 输出
	nn, _ := s.pending.Read(p)
	if nn == 0 && err != nil {
		return 0, err
	}
	// 如果上游返回了 err（如 io.EOF）但我们还有 pending 数据，先不返回 err
	if s.pending.Len() > 0 || nn > 0 {
		return nn, nil
	}
	return nn, err
}

// fixSSEFraming 扫描原始字节，确保 SSE 事件之间有 \n\n 分隔。
// SSE 事件行以 "event:", "data:", "id:", "retry:" 开头。
// 如果一个事件行紧跟在非空行后面（没有空行分隔），插入 \n。
func (s *sseFixReader) fixSSEFraming(raw []byte) {
	lines := bytes.Split(raw, []byte("\n"))

	// 最后一行可能不完整，保存为 carry
	if len(lines) > 0 && !bytes.HasSuffix(raw, []byte("\n")) {
		s.carry = make([]byte, len(lines[len(lines)-1]))
		copy(s.carry, lines[len(lines)-1])
		lines = lines[:len(lines)-1]
	}

	prevWasEmpty := false
	prevWasField := false

	for i, line := range lines {
		isField := isSSEFieldLine(line)
		isEmpty := len(bytes.TrimSpace(line)) == 0

		// 如果当前行是 SSE 字段行，且前一行不是空行，且前一行也是字段行
		// 且当前行是 "event:" 开头（表示新事件开始），需要插入空行分隔
		if isField && isNewEventStart(line) && !prevWasEmpty && prevWasField && i > 0 {
			s.pending.WriteByte('\n') // 插入空行作为事件分隔
		}

		s.pending.Write(line)
		s.pending.WriteByte('\n')

		prevWasEmpty = isEmpty
		prevWasField = isField
	}
}

// isSSEFieldLine 判断是否是 SSE 字段行
func isSSEFieldLine(line []byte) bool {
	trimmed := bytes.TrimSpace(line)
	return bytes.HasPrefix(trimmed, []byte("event:")) ||
		bytes.HasPrefix(trimmed, []byte("data:")) ||
		bytes.HasPrefix(trimmed, []byte("id:")) ||
		bytes.HasPrefix(trimmed, []byte("retry:"))
}

// isNewEventStart 判断是否是新 SSE 事件的开始（event: 行）
func isNewEventStart(line []byte) bool {
	return bytes.HasPrefix(bytes.TrimSpace(line), []byte("event:"))
}

func (s *sseFixReader) Close() error {
	return s.inner.Close()
}
