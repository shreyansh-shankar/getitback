package output_test

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/shreyansh-shankar/getitback/internal/module"
	"github.com/shreyansh-shankar/getitback/internal/output"
)

func TestParseFormat(t *testing.T) {
	tests := []struct {
		input string
		want  output.Format
	}{
		{"terminal", output.FormatTerminal},
		{"json", output.FormatJSON},
		{"yaml", output.FormatYAML},
		{"yml", output.FormatYAML},
		{"markdown", output.FormatMarkdown},
		{"md", output.FormatMarkdown},
		{"unknown", output.FormatTerminal},
		{"", output.FormatTerminal},
	}

	for _, tc := range tests {
		got := output.ParseFormat(tc.input)
		if got != tc.want {
			t.Errorf("ParseFormat(%q) = %v, want %v", tc.input, got, tc.want)
		}
	}
}

func TestTerminalRenderer_Inventory(t *testing.T) {
	renderer := output.NewRenderer(output.FormatTerminal)
	var buf bytes.Buffer

	results := []*module.InventoryResult{
		{
			Module:   "test",
			Detected: true,
			Version:  "v1.0",
			Metadata: map[string]any{"key": "value"},
			Resources: []module.Resource{
				{Name: "file.txt", Path: "/tmp/file.txt", Size: 1024, Modified: time.Now(), Type: "config"},
			},
		},
	}

	opts := output.RenderOptions{Verbose: true, Categories: map[string]string{"test": "Test"}}
	err := renderer.RenderInventory(&buf, results, opts)
	if err != nil {
		t.Fatalf("RenderInventory() error = %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "test") {
		t.Error("expected output to contain module name")
	}
	if !strings.Contains(output, "v1.0") {
		t.Error("expected output to contain version")
	}
}

func TestTerminalRenderer_InventoryEmpty(t *testing.T) {
	renderer := output.NewRenderer(output.FormatTerminal)
	var buf bytes.Buffer

	err := renderer.RenderInventory(&buf, nil, output.RenderOptions{})
	if err != nil {
		t.Fatalf("RenderInventory() error = %v", err)
	}
}

func TestTerminalRenderer_SkipsUndetected(t *testing.T) {
	renderer := output.NewRenderer(output.FormatTerminal)
	var buf bytes.Buffer

	results := []*module.InventoryResult{
		{Module: "undetected", Detected: false},
	}

	err := renderer.RenderInventory(&buf, results, output.RenderOptions{})
	if err != nil {
		t.Fatalf("RenderInventory() error = %v", err)
	}
	// Should still show health summary even if only skipped modules
	if buf.Len() == 0 {
		t.Error("expected output with health summary even for skipped modules")
	}
	if !strings.Contains(buf.String(), "Healthy:") {
		t.Error("expected health summary in output")
	}
}
