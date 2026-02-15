package output

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"
)

// ANSI color codes
const (
	Reset   = "\033[0m"
	Bold    = "\033[1m"
	Red     = "\033[31m"
	Green   = "\033[32m"
	Yellow  = "\033[33m"
	Blue    = "\033[34m"
	Magenta = "\033[35m"
	Cyan    = "\033[36m"
	Gray    = "\033[90m"
)

type Formatter struct {
	JSON    bool
	Verbose bool
	Quiet   bool
	NoColor bool
	Writer  io.Writer
}

func New(jsonOutput, verbose, quiet, noColor bool) *Formatter {
	return &Formatter{
		JSON:    jsonOutput,
		Verbose: verbose,
		Quiet:   quiet,
		NoColor: noColor,
		Writer:  os.Stdout,
	}
}

// Color wraps text in ANSI color codes if colors are enabled
func (f *Formatter) Color(color, text string) string {
	if f.NoColor || f.JSON {
		return text
	}
	return color + text + Reset
}

// Bold wraps text in bold if colors are enabled
func (f *Formatter) Bold(text string) string {
	return f.Color(Bold, text)
}

// Success color (green)
func (f *Formatter) SuccessText(text string) string {
	return f.Color(Green, text)
}

// Error color (red)
func (f *Formatter) ErrorText(text string) string {
	return f.Color(Red, text)
}

// Warning color (yellow)
func (f *Formatter) WarningText(text string) string {
	return f.Color(Yellow, text)
}

// Info color (cyan)
func (f *Formatter) InfoText(text string) string {
	return f.Color(Cyan, text)
}

// Muted color (gray)
func (f *Formatter) MutedText(text string) string {
	return f.Color(Gray, text)
}

func (f *Formatter) Print(v interface{}) error {
	if f.JSON {
		return f.PrintJSON(v)
	}
	fmt.Fprintln(f.Writer, v)
	return nil
}

func (f *Formatter) PrintJSON(v interface{}) error {
	enc := json.NewEncoder(f.Writer)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func (f *Formatter) PrintError(err error) {
	if f.JSON {
		f.PrintJSON(map[string]interface{}{
			"error":   true,
			"message": err.Error(),
		})
		return
	}
	fmt.Fprintf(os.Stderr, "%s %s\n", f.ErrorText("Error:"), err)
}

func (f *Formatter) PrintSuccess(message string) {
	if f.Quiet {
		return
	}
	if f.JSON {
		f.PrintJSON(map[string]interface{}{
			"success": true,
			"message": message,
		})
		return
	}
	fmt.Fprintln(f.Writer, f.SuccessText("✓")+" "+message)
}

func (f *Formatter) Verbosef(format string, args ...interface{}) {
	if f.Verbose && !f.Quiet {
		msg := fmt.Sprintf(format, args...)
		fmt.Fprintln(f.Writer, f.MutedText(msg))
	}
}

type TableWriter struct {
	w         *tabwriter.Writer
	headers   []string
	formatter *Formatter
}

func (f *Formatter) NewTable(headers ...string) *TableWriter {
	tw := &TableWriter{
		w:         tabwriter.NewWriter(f.Writer, 0, 0, 2, ' ', 0),
		headers:   headers,
		formatter: f,
	}
	if len(headers) > 0 {
		// Bold headers
		coloredHeaders := make([]string, len(headers))
		for i, h := range headers {
			coloredHeaders[i] = f.Bold(h)
		}
		fmt.Fprintln(tw.w, strings.Join(coloredHeaders, "\t"))
	}
	return tw
}

func (t *TableWriter) AddRow(values ...string) {
	fmt.Fprintln(t.w, strings.Join(values, "\t"))
}

func (t *TableWriter) Flush() {
	t.w.Flush()
}

type JSONResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
	Message string      `json:"message,omitempty"`
}

func (f *Formatter) Success(data interface{}) error {
	if f.JSON {
		return f.PrintJSON(JSONResponse{
			Success: true,
			Data:    data,
		})
	}
	return nil
}

func (f *Formatter) Error(err error) error {
	if f.JSON {
		return f.PrintJSON(JSONResponse{
			Success: false,
			Error:   err.Error(),
		})
	}
	fmt.Fprintf(os.Stderr, "%s %s\n", f.ErrorText("Error:"), err)
	return err
}
