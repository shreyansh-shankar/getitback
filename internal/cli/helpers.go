package cli

import (
	"fmt"
	"strings"

	"github.com/shreyansh-shankar/getitback/internal/module"
	"github.com/shreyansh-shankar/getitback/internal/output"
)

func formatMetaValue(v any) string {
	switch val := v.(type) {
	case []string:
		if len(val) > 10 {
			return fmt.Sprintf("%d items", len(val))
		}
		return strings.Join(val, ", ")
	case []any:
		if len(val) > 10 {
			return fmt.Sprintf("%d items", len(val))
		}
		parts := make([]string, len(val))
		for i, item := range val {
			if m, ok := item.(map[string]any); ok {
				if name, has := m["filename"]; has {
					parts[i] = fmt.Sprintf("%v", name)
					continue
				}
			}
			parts[i] = fmt.Sprint(item)
		}
		return strings.Join(parts, ", ")
	case map[string]string:
		if len(val) <= 5 {
			parts := make([]string, 0, len(val))
			for k, v := range val {
				parts = append(parts, k+":"+v)
			}
			return strings.Join(parts, ", ")
		}
		return fmt.Sprintf("%d entries", len(val))
	case map[string]int:
		if len(val) <= 5 {
			parts := make([]string, 0, len(val))
			for k, v := range val {
				parts = append(parts, fmt.Sprintf("%s:%d", k, v))
			}
			return strings.Join(parts, ", ")
		}
		return fmt.Sprintf("%d channels", len(val))
	case map[string]any:
		if len(val) <= 5 {
			parts := make([]string, 0, len(val))
			for k, v := range val {
				parts = append(parts, fmt.Sprintf("%s:%v", k, v))
			}
			return strings.Join(parts, ", ")
		}
		return fmt.Sprintf("%d entries", len(val))
	case bool:
		if val {
			return "yes"
		}
		return "no"
	default:
		return fmt.Sprint(v)
	}
}

func priorityColor(p module.RecommendationPriority) string {
	switch p {
	case module.RecPriorityCritical:
		return output.ColorRed
	case module.RecPriorityHigh:
		return output.ColorYellow
	case module.RecPriorityMedium:
		return output.ColorGreen
	default:
		return output.ColorCyan
	}
}
