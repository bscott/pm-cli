package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/zalando/go-keyring"
	"gopkg.in/yaml.v3"
)

const (
	AppName       = "pm-cli"
	KeyringUser   = "bridge-password"
	DefaultIMAP   = "127.0.0.1"
	DefaultIMAPPort = 1143
	DefaultSMTP   = "127.0.0.1"
	DefaultSMTPPort = 1025
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

func Exists() bool {
	path, err := ConfigPath()
	if err != nil {
		return false
	}
	_, err = os.Stat(path)
	return err == nil
}

// Idempotency support

const idempotencyTTL = 24 * time.Hour

type idempotencyStore struct {
	Keys map[string]int64 `json:"keys"` // key -> unix timestamp
}

func idempotencyPath() (string, error) {
	dir, err := ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "idempotency.json"), nil
}

func loadIdempotencyStore() (*idempotencyStore, error) {
	path, err := idempotencyPath()
	if err != nil {
		return nil, err
	}

	store := &idempotencyStore{Keys: make(map[string]int64)}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return store, nil
		}
		return nil, err
	}

	if err := json.Unmarshal(data, store); err != nil {
		return store, nil // Return empty store on parse error
	}

	return store, nil
}

func (s *idempotencyStore) save() error {
	path, err := idempotencyPath()
	if err != nil {
		return err
	}

	// Clean expired keys
	now := time.Now().Unix()
	for key, ts := range s.Keys {
		if now-ts > int64(idempotencyTTL.Seconds()) {
			delete(s.Keys, key)
		}
	}

	data, err := json.Marshal(s)
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0600)
}

// CheckIdempotencyKey returns true if the key was already used (within TTL)
func CheckIdempotencyKey(key string) (bool, error) {
	if key == "" {
		return false, nil
	}

	store, err := loadIdempotencyStore()
	if err != nil {
		return false, err
	}

	ts, exists := store.Keys[key]
	if !exists {
		return false, nil
	}

	// Check if expired
	if time.Now().Unix()-ts > int64(idempotencyTTL.Seconds()) {
		return false, nil
	}

	return true, nil
}

// RecordIdempotencyKey marks a key as used
func RecordIdempotencyKey(key string) error {
	if key == "" {
		return nil
	}

	store, err := loadIdempotencyStore()
	if err != nil {
		return err
	}

	store.Keys[key] = time.Now().Unix()
	return store.save()
}
