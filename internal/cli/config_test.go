package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/bscott/pm-cli/internal/config"
	"github.com/bscott/pm-cli/internal/output"
)

func TestConfigShowCmdRunWithoutConfig(t *testing.T) {
	cmd := &ConfigShowCmd{}

	globals := &Globals{}
	ctx := &Context{
		Config:    nil,
		Formatter: output.New(false, false, false, false),
		Globals:   globals,
	}

	err := cmd.Run(ctx)
	if err == nil {
		t.Error("expected error when config is nil")
	}
}

func TestConfigShowCmdRunJSON(t *testing.T) {
	cmd := &ConfigShowCmd{}

	cfg := config.DefaultConfig()
	cfg.Bridge.Email = "test@example.com"
	cfg.Bridge.IMAPPort = 1143

	var buf bytes.Buffer
	formatter := output.New(true, false, false, false)
	formatter.Writer = &buf

	ctx := &Context{
		Config:    cfg,
		Formatter: formatter,
		Globals:   &Globals{JSON: true},
	}

	err := cmd.Run(ctx)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	// Verify JSON output
	var result map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("invalid JSON output: %v", err)
	}

	bridge, ok := result["bridge"].(map[string]interface{})
	if !ok {
		t.Fatal("expected bridge section in output")
	}

	if bridge["email"] != "test@example.com" {
		t.Errorf("email = %v, want %v", bridge["email"], "test@example.com")
	}
}

func TestConfigSetCmdRunWithInvalidKey(t *testing.T) {
	tests := []struct {
		name string
		key  string
	}{
		{"no dot", "invalid"},
		{"too many dots", "a.b.c"},
		{"unknown section", "unknown.key"},
		{"unknown bridge key", "bridge.unknown"},
		{"unknown defaults key", "defaults.unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := &ConfigSetCmd{
				Key:   tt.key,
				Value: "value",
			}

			ctx := &Context{
				Config:    config.DefaultConfig(),
				Formatter: output.New(false, false, false, false),
				Globals:   &Globals{},
			}

			err := cmd.Run(ctx)
			if err == nil {
				t.Errorf("expected error for invalid key %q", tt.key)
			}
		})
	}
}

func TestConfigSetCmdRunBridgeKeys(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "pm-cli-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	configPath := filepath.Join(tmpDir, "config.yaml")

	tests := []struct {
		name    string
		key     string
		value   string
		checker func(*config.Config) bool
	}{
		{
			name:  "set imap_host",
			key:   "bridge.imap_host",
			value: "192.168.1.1",
			checker: func(c *config.Config) bool {
				return c.Bridge.IMAPHost == "192.168.1.1"
			},
		},
		{
			name:  "set imap_port",
			key:   "bridge.imap_port",
			value: "9999",
			checker: func(c *config.Config) bool {
				return c.Bridge.IMAPPort == 9999
			},
		},
		{
			name:  "set smtp_host",
			key:   "bridge.smtp_host",
			value: "192.168.1.2",
			checker: func(c *config.Config) bool {
				return c.Bridge.SMTPHost == "192.168.1.2"
			},
		},
		{
			name:  "set smtp_port",
			key:   "bridge.smtp_port",
			value: "8888",
			checker: func(c *config.Config) bool {
				return c.Bridge.SMTPPort == 8888
			},
		},
		{
			name:  "set email",
			key:   "bridge.email",
			value: "new@example.com",
			checker: func(c *config.Config) bool {
				return c.Bridge.Email == "new@example.com"
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := config.DefaultConfig()

			var buf bytes.Buffer
			formatter := output.New(false, false, true, false) // Quiet mode
			formatter.Writer = &buf

			ctx := &Context{
				Config:    cfg,
				Formatter: formatter,
				Globals:   &Globals{Config: configPath},
			}

			cmd := &ConfigSetCmd{
				Key:   tt.key,
				Value: tt.value,
			}

			err := cmd.Run(ctx)
			if err != nil {
				t.Fatalf("Run() error = %v", err)
			}

			if !tt.checker(cfg) {
				t.Errorf("config value not set correctly for %s", tt.key)
			}
		})
	}
}

func TestConfigSetCmdRunDefaultsKeys(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "pm-cli-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	configPath := filepath.Join(tmpDir, "config.yaml")

	tests := []struct {
		name    string
		key     string
		value   string
		checker func(*config.Config) bool
	}{
		{
			name:  "set mailbox",
			key:   "defaults.mailbox",
			value: "Sent",
			checker: func(c *config.Config) bool {
				return c.Defaults.Mailbox == "Sent"
			},
		},
		{
			name:  "set limit",
			key:   "defaults.limit",
			value: "50",
			checker: func(c *config.Config) bool {
				return c.Defaults.Limit == 50
			},
		},
		{
			name:  "set format text",
			key:   "defaults.format",
			value: "text",
			checker: func(c *config.Config) bool {
				return c.Defaults.Format == "text"
			},
		},
		{
			name:  "set format json",
			key:   "defaults.format",
			value: "json",
			checker: func(c *config.Config) bool {
				return c.Defaults.Format == "json"
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := config.DefaultConfig()

			var buf bytes.Buffer
			formatter := output.New(false, false, true, false) // Quiet mode
			formatter.Writer = &buf

			ctx := &Context{
				Config:    cfg,
				Formatter: formatter,
				Globals:   &Globals{Config: configPath},
			}

			cmd := &ConfigSetCmd{
				Key:   tt.key,
				Value: tt.value,
			}

			err := cmd.Run(ctx)
			if err != nil {
				t.Fatalf("Run() error = %v", err)
			}

			if !tt.checker(cfg) {
				t.Errorf("config value not set correctly for %s", tt.key)
			}
		})
	}
}

func TestConfigSetCmdRunInvalidPortValue(t *testing.T) {
	cmd := &ConfigSetCmd{
		Key:   "bridge.imap_port",
		Value: "not-a-number",
	}

	ctx := &Context{
		Config:    config.DefaultConfig(),
		Formatter: output.New(false, false, false, false),
		Globals:   &Globals{},
	}

	err := cmd.Run(ctx)
	if err == nil {
		t.Error("expected error for invalid port value")
	}
}

func TestConfigSetCmdRunInvalidLimitValue(t *testing.T) {
	cmd := &ConfigSetCmd{
		Key:   "defaults.limit",
		Value: "not-a-number",
	}

	ctx := &Context{
		Config:    config.DefaultConfig(),
		Formatter: output.New(false, false, false, false),
		Globals:   &Globals{},
	}

	err := cmd.Run(ctx)
	if err == nil {
		t.Error("expected error for invalid limit value")
	}
}

func TestConfigSetCmdRunInvalidFormatValue(t *testing.T) {
	cmd := &ConfigSetCmd{
		Key:   "defaults.format",
		Value: "xml", // Invalid, must be 'text' or 'json'
	}

	ctx := &Context{
		Config:    config.DefaultConfig(),
		Formatter: output.New(false, false, false, false),
		Globals:   &Globals{},
	}

	err := cmd.Run(ctx)
	if err == nil {
		t.Error("expected error for invalid format value")
	}
}

func TestConfigSetCmdCreatesDefaultConfig(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "pm-cli-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	configPath := filepath.Join(tmpDir, "config.yaml")

	cmd := &ConfigSetCmd{
		Key:   "bridge.email",
		Value: "test@example.com",
	}

	var buf bytes.Buffer
	formatter := output.New(false, false, true, false) // Quiet mode
	formatter.Writer = &buf

	ctx := &Context{
		Config:    nil, // No config
		Formatter: formatter,
		Globals:   &Globals{Config: configPath},
	}

	err = cmd.Run(ctx)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	// Config should have been created
	if ctx.Config == nil {
		t.Error("expected config to be created")
	}
	if ctx.Config.Bridge.Email != "test@example.com" {
		t.Error("email not set in newly created config")
	}
}

func TestConfigValidateCmdRunWithoutConfig(t *testing.T) {
	cmd := &ConfigValidateCmd{}

	ctx := &Context{
		Config:    nil,
		Formatter: output.New(false, false, false, false),
		Globals:   &Globals{},
	}

	err := cmd.Run(ctx)
	if err == nil {
		t.Error("expected error when config is nil")
	}
}

func TestConfigDoctorSkipsSMTPAuthWhenSMTPPortUnreachable(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)
	t.Setenv("XDG_CONFIG_HOME", tmpHome)

	cfg := config.DefaultConfig()
	cfg.Bridge.Email = "test@example.com"
	cfg.Bridge.IMAPHost = "127.0.0.1"
	cfg.Bridge.IMAPPort = 1
	cfg.Bridge.SMTPHost = "127.0.0.1"
	cfg.Bridge.SMTPPort = 1

	var buf bytes.Buffer
	formatter := output.New(true, false, false, false)
	formatter.Writer = &buf

	cmd := &ConfigDoctorCmd{}
	ctx := &Context{
		Config:    cfg,
		Formatter: formatter,
		Globals:   &Globals{JSON: true},
	}

	if err := cmd.Run(ctx); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	var result struct {
		Checks []struct {
			Name    string `json:"name"`
			Status  string `json:"status"`
			Message string `json:"message"`
		} `json:"checks"`
	}
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}

	found := false
	for _, check := range result.Checks {
		if check.Name != "SMTP connection succeeds" {
			continue
		}
		found = true
		if check.Status != "fail" {
			t.Fatalf("status = %q, want %q", check.Status, "fail")
		}
		if check.Message != "cannot test - SMTP port not reachable" {
			t.Fatalf("message = %q, want %q", check.Message, "cannot test - SMTP port not reachable")
		}
	}

	if !found {
		t.Fatal("expected SMTP connection succeeds check in doctor output")
	}
}
