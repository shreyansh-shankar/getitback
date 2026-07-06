package output

import (
	"io"

	"github.com/shreyansh-shankar/getitback/internal/module"
	"gopkg.in/yaml.v3"
)

type YAMLRenderer struct{}

func (r *YAMLRenderer) RenderInventory(w io.Writer, results []*module.InventoryResult, _ RenderOptions) error {
	encoder := yaml.NewEncoder(w)
	encoder.SetIndent(2)
	if err := encoder.Encode(results); err != nil {
		return err
	}
	return encoder.Close()
}
