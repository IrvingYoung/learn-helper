package handler

import (
	"encoding/json"
	"fmt"
	"strings"
)

// parseWriteInput validates and decodes a write tool's input.
// Returns a map with the required fields populated.
// Returns an error if required fields are missing.
// Returns an error if any field value contains a {{action:X}} placeholder
// (same-batch chaining is not allowed — split across turns).
func parseWriteInput(tool string, raw json.RawMessage) (map[string]any, error) {
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, fmt.Errorf("parse %s input: %w", tool, err)
	}
	for k, v := range m {
		if s, ok := v.(string); ok && strings.Contains(s, "{{action:") {
			return nil, fmt.Errorf("%s: field %q references a placeholder from a pending tool call in the same batch; split the call into a later turn", tool, k)
		}
	}
	switch tool {
	case "create_page":
		if _, ok := m["title"].(string); !ok {
			return nil, fmt.Errorf("create_page: title (string) is required")
		}
	case "update_page":
		if _, ok := numberField(m, "page_id"); !ok {
			return nil, fmt.Errorf("update_page: page_id (integer) is required")
		}
		if _, ok := m["content"].(string); !ok {
			return nil, fmt.Errorf("update_page: content (string) is required")
		}
	case "patch_page":
		if _, ok := numberField(m, "page_id"); !ok {
			return nil, fmt.Errorf("patch_page: page_id (integer) is required")
		}
		if _, ok := m["operations"].([]any); !ok {
			return nil, fmt.Errorf("patch_page: operations (array) is required")
		}
	case "delete_page", "move_page":
		if _, ok := numberField(m, "page_id"); !ok {
			return nil, fmt.Errorf("%s: page_id (integer) is required", tool)
		}
		if tool == "move_page" {
			if _, ok := numberField(m, "new_parent_id"); !ok {
				return nil, fmt.Errorf("move_page: new_parent_id (integer) is required")
			}
		}
	case "link_pages":
		if _, ok := numberField(m, "source_page_id"); !ok {
			return nil, fmt.Errorf("link_pages: source_page_id (integer) is required")
		}
		if _, ok := numberField(m, "target_page_id"); !ok {
			return nil, fmt.Errorf("link_pages: target_page_id (integer) is required")
		}
	default:
		return nil, fmt.Errorf("unknown write tool: %s", tool)
	}
	return m, nil
}

func numberField(m map[string]any, key string) (float64, bool) {
	v, ok := m[key]
	if !ok {
		return 0, false
	}
	f, ok := v.(float64)
	return f, ok
}

// previewWrite produces a human-readable summary of a write op for the permission queue UI.
func previewWrite(tool string, in map[string]any) string {
	id := intField(in, "page_id")
	switch tool {
	case "create_page":
		title, _ := in["title"].(string)
		pid := intField(in, "parent_id")
		if pid != 0 {
			return fmt.Sprintf("在父页 %d 下创建页面「%s」", pid, title)
		}
		return fmt.Sprintf("创建顶级页面「%s」", title)
	case "update_page":
		return fmt.Sprintf("更新页面 %d", id)
	case "patch_page":
		return fmt.Sprintf("增量编辑页面 %d", id)
	case "delete_page":
		return fmt.Sprintf("删除页面 %d", id)
	case "link_pages":
		s := intField(in, "source_page_id")
		t := intField(in, "target_page_id")
		return fmt.Sprintf("在页面 %d 添加指向 %d 的链接", s, t)
	case "move_page":
		np := intField(in, "new_parent_id")
		return fmt.Sprintf("把页面 %d 移到父页 %d 下", id, np)
	default:
		return fmt.Sprintf("%s: %v", tool, in)
	}
}

func intField(m map[string]any, key string) int64 {
	v, ok := m[key]
	if !ok {
		return 0
	}
	f, ok := v.(float64)
	if !ok {
		return 0
	}
	return int64(f)
}
