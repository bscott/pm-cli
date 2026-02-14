package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

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
