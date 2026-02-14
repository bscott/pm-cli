package output

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"
)

type Formatter struct {
	JSON    bool
	Verbose bool
	Quiet   bool
	Writer  io.Writer
}

func New(jsonOutput, verbose, quiet bool) *Formatter {
	return &Formatter{
		JSON:    jsonOutput,
		Verbose: verbose,
		Quiet:   quiet,
		Writer:  os.Stdout,
	}
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
	fmt.Fprintf(os.Stderr, "Error: %s\n", err)
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
	fmt.Fprintln(f.Writer, message)
}

func (f *Formatter) Verbosef(format string, args ...interface{}) {
	if f.Verbose && !f.Quiet {
		fmt.Fprintf(f.Writer, format+"\n", args...)
	}
}

type TableWriter struct {
	w       *tabwriter.Writer
	headers []string
}

func (f *Formatter) NewTable(headers ...string) *TableWriter {
	tw := &TableWriter{
		w:       tabwriter.NewWriter(f.Writer, 0, 0, 2, ' ', 0),
		headers: headers,
	}
	if len(headers) > 0 {
		fmt.Fprintln(tw.w, strings.Join(headers, "\t"))
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
	fmt.Fprintf(os.Stderr, "Error: %s\n", err)
	return err
}
