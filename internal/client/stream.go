package client

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
)

func stream(r io.Reader) error {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	var event string
	var data []string

	flush := func() {
		if len(data) == 0 {
			return
		}
		payload := strings.Join(data, "\n")
		if event == "content_block_delta" {
			var obj map[string]any
			if err := json.Unmarshal([]byte(payload), &obj); err == nil {
				if delta, ok := obj["delta"].(map[string]any); ok {
					if t, ok := delta["text"].(string); ok {
						fmt.Print(t)
					}
				}
			}
		} else if event == "message_delta" {
			fmt.Fprintf(os.Stderr, "\n[%s]\n", payload)
		}
		event = ""
		data = data[:0]
	}

	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "event:") {
			event = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
		} else if strings.HasPrefix(line, "data:") {
			data = append(data, strings.TrimSpace(strings.TrimPrefix(line, "data:")))
		} else if line == "" {
			flush()
		}
	}
	flush()
	return scanner.Err()
}
