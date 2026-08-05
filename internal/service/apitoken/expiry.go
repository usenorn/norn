package apitoken

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/observability/logging"
)

func (s *apiTokensService) SweepExpiring(ctx context.Context) error {
	now := time.Now().UTC()

	widest := entity.APITokenExpiryNoticeDays[0]
	for _, candidate := range entity.APITokenExpiryNoticeDays {
		if candidate > widest {
			widest = candidate
		}
	}

	tokens, err := s.tokens.ListExpiring(ctx, now.AddDate(0, 0, widest))
	if err != nil {
		return err
	}

	failures := make([]error, 0)

	for _, token := range tokens {
		if token.ExpiresAt == nil {
			continue
		}

		daysLeft := entity.DaysUntil(*token.ExpiresAt, now)

		threshold, due := entity.ExpiryNoticeDue(
			entity.APITokenExpiryNoticeDays, daysLeft, token.ExpiryNoticeDays,
		)
		if !due {
			continue
		}

		if err := s.warn(ctx, token, daysLeft, now); err != nil {
			logging.From(ctx).ErrorContext(
				ctx,
				"warning an owner about an expiring api token failed",
				"token_id", token.ID.String(),
				"error", err.Error(),
			)

			failures = append(failures, fmt.Errorf(
				"warn the owner of %s about an expiring token: %w", token.ID, err,
			))

			continue
		}

		if err := s.tokens.RecordExpiryNotice(ctx, token.ID, threshold); err != nil {
			return err
		}
	}

	return errors.Join(failures...)
}

func (s *apiTokensService) warn(
	ctx context.Context,
	token entity.APIToken,
	daysLeft int,
	now time.Time,
) error {
	account, err := s.accounts.GetByID(ctx, token.AccountID)
	if err != nil {
		return err
	}

	if account.Status != entity.AccountStatusActive {
		return nil
	}

	reaches, err := s.names(ctx, token.Grants)
	if err != nil {
		return err
	}

	mail, err := buildTokenExpiry(account.Email, tokenExpiryContent{
		Name:        token.Name,
		ExpiresAt:   token.ExpiresAt.UTC().Format("2 January 2006"),
		DaysLeft:    daysLeft,
		Expired:     token.ExpiredAt(now),
		Workspaces:  reaches,
		SettingsURL: tokensURL(s.app.BaseURL),
	})
	if err != nil {
		return err
	}

	return s.mailer.Send(ctx, mail)
}

func (s *apiTokensService) names(ctx context.Context, grants entity.APITokenGrants) (string, error) {
	names := make([]string, 0, len(grants))

	for _, grant := range grants {
		workspace, err := s.workspaces.GetByID(ctx, grant.WorkspaceID)
		if err != nil {
			return "", err
		}

		names = append(names, workspace.Name)
	}

	return strings.Join(names, ", "), nil
}
