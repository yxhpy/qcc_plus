package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"strings"
)

// 模拟 cleanTools 函数
func cleanTools(body []byte) ([]byte, bool) {
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, false
	}
	rawTools, ok := payload["tools"].([]any)
	if !ok || len(rawTools) == 0 {
		return nil, false
	}

	changed := false
	sanitized := make([]any, 0, len(rawTools))
	for _, item := range rawTools {
		obj, ok := item.(map[string]any)
		if !ok {
			sanitized = append(sanitized, item)
			continue
		}
		cleaned := make(map[string]any, 3)
		if v, ok := obj["name"]; ok {
			cleaned["name"] = v
		}
		if v, ok := obj["description"]; ok {
			cleaned["description"] = v
		}
		if v, ok := obj["input_schema"]; ok {
			cleaned["input_schema"] = v
		}
		if len(cleaned) != len(obj) {
			changed = true
		}
		sanitized = append(sanitized, cleaned)
	}
	if !changed {
		return nil, false
	}
	payload["tools"] = sanitized
	buf, err := json.Marshal(payload)
	if err != nil {
		return nil, false
	}
	return buf, true
}

func main() {
	fmt.Println("=== 工具清理功能验证 ===")

	// 测试用例 1: 包含 custom 字段的工具
	testCase1 := map[string]any{
		"model": "claude-sonnet-4-5-20250929",
		"tools": []any{
			map[string]any{
				"name":        "test_tool",
				"description": "test description",
				"input_schema": map[string]any{
					"type": "object",
				},
				"custom": map[string]any{
					"input_examples": []string{"example1", "example2"},
				},
			},
		},
		"messages": []any{
			map[string]any{
				"role":    "user",
				"content": "test",
			},
		},
	}

	body1, _ := json.Marshal(testCase1)
	fmt.Println("测试用例 1: 包含 custom.input_examples 字段")
	fmt.Printf("原始请求体:\n%s\n\n", string(body1))

	cleaned1, changed1 := cleanTools(body1)
	if changed1 {
		fmt.Printf("✓ 成功清理\n清理后:\n%s\n\n", string(cleaned1))

		// 验证 custom 字段已被移除
		var result1 map[string]any
		json.Unmarshal(cleaned1, &result1)
		tools1 := result1["tools"].([]any)
		tool1 := tools1[0].(map[string]any)
		if _, hasCustom := tool1["custom"]; hasCustom {
			fmt.Println("✗ 错误: custom 字段仍然存在")
		} else {
			fmt.Println("✓ 验证通过: custom 字段已被移除")
		}
	} else {
		fmt.Println("✗ 清理失败")
	}

	fmt.Println("\n---")

	// 测试用例 2: 不包含额外字段的标准工具
	testCase2 := map[string]any{
		"model": "claude-sonnet-4-5-20250929",
		"tools": []any{
			map[string]any{
				"name":        "standard_tool",
				"description": "standard description",
				"input_schema": map[string]any{
					"type": "object",
				},
			},
		},
	}

	body2, _ := json.Marshal(testCase2)
	fmt.Println("\n测试用例 2: 标准工具定义(无额外字段)")
	fmt.Printf("原始请求体:\n%s\n\n", string(body2))

	_, changed2 := cleanTools(body2)
	if !changed2 {
		fmt.Println("✓ 正确: 无需清理，原样保留")
	} else {
		fmt.Println("✗ 错误: 不应该修改标准格式")
	}

	fmt.Println("\n---")

	// 测试用例 3: 多个工具，部分包含额外字段
	testCase3 := map[string]any{
		"model": "claude-sonnet-4-5-20250929",
		"tools": []any{
			map[string]any{
				"name":        "tool1",
				"description": "tool 1",
				"input_schema": map[string]any{
					"type": "object",
				},
			},
			map[string]any{
				"name":        "tool2",
				"description": "tool 2",
				"input_schema": map[string]any{
					"type": "object",
				},
				"custom": map[string]any{
					"input_examples": []string{"example"},
				},
				"extra_field": "should be removed",
			},
			map[string]any{
				"name":        "tool3",
				"description": "tool 3",
				"input_schema": map[string]any{
					"type": "object",
				},
			},
		},
	}

	body3, _ := json.Marshal(testCase3)
	fmt.Println("\n测试用例 3: 多个工具，第2个包含额外字段")
	fmt.Printf("原始请求体:\n%s\n\n", string(body3))

	cleaned3, changed3 := cleanTools(body3)
	if changed3 {
		fmt.Printf("✓ 成功清理\n清理后:\n%s\n\n", string(cleaned3))

		var result3 map[string]any
		json.Unmarshal(cleaned3, &result3)
		tools3 := result3["tools"].([]any)
		tool3_2 := tools3[1].(map[string]any)
		if _, hasCustom := tool3_2["custom"]; hasCustom {
			fmt.Println("✗ 错误: custom 字段仍然存在")
		} else if _, hasExtra := tool3_2["extra_field"]; hasExtra {
			fmt.Println("✗ 错误: extra_field 字段仍然存在")
		} else {
			fmt.Println("✓ 验证通过: 额外字段已被移除")
		}
	} else {
		fmt.Println("✗ 清理失败")
	}

	fmt.Println("\n---")

	// 测试用例 4: 集成测试 - 模拟 HTTP 代理
	fmt.Println("\n测试用例 4: HTTP 代理集成测试")

	// 创建模拟上游服务器
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var payload map[string]any
		json.Unmarshal(body, &payload)

		if tools, ok := payload["tools"].([]any); ok && len(tools) > 0 {
			tool := tools[0].(map[string]any)
			if _, hasCustom := tool["custom"]; hasCustom {
				// 模拟 Anthropic API 拒绝包含 custom 字段的请求
				w.WriteHeader(400)
				json.NewEncoder(w).Encode(map[string]any{
					"error": map[string]any{
						"type":    "invalid_request_error",
						"message": "tools.0.custom.input_examples: Extra inputs are not permitted",
					},
				})
				return
			}
		}

		w.WriteHeader(200)
		json.NewEncoder(w).Encode(map[string]any{
			"id":      "msg_test",
			"type":    "message",
			"content": []any{},
		})
	}))
	defer upstream.Close()

	// 创建带清理功能的代理
	upstreamURL := upstream.URL
	proxy := httputil.NewSingleHostReverseProxy(mustParseURL(upstreamURL))
	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req)
		req.URL.Scheme = "http"
		req.URL.Host = strings.TrimPrefix(upstreamURL, "http://")

		if req.Body != nil && req.Method == http.MethodPost {
			ct := strings.ToLower(req.Header.Get("Content-Type"))
			if strings.Contains(ct, "application/json") {
				bodyBytes, _ := io.ReadAll(req.Body)
				req.Body.Close()

				if cleaned, ok := cleanTools(bodyBytes); ok {
					bodyBytes = cleaned
				}

				req.Body = io.NopCloser(bytes.NewReader(bodyBytes))
				req.ContentLength = int64(len(bodyBytes))
			}
		}
	}

	proxyServer := httptest.NewServer(proxy)
	defer proxyServer.Close()

	// 测试发送包含 custom 字段的请求
	requestBody := map[string]any{
		"model": "claude-sonnet-4-5-20250929",
		"tools": []any{
			map[string]any{
				"name":        "test",
				"description": "test",
				"input_schema": map[string]any{
					"type": "object",
				},
				"custom": map[string]any{
					"input_examples": []string{"example"},
				},
			},
		},
		"messages": []any{
			map[string]any{
				"role":    "user",
				"content": "test",
			},
		},
	}

	reqBody, _ := json.Marshal(requestBody)
	resp, err := http.Post(proxyServer.URL, "application/json", bytes.NewReader(reqBody))
	if err != nil {
		fmt.Printf("✗ 请求失败: %v\n", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == 200 {
		fmt.Println("✓ 成功: 代理正确清理了请求，上游接受了请求")
	} else {
		body, _ := io.ReadAll(resp.Body)
		fmt.Printf("✗ 失败: 状态码 %d\n响应: %s\n", resp.StatusCode, string(body))
	}

	fmt.Println("\n=== 所有测试完成 ===")
}

func mustParseURL(rawURL string) *url.URL {
	u, err := url.Parse(rawURL)
	if err != nil {
		panic(err)
	}
	return u
}
