package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/bscott/pm-cli/internal/imap"
	"github.com/bscott/pm-cli/internal/safetext"
)

// maxBatchInputSize caps batch input to guard against unbounded reads from
// stdin or an oversized file.
const maxBatchInputSize = 10 * 1024 * 1024 // 10 MB

// batchOp is a single operation in a batch request.
type batchOp struct {
	Op      string   `json:"op"`
	UIDs    []string `json:"uids"`
	Label   string   `json:"label,omitempty"`
	To      string   `json:"to,omitempty"`
	Mailbox string   `json:"mailbox,omitempty"`
	Read    bool     `json:"read,omitempty"`
	Unread  bool     `json:"unread,omitempty"`
	Star    bool     `json:"star,omitempty"`
	Unstar  bool     `json:"unstar,omitempty"`
}

// batchResult reports the outcome of a single operation.
type batchResult struct {
	Op      string `json:"op"`
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
}

// batchOutput is the top-level JSON response.
type batchOutput struct {
	Results   []batchResult `json:"results"`
	Total     int           `json:"total"`
	Succeeded int           `json:"succeeded"`
	Failed    int           `json:"failed"`
}

var validBatchOps = map[string]bool{
	"label":   true,
	"unlabel": true,
	"archive": true,
	"move":    true,
	"flag":    true,
	"delete":  true,
}

func (c *MailBatchCmd) Run(ctx *Context) error {
	if ctx.Config.Bridge.Email == "" {
		return fmt.Errorf("not configured - run 'pm-cli config init' first")
	}

	ops, err := readBatchOps(c.File)
	if err != nil {
		return err
	}

	// Validate every operation up front so malformed input fails before we
	// open a connection or mutate any mailbox.
	for i, op := range ops {
		if err := validateBatchOp(i, op); err != nil {
			return err
		}
	}

	client, err := imap.NewClient(ctx.Config)
	if err != nil {
		return err
	}
	if err := client.Connect(); err != nil {
		return err
	}
	defer client.Close()

	output := batchOutput{
		Results: make([]batchResult, 0, len(ops)),
		Total:   len(ops),
	}

	for _, op := range ops {
		result := executeBatchOp(client, op)
		output.Results = append(output.Results, result)
		if result.Success {
			output.Succeeded++
		} else {
			output.Failed++
			if c.StopOnError {
				break
			}
		}
	}

	return ctx.Formatter.PrintJSON(output)
}

// readBatchOps reads and decodes the batch request from a file or stdin.
func readBatchOps(file string) ([]batchOp, error) {
	var r io.Reader = os.Stdin
	if file != "" {
		f, err := os.Open(file)
		if err != nil {
			return nil, fmt.Errorf("failed to open batch file: %w", err)
		}
		defer f.Close()
		r = f
	}

	// Read one extra byte so we can detect input that exceeds the cap.
	data, err := io.ReadAll(io.LimitReader(r, maxBatchInputSize+1))
	if err != nil {
		return nil, fmt.Errorf("failed to read input: %w", err)
	}
	if len(data) > maxBatchInputSize {
		return nil, fmt.Errorf("input exceeds maximum size of %d bytes", maxBatchInputSize)
	}

	return decodeBatchOps(data)
}

// decodeBatchOps parses a JSON array of operations and rejects empty input.
func decodeBatchOps(data []byte) ([]batchOp, error) {
	var ops []batchOp
	if err := json.Unmarshal(data, &ops); err != nil {
		return nil, fmt.Errorf("invalid JSON input: %w", err)
	}
	if len(ops) == 0 {
		return nil, fmt.Errorf("no operations in input")
	}
	return ops, nil
}

// validateBatchOp checks required fields and rejects names containing IMAP
// special characters before any IMAP command is issued.
func validateBatchOp(index int, op batchOp) error {
	if !validBatchOps[op.Op] {
		return fmt.Errorf("operation %d: unknown op %q (valid: label, unlabel, archive, move, flag, delete)", index, op.Op)
	}
	if len(op.UIDs) == 0 {
		return fmt.Errorf("operation %d (%s): uids is required", index, op.Op)
	}
	if err := validateMailboxName(op.Mailbox); err != nil {
		return fmt.Errorf("operation %d (%s): source %w", index, op.Op, err)
	}

	switch op.Op {
	case "label", "unlabel":
		if op.Label == "" {
			return fmt.Errorf("operation %d (%s): label is required", index, op.Op)
		}
		if err := validateMailboxName(op.Label); err != nil {
			return fmt.Errorf("operation %d (%s): %w", index, op.Op, err)
		}
	case "move":
		if op.To == "" {
			return fmt.Errorf("operation %d (move): to is required", index)
		}
		if err := validateMailboxName(op.To); err != nil {
			return fmt.Errorf("operation %d (move): %w", index, err)
		}
	case "flag":
		if !op.Read && !op.Unread && !op.Star && !op.Unstar {
			return fmt.Errorf("operation %d (flag): at least one of read, unread, star, unstar is required", index)
		}
	}
	return nil
}

// validateMailboxName rejects names containing IMAP wildcards or CR/LF. An
// empty name is allowed (callers default it to INBOX).
func validateMailboxName(name string) error {
	if strings.ContainsAny(name, "{*%\r\n") {
		return fmt.Errorf("invalid mailbox/label name %q: contains IMAP special characters", name)
	}
	return nil
}

func executeBatchOp(client *imap.Client, op batchOp) batchResult {
	mailbox := op.Mailbox
	if mailbox == "" {
		mailbox = "INBOX"
	}

	var err error
	switch op.Op {
	case "label":
		err = client.CopyMessages(mailbox, op.UIDs, "Labels/"+op.Label)
	case "unlabel":
		err = client.DeleteMessages("Labels/"+op.Label, op.UIDs, true)
	case "archive":
		err = client.MoveMessages(mailbox, op.UIDs, "Archive")
	case "move":
		err = client.MoveMessages(mailbox, op.UIDs, op.To)
	case "flag":
		err = client.SetFlagsMultiple(mailbox, op.UIDs, op.Read, op.Unread, op.Star, op.Unstar)
	case "delete":
		err = client.DeleteMessages(mailbox, op.UIDs, false)
	}

	if err != nil {
		return batchResult{Op: op.Op, Error: safetext.SanitizeForTerminal(err.Error())}
	}
	return batchResult{Op: op.Op, Success: true}
}
