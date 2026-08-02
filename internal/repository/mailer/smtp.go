package mailer

import (
	"context"
	"fmt"

	"github.com/wneessen/go-mail"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/smtp"
	"github.com/usenorn/norn/internal/repository"
)

type mailerRepository struct {
	client *smtp.Client
}

func New(client *smtp.Client) repository.Mailer {
	return &mailerRepository{client: client}
}

func (r *mailerRepository) Send(ctx context.Context, message entity.Mail) error {
	if !r.client.Configured() {
		return entity.ErrMailDeliveryNotConfigured
	}

	envelope := mail.NewMsg()

	if err := envelope.FromFormat(r.client.FromName(), r.client.FromAddress()); err != nil {
		return fmt.Errorf("set mail sender: %w", err)
	}

	if err := envelope.To(message.To); err != nil {
		return fmt.Errorf("set mail recipient: %w", err)
	}

	envelope.Subject(message.Subject)
	envelope.SetBodyString(mail.TypeTextPlain, message.PlainBody)

	if message.HTMLBody != "" {
		envelope.AddAlternativeString(mail.TypeTextHTML, message.HTMLBody)
	}

	if err := r.client.DialAndSendWithContext(ctx, envelope); err != nil {
		return fmt.Errorf("send mail: %w", err)
	}

	return nil
}
