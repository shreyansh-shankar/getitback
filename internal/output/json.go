package output

import (
	"encoding/json"
	"io"

	"github.com/shreyansh-shankar/getitback/internal/doctor"
	"github.com/shreyansh-shankar/getitback/internal/module"
	"github.com/shreyansh-shankar/getitback/internal/report"
)

type JSONRenderer struct{}

func (r *JSONRenderer) RenderInventory(w io.Writer, results []*module.InventoryResult, _ RenderOptions) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(results)
}

func (r *JSONRenderer) RenderDoctor(w io.Writer, report *doctor.Report) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(report)
}

func (r *JSONRenderer) RenderReport(w io.Writer, rep *report.Report) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(rep)
}
