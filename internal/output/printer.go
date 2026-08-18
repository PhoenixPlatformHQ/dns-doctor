package output

import (
	"fmt"
	"io"
	"os"
	"strings"
)

// Printer writes human-readable check results to a writer.
type Printer struct {
	w io.Writer
}

// NewPrinter returns a Printer that writes to stdout.
func NewPrinter() *Printer {
	return &Printer{w: os.Stdout}
}

// NewPrinterTo returns a Printer that writes to the given writer.
func NewPrinterTo(w io.Writer) *Printer {
	return &Printer{w: w}
}

// Print formats and writes a single CheckResult.
func (p *Printer) Print(r CheckResult) {
	fmt.Fprintf(p.w, "[%s] %s\n", r.Status, r.Name)
	if r.Fact != "" {
		fmt.Fprintf(p.w, "  Fact:      %s\n", r.Fact)
	}
	if r.Evidence != "" {
		fmt.Fprintf(p.w, "  Evidence:  %s\n", r.Evidence)
	}
	if r.FaultDomain != "" {
		fmt.Fprintf(p.w, "  Likely fault domain: %s\n", r.FaultDomain)
	}
	if r.NextCheck != "" {
		fmt.Fprintf(p.w, "  Next check: %s\n", r.NextCheck)
	}
	if r.Error != "" {
		fmt.Fprintf(p.w, "  Error: %s\n", r.Error)
	}
}

// PrintAll writes a header and all results, then a summary line.
func (p *Printer) PrintAll(results []CheckResult) {
	sep := strings.Repeat("─", 60)
	fmt.Fprintln(p.w, sep)
	fmt.Fprintln(p.w, "  kubectl dns-doctor")
	fmt.Fprintln(p.w, sep)
	for _, r := range results {
		p.Print(r)
		fmt.Fprintln(p.w)
	}
}

// PrintSummaryLine writes a one-line tally of statuses.
func (p *Printer) PrintSummaryLine(results []CheckResult) {
	pass, warn, fail, info := tally(results)
	fmt.Fprintf(p.w, "Summary: %d PASS  %d WARN  %d FAIL  %d INFO\n", pass, warn, fail, info)
}

func tally(results []CheckResult) (pass, warn, fail, info int) {
	for _, r := range results {
		switch r.Status {
		case StatusPass:
			pass++
		case StatusWarn:
			warn++
		case StatusFail:
			fail++
		case StatusInfo:
			info++
		}
	}
	return
}
