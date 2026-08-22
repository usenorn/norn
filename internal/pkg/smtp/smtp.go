package smtp

import (
	"fmt"

	"github.com/wneessen/go-mail"

	"github.com/usenorn/norn/internal/config"
)

type Client struct {
	*mail.Client

	fromAddress string
	fromName    string
}

func New(cfg config.SMTP) (*Client, func(), error) {
	policy, err := tlsPolicy(cfg.TLSPolicy)
	if err != nil {
		return nil, nil, err
	}

	options := []mail.Option{
		mail.WithPort(cfg.Port),
		mail.WithTimeout(cfg.Timeout),
		mail.WithTLSPortPolicy(policy),
	}

	if cfg.AuthType != authTypeNone {
		auth, err := authType(cfg.AuthType)
		if err != nil {
			return nil, nil, err
		}

		options = append(options,
			mail.WithSMTPAuth(auth),
			mail.WithUsername(cfg.Username),
			mail.WithPassword(cfg.Password),
		)
	}

	client, err := mail.NewClient(cfg.Host, options...)
	if err != nil {
		return nil, nil, fmt.Errorf("create smtp client: %w", err)
	}

	cleanup := func() {
		_ = client.Close()
	}

	return &Client{
		Client:      client,
		fromAddress: cfg.FromAddress,
		fromName:    cfg.FromName,
	}, cleanup, nil
}

func (c *Client) FromAddress() string {
	return c.fromAddress
}

func (c *Client) FromName() string {
	return c.fromName
}

const authTypeNone = "none"

func tlsPolicy(name string) (mail.TLSPolicy, error) {
	switch name {
	case "none":
		return mail.NoTLS, nil
	case "opportunistic":
		return mail.TLSOpportunistic, nil
	case "mandatory":
		return mail.TLSMandatory, nil
	default:
		return 0, fmt.Errorf("unknown smtp tls policy %q", name)
	}
}

func authType(name string) (mail.SMTPAuthType, error) {
	switch name {
	case "plain":
		return mail.SMTPAuthPlain, nil
	case "login":
		return mail.SMTPAuthLogin, nil
	case "cram-md5":
		return mail.SMTPAuthCramMD5, nil
	default:
		return "", fmt.Errorf("unknown smtp auth type %q", name)
	}
}
