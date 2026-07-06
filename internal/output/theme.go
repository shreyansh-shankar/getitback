package output

import (
	"fmt"
	"strings"
)

type Icon string

const (
	IconCheck  Icon = "✓"
	IconCross  Icon = "✗"
	IconWarn   Icon = "⚠"
	IconInfo   Icon = "ℹ"
	IconBullet Icon = "•"
	IconArrow  Icon = "▸"
	IconBranch Icon = "├"
	IconLeaf   Icon = "└"
)

func SectionHeader(title string) string {
	line := strings.Repeat("━", 50)
	return fmt.Sprintf("\n%s%s%s\n  %s%s%s\n%s%s%s\n",
		ColorBold+ColorCyan, line, ColorReset,
		ColorBold+ColorCyan, title, ColorReset,
		ColorBold+ColorCyan, line, ColorReset)
}

func StatusLine(icon Icon, color, label, detail string) string {
	return fmt.Sprintf(" %s %s%s%s%s%s",
		color, icon, ColorReset,
		ColorBold, label, ColorReset) +
		detail
}

func KeyValue(key string, value any) string {
	return fmt.Sprintf("  %s%s:%s %v", ColorBlue, key, ColorReset, value)
}

func KeyValuePadded(key string, value any, width int) string {
	padding := width - len(key)
	if padding < 1 {
		padding = 1
	}
	return fmt.Sprintf("  %s%s%s%v",
		ColorBlue, key, ColorReset,
		value)
}

func TreeItem(prefix string, name, value string, isLast bool) string {
	branch := IconBranch
	if isLast {
		branch = IconLeaf
	}
	if value != "" {
		return fmt.Sprintf("    %s%s %s%s %s", ColorCyan, branch, ColorReset, name, value)
	}
	return fmt.Sprintf("    %s%s %s%s", ColorCyan, branch, ColorReset, name)
}

func Badge(text string, color string) string {
	return fmt.Sprintf("%s%s%s", color, text, ColorReset)
}

func SummaryLine(label string, value string) string {
	return fmt.Sprintf("  %s%s:%s %s\n", ColorBold+ColorBlue, label, ColorReset, value)
}

func Separator() string {
	return fmt.Sprintf("  %s──────────────────────────────────────────────────%s\n", ColorCyan, ColorReset)
}
