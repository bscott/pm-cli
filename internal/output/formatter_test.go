package output

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestNew(t *testing.T) {
	tests := []struct {
		name    string
		json    bool
		verbose bool
		quiet   bool
	}{
		{"default", false, false, false},
		{"json mode", true, false, false},
		{"verbose mode", false, true, false},
		{"quiet mode", false, false, true},
		{"all options", true, true, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := New(tt.json, tt.verbose, tt.quiet)
			if f == nil {
				t.Fatal("expected non-nil formatter")
			}
			if f.JSON != tt.json {
				t.Errorf("JSON = %v, want %v", f.JSON, tt.json)
			}
			if f.Verbose != tt.verbose {
				t.Errorf("Verbose = %v, want %v", f.Verbose, tt.verbose)
			}
			if f.Quiet != tt.quiet {
				t.Errorf("Quiet = %v, want %v", f.Quiet, tt.quiet)
			}
			if f.Writer == nil {
				t.Error("expected Writer to be set")
			}
		})
	}
}

func TestPrint(t *testing.T) {
	tests := []struct {
		name     string
		json     bool
		input    interface{}
		contains string
	}{
		{
			name:     "text mode prints value",
			json:     false,
			input:    "hello world",
			contains: "hello world",
		},
		{
			name:     "json mode prints JSON",
			json:     true,
			input:    map[string]string{"key": "value"},
			contains: `"key": "value"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			f := New(tt.json, false, false)
			f.Writer = &buf

			err := f.Print(tt.input)
			if err != nil {
				t.Fatalf("Print() error = %v", err)
			}

			if !strings.Contains(buf.String(), tt.contains) {
				t.Errorf("output = %q, want to contain %q", buf.String(), tt.contains)
			}
		})
	}
}

func TestPrintJSON(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected map[string]interface{}
	}{
		{
			name:     "simple map",
			input:    map[string]string{"key": "value"},
			expected: map[string]interface{}{"key": "value"},
		},
		{
			name:     "struct",
			input:    struct{ Name string }{Name: "test"},
			expected: map[string]interface{}{"Name": "test"},
		},
		{
			name:     "slice",
			input:    []int{1, 2, 3},
			expected: nil, // Will verify differently
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			f := New(true, false, false)
			f.Writer = &buf

			err := f.PrintJSON(tt.input)
			if err != nil {
				t.Fatalf("PrintJSON() error = %v", err)
			}

			// Verify valid JSON was written
			var result interface{}
			if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
				t.Errorf("invalid JSON output: %v", err)
			}
		})
	}
}

func TestPrintError(t *testing.T) {
	testErr := errors.New("test error message")

	t.Run("text mode", func(t *testing.T) {
		var buf bytes.Buffer
		f := New(false, false, false)
		f.Writer = &buf

		f.PrintError(testErr)

		// In text mode, error goes to stderr, not the writer.
		// The implementation uses os.Stderr directly for text mode.
	})

	t.Run("json mode", func(t *testing.T) {
		var buf bytes.Buffer
		f := New(true, false, false)
		f.Writer = &buf

		f.PrintError(testErr)

		var result map[string]interface{}
		if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
			t.Fatalf("invalid JSON: %v", err)
		}

		if result["error"] != true {
			t.Errorf("expected error=true, got %v", result["error"])
		}
		if result["message"] != "test error message" {
			t.Errorf("expected message='test error message', got %v", result["message"])
		}
	})
}

func TestPrintSuccess(t *testing.T) {
	t.Run("quiet mode suppresses output", func(t *testing.T) {
		var buf bytes.Buffer
		f := New(false, false, true)
		f.Writer = &buf

		f.PrintSuccess("should not appear")

		if buf.Len() != 0 {
			t.Errorf("expected empty output in quiet mode, got %q", buf.String())
		}
	})

	t.Run("text mode prints message", func(t *testing.T) {
		var buf bytes.Buffer
		f := New(false, false, false)
		f.Writer = &buf

		f.PrintSuccess("operation successful")

		if !strings.Contains(buf.String(), "operation successful") {
			t.Errorf("expected message in output, got %q", buf.String())
		}
	})

	t.Run("json mode prints JSON", func(t *testing.T) {
		var buf bytes.Buffer
		f := New(true, false, false)
		f.Writer = &buf

		f.PrintSuccess("operation successful")

		var result map[string]interface{}
		if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
			t.Fatalf("invalid JSON: %v", err)
		}

		if result["success"] != true {
			t.Errorf("expected success=true, got %v", result["success"])
		}
		if result["message"] != "operation successful" {
			t.Errorf("expected message='operation successful', got %v", result["message"])
		}
	})
}

func TestVerbosef(t *testing.T) {
	t.Run("verbose mode prints", func(t *testing.T) {
		var buf bytes.Buffer
		f := New(false, true, false)
		f.Writer = &buf

		f.Verbosef("verbose message: %s", "test")

		if !strings.Contains(buf.String(), "verbose message: test") {
			t.Errorf("expected verbose message, got %q", buf.String())
		}
	})

	t.Run("non-verbose mode suppresses", func(t *testing.T) {
		var buf bytes.Buffer
		f := New(false, false, false)
		f.Writer = &buf

		f.Verbosef("should not appear: %s", "test")

		if buf.Len() != 0 {
			t.Errorf("expected empty output, got %q", buf.String())
		}
	})

	t.Run("quiet mode overrides verbose", func(t *testing.T) {
		var buf bytes.Buffer
		f := New(false, true, true)
		f.Writer = &buf

		f.Verbosef("should not appear: %s", "test")

		if buf.Len() != 0 {
			t.Errorf("expected empty output when quiet, got %q", buf.String())
		}
	})
}

func TestTableWriter(t *testing.T) {
	t.Run("creates table with headers", func(t *testing.T) {
		var buf bytes.Buffer
		f := New(false, false, false)
		f.Writer = &buf

		table := f.NewTable("NAME", "AGE", "CITY")
		table.AddRow("Alice", "30", "NYC")
		table.AddRow("Bob", "25", "LA")
		table.Flush()

		output := buf.String()
		if !strings.Contains(output, "NAME") {
			t.Error("expected header NAME in output")
		}
		if !strings.Contains(output, "Alice") {
			t.Error("expected row data Alice in output")
		}
		if !strings.Contains(output, "Bob") {
			t.Error("expected row data Bob in output")
		}
	})

	t.Run("empty headers", func(t *testing.T) {
		var buf bytes.Buffer
		f := New(false, false, false)
		f.Writer = &buf

		table := f.NewTable()
		table.AddRow("value1", "value2")
		table.Flush()

		output := buf.String()
		if !strings.Contains(output, "value1") {
			t.Error("expected row data in output")
		}
	})
}

func TestSuccess(t *testing.T) {
	t.Run("json mode returns data", func(t *testing.T) {
		var buf bytes.Buffer
		f := New(true, false, false)
		f.Writer = &buf

		data := map[string]string{"key": "value"}
		err := f.Success(data)
		if err != nil {
			t.Fatalf("Success() error = %v", err)
		}

		var result JSONResponse
		if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
			t.Fatalf("invalid JSON: %v", err)
		}

		if !result.Success {
			t.Error("expected Success=true")
		}
	})

	t.Run("text mode returns nil", func(t *testing.T) {
		var buf bytes.Buffer
		f := New(false, false, false)
		f.Writer = &buf

		err := f.Success(map[string]string{"key": "value"})
		if err != nil {
			t.Fatalf("Success() error = %v", err)
		}

		// Text mode doesn't output anything for Success()
		if buf.Len() != 0 {
			t.Errorf("expected no output in text mode, got %q", buf.String())
		}
	})
}

func TestError(t *testing.T) {
	testErr := errors.New("test error")

	t.Run("json mode prints error response", func(t *testing.T) {
		var buf bytes.Buffer
		f := New(true, false, false)
		f.Writer = &buf

		// In JSON mode, Error() returns the result of PrintJSON (nil on success)
		err := f.Error(testErr)
		if err != nil {
			// If PrintJSON fails, that's the error returned
			t.Logf("Error() returned: %v", err)
		}

		var result JSONResponse
		if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
			t.Fatalf("invalid JSON: %v", err)
		}

		if result.Success {
			t.Error("expected Success=false")
		}
		if result.Error != "test error" {
			t.Errorf("expected Error='test error', got %q", result.Error)
		}
	})

	t.Run("text mode returns error", func(t *testing.T) {
		var buf bytes.Buffer
		f := New(false, false, false)
		f.Writer = &buf

		err := f.Error(testErr)
		if err != testErr {
			t.Errorf("expected original error returned, got %v", err)
		}
	})
}

func TestJSONResponseStruct(t *testing.T) {
	resp := JSONResponse{
		Success: true,
		Data:    "test data",
		Error:   "",
		Message: "test message",
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var result JSONResponse
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if result.Success != resp.Success {
		t.Errorf("Success = %v, want %v", result.Success, resp.Success)
	}
	if result.Message != resp.Message {
		t.Errorf("Message = %v, want %v", result.Message, resp.Message)
	}
}
