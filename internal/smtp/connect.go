package smtp

import (
	"fmt"
	"net"
	"net/smtp"
	"time"
)

// ConnectTimeout is the maximum time allowed for establishing SMTP TCP connections.
const ConnectTimeout = 5 * time.Second

var dialTimeout = net.DialTimeout

// DialClient connects to an SMTP server with an explicit timeout and
// returns a client that can be upgraded with STARTTLS.
func DialClient(addr, host string) (*smtp.Client, error) {
	conn, err := dialTimeout("tcp", addr, ConnectTimeout)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to SMTP server: %w", err)
	}

	client, err := smtp.NewClient(conn, host)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to create SMTP client: %w", err)
	}

	return client, nil
}
