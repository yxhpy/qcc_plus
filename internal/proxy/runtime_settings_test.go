package proxy

import "testing"

func TestRuntimeSettingDefinitions(t *testing.T) {
	defs := RuntimeSettingDefinitions()
	if len(defs) == 0 {
		t.Fatal("expected runtime settings definitions")
	}

	seen := make(map[string]struct{}, len(defs))
	for _, def := range defs {
		if def.Key == "" {
			t.Fatal("runtime setting key should not be empty")
		}
		if _, ok := seen[def.Key]; ok {
			t.Fatalf("duplicate runtime setting key: %s", def.Key)
		}
		seen[def.Key] = struct{}{}
		if _, ok := LookupRuntimeSettingDefinition(def.Key); !ok {
			t.Fatalf("lookup failed for key %s", def.Key)
		}
	}
}
