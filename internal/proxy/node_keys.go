package proxy

import (
	"encoding/json"
	"fmt"
	"strings"
)

// NamedAPIKey 表示一个可命名的节点密钥。
type NamedAPIKey struct {
	Name string `json:"name"`
	Key  string `json:"key"`
}

func NewKeyRotatorFromNamedKeys(keys []NamedAPIKey, cfg KeyRotatorConfig) *KeyRotator {
	normalized := normalizeNamedAPIKeys(keys)
	if len(normalized) == 0 {
		return nil
	}
	return NewKeyRotator(joinNamedAPIKeys(normalized), cfg)
}

func cloneNamedAPIKeys(keys []NamedAPIKey) []NamedAPIKey {
	if len(keys) == 0 {
		return nil
	}
	out := make([]NamedAPIKey, len(keys))
	copy(out, keys)
	return out
}

func decodeNamedAPIKeys(configJSON, legacy string) []NamedAPIKey {
	configJSON = strings.TrimSpace(configJSON)
	if configJSON != "" {
		var direct []NamedAPIKey
		if err := json.Unmarshal([]byte(configJSON), &direct); err == nil {
			return normalizeNamedAPIKeys(direct)
		}
		var wrapped struct {
			Keys []NamedAPIKey `json:"keys"`
		}
		if err := json.Unmarshal([]byte(configJSON), &wrapped); err == nil {
			return normalizeNamedAPIKeys(wrapped.Keys)
		}
	}
	return normalizeNamedAPIKeys(parseLegacyAPIKeys(legacy))
}

func encodeNamedAPIKeys(keys []NamedAPIKey) string {
	keys = normalizeNamedAPIKeys(keys)
	if len(keys) == 0 {
		return ""
	}
	if len(keys) == 1 && strings.TrimSpace(keys[0].Name) == "" {
		return ""
	}
	body, err := json.Marshal(keys)
	if err != nil {
		return ""
	}
	return string(body)
}

func joinNamedAPIKeys(keys []NamedAPIKey) string {
	if len(keys) == 0 {
		return ""
	}
	parts := make([]string, 0, len(keys))
	for _, item := range normalizeNamedAPIKeys(keys) {
		if item.Key != "" {
			parts = append(parts, item.Key)
		}
	}
	return strings.Join(parts, ",")
}

func normalizeNamedAPIKeys(keys []NamedAPIKey) []NamedAPIKey {
	if len(keys) == 0 {
		return nil
	}
	result := make([]NamedAPIKey, 0, len(keys))
	usedNames := make(map[string]int)
	for idx, item := range keys {
		key := strings.TrimSpace(item.Key)
		if key == "" {
			continue
		}
		name := strings.TrimSpace(item.Name)
		if len(keys) > 1 && name == "" {
			name = fmt.Sprintf("key%d", idx+1)
		}
		if name != "" {
			base := name
			lower := strings.ToLower(base)
			if usedNames[lower] > 0 {
				for suffix := usedNames[lower] + 1; ; suffix++ {
					candidate := fmt.Sprintf("%s-%d", base, suffix)
					if usedNames[strings.ToLower(candidate)] == 0 {
						name = candidate
						usedNames[lower] = suffix
						break
					}
				}
			}
			usedNames[strings.ToLower(name)]++
		}
		result = append(result, NamedAPIKey{Name: name, Key: key})
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

func parseLegacyAPIKeys(raw string) []NamedAPIKey {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	result := make([]NamedAPIKey, 0, len(parts))
	for _, part := range parts {
		key := strings.TrimSpace(part)
		if key == "" {
			continue
		}
		result = append(result, NamedAPIKey{Key: key})
	}
	return result
}

func buildNodeKeyState(rawAPIKey, rawConfig string) (string, []NamedAPIKey, *KeyRotator) {
	keys := decodeNamedAPIKeys(rawConfig, rawAPIKey)
	joined := joinNamedAPIKeys(keys)
	if joined == "" {
		joined = strings.TrimSpace(rawAPIKey)
	}
	if len(keys) == 0 {
		return joined, nil, nil
	}
	return joined, keys, NewKeyRotatorFromNamedKeys(keys, loadKeyRotatorConfig())
}

func applyNodeKeyState(node *Node, rawAPIKey, rawConfig string) {
	if node == nil {
		return
	}
	joined, keys, rotator := buildNodeKeyState(rawAPIKey, rawConfig)
	node.APIKey = joined
	node.APIKeyConfig = encodeNamedAPIKeys(keys)
	node.APIKeyItems = cloneNamedAPIKeys(keys)
	node.APIKeys = rotator
}

func displayNodeNameForKey(nodeName, keyName string) string {
	nodeName = strings.TrimSpace(nodeName)
	keyName = strings.TrimSpace(keyName)
	if nodeName == "" {
		return keyName
	}
	if keyName == "" {
		return nodeName
	}
	return nodeName + "-" + keyName
}

func maskAPIKey(key string) string {
	key = strings.TrimSpace(key)
	if key == "" {
		return ""
	}
	if len(key) <= 8 {
		return "********"
	}
	return key[:4] + "****" + key[len(key)-4:]
}
