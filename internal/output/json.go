package output

import (
	"encoding/json"
	"io"

	"github.com/shreyansh-shankar/getitback/internal/module"
)

type JSONRenderer struct{}

func (r *JSONRenderer) RenderInventory(w io.Writer, results []*module.InventoryResult, _ RenderOptions) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(results)
}
