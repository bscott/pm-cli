package cli

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/bscott/pm-cli/internal/config"
	"golang.org/x/term"
)

func (c *ConfigInitCmd) Run(ctx *Context) error {
	fmt.Println("ProtonMail CLI Configuration Wizard")
	fmt.Println("====================================")
	fmt.Println()
	fmt.Println("This wizard will help you configure pm-cli to connect to Proton Bridge.")
	fmt.Println("Make sure Proton Bridge is running before proceeding.")
	fmt.Println()

	reader := bufio.NewReader(os.Stdin)
	cfg := config.DefaultConfig()

	// Email
	fmt.Printf("ProtonMail email address: ")
	email, _ := reader.ReadString('\n')
	email = strings.TrimSpace(email)
	if email == "" {
		return fmt.Errorf("email address is required")
	}
	cfg.Bridge.Email = email

	// IMAP Host
	fmt.Printf("IMAP host [%s]: ", config.DefaultIMAP)
	imapHost, _ := reader.ReadString('\n')
	imapHost = strings.TrimSpace(imapHost)
	if imapHost != "" {
		cfg.Bridge.IMAPHost = imapHost
	}

	// IMAP Port
	fmt.Printf("IMAP port [%d]: ", config.DefaultIMAPPort)
	imapPortStr, _ := reader.ReadString('\n')
	imapPortStr = strings.TrimSpace(imapPortStr)
	if imapPortStr != "" {
		port, err := strconv.Atoi(imapPortStr)
		if err != nil {
			return fmt.Errorf("invalid IMAP port: %s", imapPortStr)
		}
		cfg.Bridge.IMAPPort = port
	}

	// SMTP Host
	fmt.Printf("SMTP host [%s]: ", config.DefaultSMTP)
	smtpHost, _ := reader.ReadString('\n')
	smtpHost = strings.TrimSpace(smtpHost)
	if smtpHost != "" {
		cfg.Bridge.SMTPHost = smtpHost
	}

	// SMTP Port
	fmt.Printf("SMTP port [%d]: ", config.DefaultSMTPPort)
	smtpPortStr, _ := reader.ReadString('\n')
	smtpPortStr = strings.TrimSpace(smtpPortStr)
	if smtpPortStr != "" {
		port, err := strconv.Atoi(smtpPortStr)
		if err != nil {
			return fmt.Errorf("invalid SMTP port: %s", smtpPortStr)
		}
		cfg.Bridge.SMTPPort = port
	}

	// Bridge Password (from Proton Bridge app)
	fmt.Println()
	fmt.Println("Enter your Proton Bridge password.")
	fmt.Println("(Find this in the Proton Bridge app under your account settings)")
	fmt.Print("Bridge password: ")

	passwordBytes, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Println()
	if err != nil {
		return fmt.Errorf("failed to read password: %w", err)
	}
	password := string(passwordBytes)
	if password == "" {
		return fmt.Errorf("bridge password is required")
	}

	// Save config
	configPath, err := config.ConfigPath()
	if err != nil {
		return err
	}

	if err := cfg.Save(""); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	// Store password in keyring
	if err := cfg.SetPassword(password); err != nil {
		return fmt.Errorf("failed to store password in keyring: %w", err)
	}

	fmt.Println()
	fmt.Printf("Configuration saved to %s\n", configPath)
	fmt.Println("Password stored securely in system keyring.")
	fmt.Println()
	fmt.Println("Test your connection with: pm-cli mailbox list")

	return nil
}

func (c *ConfigShowCmd) Run(ctx *Context) error {
	if ctx.Config == nil {
		return fmt.Errorf("no configuration found - run 'pm-cli config init' first")
	}

	if ctx.Formatter.JSON {
		return ctx.Formatter.PrintJSON(map[string]interface{}{
			"bridge": map[string]interface{}{
				"imap_host": ctx.Config.Bridge.IMAPHost,
				"imap_port": ctx.Config.Bridge.IMAPPort,
				"smtp_host": ctx.Config.Bridge.SMTPHost,
				"smtp_port": ctx.Config.Bridge.SMTPPort,
				"email":     ctx.Config.Bridge.Email,
			},
			"defaults": map[string]interface{}{
				"mailbox": ctx.Config.Defaults.Mailbox,
				"limit":   ctx.Config.Defaults.Limit,
				"format":  ctx.Config.Defaults.Format,
			},
		})
	}

	configPath, _ := config.ConfigPath()
	fmt.Printf("Configuration file: %s\n\n", configPath)

	fmt.Println("Bridge Settings:")
	fmt.Printf("  IMAP Host: %s\n", ctx.Config.Bridge.IMAPHost)
	fmt.Printf("  IMAP Port: %d\n", ctx.Config.Bridge.IMAPPort)
	fmt.Printf("  SMTP Host: %s\n", ctx.Config.Bridge.SMTPHost)
	fmt.Printf("  SMTP Port: %d\n", ctx.Config.Bridge.SMTPPort)
	fmt.Printf("  Email:     %s\n", ctx.Config.Bridge.Email)

	fmt.Println()
	fmt.Println("Defaults:")
	fmt.Printf("  Mailbox: %s\n", ctx.Config.Defaults.Mailbox)
	fmt.Printf("  Limit:   %d\n", ctx.Config.Defaults.Limit)
	fmt.Printf("  Format:  %s\n", ctx.Config.Defaults.Format)

	// Check if password is set
	_, err := ctx.Config.GetPassword()
	fmt.Println()
	if err != nil {
		fmt.Println("Password: not set (run 'pm-cli config init' to set)")
	} else {
		fmt.Println("Password: ********** (stored in keyring)")
	}

	return nil
}

func (c *ConfigSetCmd) Run(ctx *Context) error {
	if ctx.Config == nil {
		ctx.Config = config.DefaultConfig()
	}

	parts := strings.Split(c.Key, ".")
	if len(parts) != 2 {
		return fmt.Errorf("invalid key format - use section.key (e.g., bridge.email, defaults.limit)")
	}

	section, key := parts[0], parts[1]

	switch section {
	case "bridge":
		switch key {
		case "imap_host":
			ctx.Config.Bridge.IMAPHost = c.Value
		case "imap_port":
			port, err := strconv.Atoi(c.Value)
			if err != nil {
				return fmt.Errorf("invalid port value: %s", c.Value)
			}
			ctx.Config.Bridge.IMAPPort = port
		case "smtp_host":
			ctx.Config.Bridge.SMTPHost = c.Value
		case "smtp_port":
			port, err := strconv.Atoi(c.Value)
			if err != nil {
				return fmt.Errorf("invalid port value: %s", c.Value)
			}
			ctx.Config.Bridge.SMTPPort = port
		case "email":
			ctx.Config.Bridge.Email = c.Value
		default:
			return fmt.Errorf("unknown bridge key: %s", key)
		}
	case "defaults":
		switch key {
		case "mailbox":
			ctx.Config.Defaults.Mailbox = c.Value
		case "limit":
			limit, err := strconv.Atoi(c.Value)
			if err != nil {
				return fmt.Errorf("invalid limit value: %s", c.Value)
			}
			ctx.Config.Defaults.Limit = limit
		case "format":
			if c.Value != "text" && c.Value != "json" {
				return fmt.Errorf("format must be 'text' or 'json'")
			}
			ctx.Config.Defaults.Format = c.Value
		default:
			return fmt.Errorf("unknown defaults key: %s", key)
		}
	default:
		return fmt.Errorf("unknown section: %s (use 'bridge' or 'defaults')", section)
	}

	if err := ctx.Config.Save(ctx.Globals.Config); err != nil {
		return err
	}

	ctx.Formatter.PrintSuccess(fmt.Sprintf("Set %s = %s", c.Key, c.Value))
	return nil
}
