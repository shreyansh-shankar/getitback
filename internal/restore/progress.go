package restore

import (
	"fmt"
	"io"

	"github.com/shreyansh-shankar/getitback/internal/output"
)

type ProgressReporter struct {
	w       io.Writer
	total   int
	current int
}

func NewProgressReporter(w io.Writer, total int) *ProgressReporter {
	return &ProgressReporter{w: w, total: total}
}

func (p *ProgressReporter) Write(b []byte) (int, error) {
	return p.w.Write(b)
}

func (p *ProgressReporter) Stage(stage, title string) {
	fmt.Fprintf(p.w, "\n  %s%s%s\n",
		output.ColorBold+output.ColorCyan,
		"━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━",
		output.ColorReset)
	fmt.Fprintf(p.w, "  %sStage %s · %s%s\n",
		output.ColorBold+output.ColorCyan, stage, title, output.ColorReset)
	fmt.Fprintf(p.w, "  %s%s%s\n",
		output.ColorBold+output.ColorCyan,
		"━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━",
		output.ColorReset)
}

func (p *ProgressReporter) ModuleSuccess(phase, name string, detail string) {
	d := ""
	if detail != "" {
		d = " " + detail
	}
	fmt.Fprintf(p.w, "  %s✓%s %s%s\n",
		output.ColorGreen, output.ColorReset, name, d)
}

func (p *ProgressReporter) ModuleSkip(phase, name, reason string) {
	fmt.Fprintf(p.w, "  %s○%s %s  %s(%s)%s\n",
		output.ColorCyan, output.ColorReset, name,
		output.ColorCyan, reason, output.ColorReset)
}

func (p *ProgressReporter) ModuleFailure(phase, name string, err error) {
	fmt.Fprintf(p.w, "  %s✗%s %s  %s%v%s\n",
		output.ColorRed, output.ColorReset, name,
		output.ColorRed, err, output.ColorReset)
}

func (p *ProgressReporter) ModuleWarning(phase, name, warning string) {
	fmt.Fprintf(p.w, "  %s⚠%s %s  %s%s%s\n",
		output.ColorYellow, output.ColorReset, name,
		output.ColorYellow, warning, output.ColorReset)
}

func (p *ProgressReporter) DetailLine(format string, args ...any) {
	fmt.Fprintf(p.w, "    %s·%s %s\n",
		output.ColorCyan, output.ColorReset,
		fmt.Sprintf(format, args...))
}

func (p *ProgressReporter) InfoLine(format string, args ...any) {
	fmt.Fprintf(p.w, "  %s%s%s\n",
		output.ColorCyan, fmt.Sprintf(format, args...), output.ColorReset)
}

func (p *ProgressReporter) SummaryLine(label, value string) {
	fmt.Fprintf(p.w, "  %s%s%s\n",
		output.ColorCyan,
		output.DotLeader(label, value, 22),
		output.ColorReset)
}
