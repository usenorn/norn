package session

import (
	"context"
	"errors"
	"slices"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/service"
)

// A browser may present a cookie per signed-in session. Which one acts is never guessed: the
// caller names it, or a single live session speaks for itself, or the workspace in the path
// decides. A selector that names a session the request did not present authenticates nobody,
// because falling back to another would let a stale link act as the wrong person.
func (s *sessionsService) Resolve(
	ctx context.Context,
	input service.ResolveSessionsInput,
) (service.ResolvedSessions, error) {
	var resolved service.ResolvedSessions

	tokens := make(map[string]string, len(input.Presented))

	for _, candidate := range input.Presented {
		session, err := s.Inspect(ctx, candidate.Token)
		if err != nil {
			if !discardable(err) {
				return service.ResolvedSessions{}, err
			}

			resolved.Dead = append(resolved.Dead, candidate.Slot)

			continue
		}

		if session.Slot != candidate.Slot {
			resolved.Dead = append(resolved.Dead, candidate.Slot)

			continue
		}

		tokens[session.Slot] = candidate.Token
		resolved.Held = append(resolved.Held, session)
	}

	if len(resolved.Held) == 0 {
		return resolved, nil
	}

	acting, err := s.choose(ctx, resolved.Held, input)
	if err != nil {
		return service.ResolvedSessions{}, err
	}

	if acting.Slot == "" {
		return resolved, nil
	}

	refreshed, err := s.Validate(ctx, tokens[acting.Slot])
	if err != nil {
		if discardable(err) {
			return resolved, nil
		}

		return service.ResolvedSessions{}, err
	}

	resolved.Acting = refreshed
	resolved.Found = true

	for index, held := range resolved.Held {
		if held.Slot == refreshed.Slot {
			resolved.Held[index] = refreshed
		}
	}

	return resolved, nil
}

func (s *sessionsService) choose(
	ctx context.Context,
	held []entity.Session,
	input service.ResolveSessionsInput,
) (entity.Session, error) {
	if input.Selector != "" {
		for _, session := range held {
			if session.Slot == input.Selector {
				return session, nil
			}
		}

		return entity.Session{}, nil
	}

	if len(held) == 1 {
		return held[0], nil
	}

	if input.WorkspaceID == uuid.Nil {
		return entity.Session{}, nil
	}

	ordered := slices.Clone(held)
	slices.SortStableFunc(ordered, func(a, b entity.Session) int { return a.IssuedAt.Compare(b.IssuedAt) })

	for _, session := range ordered {
		if _, err := s.memberships.Get(ctx, input.WorkspaceID, session.AccountID); err != nil {
			if errors.Is(err, entity.ErrMembershipNotFound) {
				continue
			}

			return entity.Session{}, err
		}

		return session, nil
	}

	return entity.Session{}, nil
}

func discardable(err error) bool {
	return errors.Is(err, entity.ErrSessionNotFound) || errors.Is(err, entity.ErrSessionRevoked)
}
