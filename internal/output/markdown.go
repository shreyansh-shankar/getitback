package output

import (
	"fmt"
	"io"

	"github.com/shreyansh-shankar/getitback/internal/module"
)

type MarkdownRenderer struct{}

func (r *MarkdownRenderer) RenderInventory(w io.Writer, results []*module.InventoryResult, _ RenderOptions) error {
	for _, res := range results {
		if !res.Detected {
			continue
		}

		fmt.Fprintf(w, "## %s\n\n", res.Module)

		if res.Version != "" {
			fmt.Fprintf(w, "**Version:** %s\n\n", res.Version)
		}

		for k, v := range res.Metadata {
			fmt.Fprintf(w, "- **%s:** %v\n", k, v)
		}

		for _, resource := range res.Resources {
			size := ""
			if resource.Size > 0 {
				size = " (" + formatSize(resource.Size) + ")"
			}
			fmt.Fprintf(w, "- %s%s\n", resource.Path, size)
		}

		if len(res.Errors) > 0 {
			fmt.Fprintln(w)
			for _, err := range res.Errors {
				fmt.Fprintf(w, "> **Error:** %s\n", err)
			}
		}

		fmt.Fprintln(w)
	}
	return nil
}
