package config

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/zalando/go-keyring"
	"gopkg.in/yaml.v3"
)

const (
	AppName         = "pm-cli"
	KeyringUser     = "bridge-password"
	DefaultIMAP     = "127.0.0.1"
	DefaultIMAPPort = 1143
	DefaultSMTP     = "127.0.0.1"
	DefaultSMTPPort = 1025

	// EnvBridgePassword is the environment variable consulted for the Bridge
	// password before falling back to the system keyring. This lets pm-cli run
	// on headless servers that have no D-Bus secret service available, where
	// keyring.Get fails with "org.freedesktop.secrets was not provided".
	EnvBridgePassword = "PM_CLI_BRIDGE_PASSWORD"
)

type BridgeConfig struct {
	IMAPHost string `yaml:"imap_host"`
	IMAPPort int    `yaml:"imap_port"`
	SMTPHost string `yaml:"smtp_host"`
	SMTPPort int    `yaml:"smtp_port"`
	Email    string `yaml:"email"`
}

type DefaultsConfig struct {
	Mailbox string `yaml:"mailbox"`
	Limit   int    `yaml:"limit"`
	Format  string `yaml:"format"`
}

type Config struct {
	Bridge   BridgeConfig   `yaml:"bridge"`
	Defaults DefaultsConfig `yaml:"defaults"`
}

func DefaultConfig() *Config {
	return &Config{
		Bridge: BridgeConfig{
			IMAPHost: DefaultIMAP,
			IMAPPort: DefaultIMAPPort,
			SMTPHost: DefaultSMTP,
			SMTPPort: DefaultSMTPPort,
		},
		Defaults: DefaultsConfig{
			Mailbox: "INBOX",
			Limit:   20,
			Format:  "text",
		},
	}
}

func ConfigDir() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("failed to get config directory: %w", err)
	}
	return filepath.Join(configDir, AppName), nil
}

func ConfigPath() (string, error) {
	dir, err := ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.yaml"), nil
}

func Load(path string) (*Config, error) {
	if path == "" {
		var err error
		path, err = ConfigPath()
		if err != nil {
			return nil, err
		}
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("config file not found at %s - run 'pm-cli config init' to create one", path)
		}
		return nil, fmt.Errorf("failed to read config: %w", err)
	}

	cfg := DefaultConfig()
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	return cfg, nil
}

func (c *Config) Save(path string) error {
	if path == "" {
		var err error
		path, err = ConfigPath()
		if err != nil {
			return err
		}
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	return nil
}

func (c *Config) SetPassword(password string) error {
	if c.Bridge.Email == "" {
		return errors.New("email must be set before storing password")
	}
	return keyring.Set(AppName, c.Bridge.Email, password)
}

func (c *Config) GetPassword() (string, error) {
	// Environment variable takes precedence over the keyring so that headless
	// and automated deployments (no D-Bus secret service) can supply the
	// Bridge password directly. Interactive users are unaffected: the variable
	// is only consulted when it is set and non-empty. Because the env var
	// carries the credential itself, no configured email is required for it.
	if pw := os.Getenv(EnvBridgePassword); pw != "" {
		return pw, nil
	}

	if c.Bridge.Email == "" {
		return "", errors.New("email not configured")
	}
	password, err := keyring.Get(AppName, c.Bridge.Email)
	if err != nil {
		if errors.Is(err, keyring.ErrNotFound) {
			return "", fmt.Errorf("password not found in keyring - run 'pm-cli config init' to set it")
		}
		return "", fmt.Errorf("failed to get password from keyring: %w", err)
	}
	return password, nil
}

func DeletePassword(email string) error {
	return keyring.Delete(AppName, email)
}

// secretEnvVars lists environment variables that carry pm-cli credentials and
// must never be handed to a child process.
var secretEnvVars = []string{EnvBridgePassword}

// ScrubSecrets returns env with every pm-cli credential variable removed.
//
// Any code spawning a child process must build its environment from this
// rather than from os.Environ() directly. Without it, a Bridge password
// supplied via EnvBridgePassword is inherited by user-supplied commands (for
// example `mail watch --exec`), handing the mail credential to arbitrary
// third-party scripts.
//
// The input slice is not modified.
func ScrubSecrets(env []string) []string {
	out := make([]string, 0, len(env))
	for _, kv := range env {
		if isSecretEnv(kv) {
			continue
		}
		out = append(out, kv)
	}
	return out
}

// isSecretEnv reports whether a "KEY=VALUE" entry names a credential variable.
// A bare "KEY" with no "=" is matched too, since Go permits such entries and
// dropping them is the safe direction.
func isSecretEnv(kv string) bool {
	name := kv
	if i := strings.IndexByte(kv, '='); i >= 0 {
		name = kv[:i]
	}
	for _, secret := range secretEnvVars {
		if name == secret {
			return true
		}
	}
	return false
}

func Exists() bool {
	path, err := ConfigPath()
	if err != nil {
		return false
	}
	_, err = os.Stat(path)
	return err == nil
}

// Idempotency support
//
// A key is reserved before the send is attempted, not recorded after it
// succeeds. Recording afterwards left a window in which two concurrent
// invocations both saw the key as unused and both sent, which defeats the
// only purpose of the mechanism.
//
// Each key is one marker file whose name is a hash of the key, created with
// O_EXCL. That makes the reservation atomic across processes without a lock
// file, and keeps a user-supplied key (which may contain path separators) from
// influencing the path. Expiry is read from the file's mtime, so there is no
// stored content that could fail to parse.

const idempotencyTTL = 24 * time.Hour

func idempotencyDir() (string, error) {
	dir, err := ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "idempotency"), nil
}

func idempotencyMarkerPath(key string) (string, error) {
	dir, err := idempotencyDir()
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256([]byte(key))
	return filepath.Join(dir, hex.EncodeToString(sum[:])), nil
}

// ReserveIdempotencyKey atomically claims key. It returns true when the caller
// owns the reservation and should proceed, and false when the key is already
// held within the TTL, meaning this is a duplicate.
//
// An empty key disables the mechanism and always reserves successfully.
//
// Errors are returned rather than swallowed: if the reservation state cannot
// be determined, the caller must not treat that as permission to send.
func ReserveIdempotencyKey(key string) (bool, error) {
	if key == "" {
		return true, nil
	}

	path, err := idempotencyMarkerPath(key)
	if err != nil {
		return false, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return false, fmt.Errorf("failed to create idempotency directory: %w", err)
	}

	purgeExpiredMarkers()

	for attempt := 0; attempt < 2; attempt++ {
		f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if err == nil {
			return true, f.Close()
		}
		if !os.IsExist(err) {
			return false, fmt.Errorf("failed to reserve idempotency key: %w", err)
		}

		// The marker exists. Honor it unless it has aged out.
		info, statErr := os.Stat(path)
		if statErr != nil {
			if os.IsNotExist(statErr) {
				continue // raced with a purge; try to claim it again
			}
			return false, fmt.Errorf("failed to read idempotency key: %w", statErr)
		}
		if time.Since(info.ModTime()) <= idempotencyTTL {
			return false, nil
		}
		if rmErr := os.Remove(path); rmErr != nil && !os.IsNotExist(rmErr) {
			return false, fmt.Errorf("failed to expire idempotency key: %w", rmErr)
		}
	}

	// Another process claimed it between our removal and retry. Treat that as
	// a duplicate rather than sending twice.
	return false, nil
}

// ReleaseIdempotencyKey drops a reservation, so a send that failed can be
// retried with the same key. It is not an error to release a key that is not
// held.
func ReleaseIdempotencyKey(key string) error {
	if key == "" {
		return nil
	}
	path, err := idempotencyMarkerPath(key)
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// purgeExpiredMarkers removes aged-out reservations opportunistically. Failures
// are ignored: this is housekeeping, and an unremoved marker only expires late.
func purgeExpiredMarkers() {
	dir, err := idempotencyDir()
	if err != nil {
		return
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		if time.Since(info.ModTime()) > idempotencyTTL {
			_ = os.Remove(filepath.Join(dir, e.Name()))
		}
	}
}
