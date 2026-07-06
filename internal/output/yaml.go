package output

import (
	"io"

	"github.com/shreyansh-shankar/getitback/internal/doctor"
	"github.com/shreyansh-shankar/getitback/internal/module"
	"github.com/shreyansh-shankar/getitback/internal/report"
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

func (r *YAMLRenderer) RenderDoctor(w io.Writer, report *doctor.Report) error {
	encoder := yaml.NewEncoder(w)
	encoder.SetIndent(2)
	if err := encoder.Encode(report); err != nil {
		return err
	}
	return encoder.Close()
}

func (r *YAMLRenderer) RenderReport(w io.Writer, rep *report.Report) error {
	encoder := yaml.NewEncoder(w)
	encoder.SetIndent(2)
	if err := encoder.Encode(rep); err != nil {
		return err
	}
	return encoder.Close()
}
