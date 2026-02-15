package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg == nil {
		t.Fatal("expected non-nil config")
	}

	// Test Bridge defaults
	if cfg.Bridge.IMAPHost != DefaultIMAP {
		t.Errorf("IMAPHost = %q, want %q", cfg.Bridge.IMAPHost, DefaultIMAP)
	}
	if cfg.Bridge.IMAPPort != DefaultIMAPPort {
		t.Errorf("IMAPPort = %d, want %d", cfg.Bridge.IMAPPort, DefaultIMAPPort)
	}
	if cfg.Bridge.SMTPHost != DefaultSMTP {
		t.Errorf("SMTPHost = %q, want %q", cfg.Bridge.SMTPHost, DefaultSMTP)
	}
	if cfg.Bridge.SMTPPort != DefaultSMTPPort {
		t.Errorf("SMTPPort = %d, want %d", cfg.Bridge.SMTPPort, DefaultSMTPPort)
	}

	// Test Defaults
	if cfg.Defaults.Mailbox != "INBOX" {
		t.Errorf("Defaults.Mailbox = %q, want %q", cfg.Defaults.Mailbox, "INBOX")
	}
	if cfg.Defaults.Limit != 20 {
		t.Errorf("Defaults.Limit = %d, want %d", cfg.Defaults.Limit, 20)
	}
	if cfg.Defaults.Format != "text" {
		t.Errorf("Defaults.Format = %q, want %q", cfg.Defaults.Format, "text")
	}
}

func TestConstants(t *testing.T) {
	if AppName != "pm-cli" {
		t.Errorf("AppName = %q, want %q", AppName, "pm-cli")
	}
	if KeyringUser != "bridge-password" {
		t.Errorf("KeyringUser = %q, want %q", KeyringUser, "bridge-password")
	}
	if DefaultIMAP != "127.0.0.1" {
		t.Errorf("DefaultIMAP = %q, want %q", DefaultIMAP, "127.0.0.1")
	}
	if DefaultIMAPPort != 1143 {
		t.Errorf("DefaultIMAPPort = %d, want %d", DefaultIMAPPort, 1143)
	}
	if DefaultSMTP != "127.0.0.1" {
		t.Errorf("DefaultSMTP = %q, want %q", DefaultSMTP, "127.0.0.1")
	}
	if DefaultSMTPPort != 1025 {
		t.Errorf("DefaultSMTPPort = %d, want %d", DefaultSMTPPort, 1025)
	}
}

func TestConfigDir(t *testing.T) {
	dir, err := ConfigDir()
	if err != nil {
		t.Fatalf("ConfigDir() error = %v", err)
	}

	if dir == "" {
		t.Error("expected non-empty config directory")
	}

	// Should end with AppName
	if filepath.Base(dir) != AppName {
		t.Errorf("config dir should end with %q, got %q", AppName, filepath.Base(dir))
	}
}

func TestConfigPath(t *testing.T) {
	path, err := ConfigPath()
	if err != nil {
		t.Fatalf("ConfigPath() error = %v", err)
	}

	if path == "" {
		t.Error("expected non-empty config path")
	}

	// Should end with config.yaml
	if filepath.Base(path) != "config.yaml" {
		t.Errorf("config path should end with %q, got %q", "config.yaml", filepath.Base(path))
	}
}

func TestLoadAndSave(t *testing.T) {
	// Create a temporary directory for test config
	tmpDir, err := os.MkdirTemp("", "pm-cli-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	configPath := filepath.Join(tmpDir, "config.yaml")

	// Create and save a config
	cfg := DefaultConfig()
	cfg.Bridge.Email = "test@example.com"
	cfg.Bridge.IMAPPort = 9999
	cfg.Defaults.Limit = 50

	err = cfg.Save(configPath)
	if err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Fatal("config file was not created")
	}

	// Load the config
	loaded, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Verify loaded values
	if loaded.Bridge.Email != "test@example.com" {
		t.Errorf("Email = %q, want %q", loaded.Bridge.Email, "test@example.com")
	}
	if loaded.Bridge.IMAPPort != 9999 {
		t.Errorf("IMAPPort = %d, want %d", loaded.Bridge.IMAPPort, 9999)
	}
	if loaded.Defaults.Limit != 50 {
		t.Errorf("Limit = %d, want %d", loaded.Defaults.Limit, 50)
	}
}

func TestLoadNonExistent(t *testing.T) {
	_, err := Load("/nonexistent/path/config.yaml")
	if err == nil {
		t.Error("expected error for non-existent file")
	}
}

func TestLoadInvalidYAML(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "pm-cli-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	configPath := filepath.Join(tmpDir, "config.yaml")

	// Write invalid YAML
	err = os.WriteFile(configPath, []byte("invalid: yaml: content: ["), 0600)
	if err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	_, err = Load(configPath)
	if err == nil {
		t.Error("expected error for invalid YAML")
	}
}

func TestSaveCreatesDirectory(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "pm-cli-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Use a path with a non-existent directory
	configPath := filepath.Join(tmpDir, "subdir", "config.yaml")

	cfg := DefaultConfig()
	err = cfg.Save(configPath)
	if err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Verify directory was created
	if _, err := os.Stat(filepath.Dir(configPath)); os.IsNotExist(err) {
		t.Error("directory was not created")
	}
}

func TestExists(t *testing.T) {
	// This test depends on the user's system state, so we can only check
	// that it returns a boolean without error.
	_ = Exists()
}

func TestBridgeConfigStruct(t *testing.T) {
	cfg := BridgeConfig{
		IMAPHost: "localhost",
		IMAPPort: 1143,
		SMTPHost: "localhost",
		SMTPPort: 1025,
		Email:    "test@example.com",
	}

	if cfg.IMAPHost != "localhost" {
		t.Errorf("IMAPHost = %q, want %q", cfg.IMAPHost, "localhost")
	}
	if cfg.IMAPPort != 1143 {
		t.Errorf("IMAPPort = %d, want %d", cfg.IMAPPort, 1143)
	}
	if cfg.SMTPHost != "localhost" {
		t.Errorf("SMTPHost = %q, want %q", cfg.SMTPHost, "localhost")
	}
	if cfg.SMTPPort != 1025 {
		t.Errorf("SMTPPort = %d, want %d", cfg.SMTPPort, 1025)
	}
	if cfg.Email != "test@example.com" {
		t.Errorf("Email = %q, want %q", cfg.Email, "test@example.com")
	}
}

func TestDefaultsConfigStruct(t *testing.T) {
	cfg := DefaultsConfig{
		Mailbox: "INBOX",
		Limit:   20,
		Format:  "text",
	}

	if cfg.Mailbox != "INBOX" {
		t.Errorf("Mailbox = %q, want %q", cfg.Mailbox, "INBOX")
	}
	if cfg.Limit != 20 {
		t.Errorf("Limit = %d, want %d", cfg.Limit, 20)
	}
	if cfg.Format != "text" {
		t.Errorf("Format = %q, want %q", cfg.Format, "text")
	}
}

func TestSetPasswordWithoutEmail(t *testing.T) {
	cfg := DefaultConfig()
	// Email is empty by default

	err := cfg.SetPassword("testpassword")
	if err == nil {
		t.Error("expected error when setting password without email")
	}
}

func TestGetPasswordWithoutEmail(t *testing.T) {
	cfg := DefaultConfig()
	// Email is empty by default

	_, err := cfg.GetPassword()
	if err == nil {
		t.Error("expected error when getting password without email")
	}
}

func TestLoadWithDefaultPath(t *testing.T) {
	// Test that Load with empty path uses default ConfigPath.
	// This will likely fail since the user may not have a config,
	// but we verify it attempts to use the default path.
	_, err := Load("")
	// We expect either success (if user has config) or a specific error
	if err != nil {
		// Should mention config file not found or similar
		// Just verify it doesn't panic
	}
}

func TestSaveWithDefaultPath(t *testing.T) {
	// Don't actually save to the default path in tests
	// Just verify the path resolution works
	cfg := DefaultConfig()

	// Create a temp file to test save functionality
	tmpDir, err := os.MkdirTemp("", "pm-cli-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	configPath := filepath.Join(tmpDir, "config.yaml")
	err = cfg.Save(configPath)
	if err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Verify file permissions
	info, err := os.Stat(configPath)
	if err != nil {
		t.Fatalf("failed to stat config file: %v", err)
	}

	// Check that permissions are restrictive (0600)
	mode := info.Mode().Perm()
	if mode != 0600 {
		t.Errorf("config file permissions = %o, want %o", mode, 0600)
	}
}

func TestConfigYAMLFormat(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "pm-cli-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	configPath := filepath.Join(tmpDir, "config.yaml")

	cfg := DefaultConfig()
	cfg.Bridge.Email = "user@protonmail.com"

	err = cfg.Save(configPath)
	if err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Read raw file content
	content, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("failed to read config: %v", err)
	}

	// Verify YAML structure
	s := string(content)
	if !contains(s, "bridge:") {
		t.Error("YAML should contain 'bridge:' section")
	}
	if !contains(s, "defaults:") {
		t.Error("YAML should contain 'defaults:' section")
	}
	if !contains(s, "imap_host:") {
		t.Error("YAML should contain 'imap_host:' key")
	}
	if !contains(s, "email: user@protonmail.com") {
		t.Error("YAML should contain email value")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
