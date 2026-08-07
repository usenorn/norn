package notification

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/postgres"
	"github.com/usenorn/norn/internal/repository"
)

const audienceQuery = `
SELECT m.account_id::text
FROM workspace_memberships m
JOIN accounts a
    ON a.id = m.account_id
   AND a.status = 'active'
LEFT JOIN workspace_teams t ON t.id = $2::uuid AND t.workspace_id = m.workspace_id
LEFT JOIN workspace_team_members tm ON tm.team_id = t.id AND tm.account_id = m.account_id
WHERE m.workspace_id = $1
  AND m.account_id = ANY($3::uuid[])
  AND ($2::uuid IS NULL
       OR (t.id IS NOT NULL
           AND (m.role = 'admin' OR t.visibility = 'public' OR tm.account_id IS NOT NULL)))
ORDER BY m.account_id`

const deliverQuery = `
INSERT INTO workspace_notification_deliveries (
    event_id, account_id, workspace_id, reason, inbox, email
)
SELECT unnest($1::uuid[]), unnest($2::uuid[]), $3, unnest($4::text[]),
       unnest($5::boolean[]), unnest($6::boolean[])
ON CONFLICT (event_id, account_id) DO NOTHING`

const visibleDeliveries = `
SELECT d.event_id,
       d.reason,
       e.subject_kind,
       e.subject_id,
       e.kind,
       e.created_at,
       coalesce(e.actor_account_id::text, '') AS actor_id,
       e.actor_kind,
       coalesce(actor.display_name, '') AS actor_name,
       coalesce(i.title, p.name, t.name, '') AS title,
       coalesce(i.reference_key || '-' || i.number::text, '') AS reference,
       coalesce(it.key, t.key, '') AS team_key,
       r.read_through,
       r.snoozed_until
FROM workspace_notification_deliveries d
JOIN workspace_notification_events e ON e.id = d.event_id
JOIN workspace_memberships m
    ON m.workspace_id = d.workspace_id AND m.account_id = d.account_id
LEFT JOIN accounts actor ON actor.id = e.actor_account_id
LEFT JOIN workspace_issues i ON i.id = e.issue_id AND i.status = 'active'
LEFT JOIN workspace_teams it ON it.id = i.team_id AND it.status = 'active'
LEFT JOIN workspace_team_members itm ON itm.team_id = it.id AND itm.account_id = d.account_id
LEFT JOIN workspace_projects p ON p.id = e.project_id AND p.archived_at IS NULL
LEFT JOIN workspace_teams t ON t.id = e.team_id AND t.status = 'active'
LEFT JOIN workspace_team_members tm ON tm.team_id = t.id AND tm.account_id = d.account_id
LEFT JOIN workspace_notification_reads r
    ON r.workspace_id = d.workspace_id
   AND r.account_id = d.account_id
   AND r.subject_kind = e.subject_kind
   AND r.subject_id = e.subject_id
WHERE d.workspace_id = $1
  AND d.account_id = $2
  AND (e.issue_id IS NULL
       OR (i.id IS NOT NULL
           AND (m.role = 'admin' OR it.visibility = 'public' OR itm.account_id IS NOT NULL)))
  AND (e.project_id IS NULL OR p.id IS NOT NULL)
  AND (e.team_id IS NULL
       OR (t.id IS NOT NULL
           AND (m.role = 'admin' OR t.visibility = 'public' OR tm.account_id IS NOT NULL)))`

const unreadRows = `read_through IS NULL OR created_at > read_through`

const latest = `ORDER BY created_at DESC, event_id DESC))[1]`

const groupedSubjects = `
SELECT subject_kind,
       subject_id,
       (array_agg(kind ` + latest + `,
       (array_agg(actor_id ` + latest + `,
       (array_agg(actor_kind ` + latest + `,
       (array_agg(actor_name ` + latest + `,
       (array_agg(title ` + latest + `,
       (array_agg(reference ` + latest + `,
       (array_agg(team_key ` + latest + `,
       coalesce(string_agg(DISTINCT reason, ' ') FILTER (WHERE ` + unreadRows + `),
                string_agg(DISTINCT reason, ' ')) AS reasons,
       count(*) FILTER (WHERE ` + unreadRows + `) AS unread,
       max(created_at) AS last_event_at,
       max(read_through) AS read_through,
       max(snoozed_until) AS snoozed_until
FROM visible
GROUP BY subject_kind, subject_id`

const inboxQuery = `
WITH visible AS (` + visibleDeliveries + ` AND d.inbox
)` + groupedSubjects + `
HAVING (max(snoozed_until) IS NULL OR max(snoozed_until) <= now())
   AND ($3::boolean IS NOT TRUE OR count(*) FILTER (WHERE ` + unreadRows + `) > 0)
   AND ($4::boolean IS NOT TRUE
        OR (max(created_at), subject_id) < ($5::timestamptz, $6::uuid))
ORDER BY last_event_at DESC, subject_id DESC
LIMIT $7`

const unreadSubjectsQuery = `
WITH visible AS (` + visibleDeliveries + ` AND d.inbox
), grouped AS (` + groupedSubjects + `
HAVING (max(snoozed_until) IS NULL OR max(snoozed_until) <= now())
   AND count(*) FILTER (WHERE ` + unreadRows + `) > 0
)
SELECT count(*) FROM grouped`

const digestEntriesQuery = `
WITH visible AS (` + visibleDeliveries + `
  AND d.email
  AND d.created_at >= $3
  AND d.created_at < $4
  AND (r.read_through IS NULL OR e.created_at > r.read_through)
)` + groupedSubjects + `
ORDER BY last_event_at DESC, subject_id DESC`

const digestRecipientsQuery = `
SELECT DISTINCT d.workspace_id::text, d.account_id::text, a.email, coalesce(a.timezone, 'UTC')
FROM workspace_notification_deliveries d
JOIN workspace_notification_events e ON e.id = d.event_id
JOIN accounts a
    ON a.id = d.account_id
   AND a.status = 'active'
   AND a.kind = 'person'
   AND coalesce(a.email, '') <> ''
LEFT JOIN workspace_notification_reads r
    ON r.workspace_id = d.workspace_id
   AND r.account_id = d.account_id
   AND r.subject_kind = e.subject_kind
   AND r.subject_id = e.subject_id
WHERE d.email
  AND d.created_at >= $1
  AND d.created_at < $2
  AND (r.read_through IS NULL OR e.created_at > r.read_through)
ORDER BY d.workspace_id::text, d.account_id::text`

const markReadQuery = `
INSERT INTO workspace_notification_reads (
    workspace_id, account_id, subject_kind, subject_id, read_through
)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (workspace_id, account_id, subject_kind, subject_id)
DO UPDATE SET read_through = excluded.read_through, updated_at = now()`

const markAllReadQuery = `
INSERT INTO workspace_notification_reads (
    workspace_id, account_id, subject_kind, subject_id, read_through
)
SELECT DISTINCT d.workspace_id, d.account_id, e.subject_kind, e.subject_id, $3::timestamptz
FROM workspace_notification_deliveries d
JOIN workspace_notification_events e ON e.id = d.event_id
WHERE d.workspace_id = $1 AND d.account_id = $2 AND d.inbox
ON CONFLICT (workspace_id, account_id, subject_kind, subject_id)
DO UPDATE SET read_through = excluded.read_through, updated_at = now()`

const snoozeQuery = `
INSERT INTO workspace_notification_reads (
    workspace_id, account_id, subject_kind, subject_id, snoozed_until
)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (workspace_id, account_id, subject_kind, subject_id)
DO UPDATE SET snoozed_until = excluded.snoozed_until, updated_at = now()`

const claimDigestQuery = `
INSERT INTO workspace_notification_digests (workspace_id, account_id, window_started_at)
VALUES ($1, $2, $3)
ON CONFLICT (workspace_id, account_id, window_started_at) DO NOTHING
RETURNING window_started_at`

const digestSentQuery = `
UPDATE workspace_notification_digests
SET sent_at = now(), updated_at = now()
WHERE workspace_id = $1 AND account_id = $2 AND window_started_at = $3`

const digestFailedQuery = `
UPDATE workspace_notification_digests
SET failed_at = now(), failure = $4, updated_at = now()
WHERE workspace_id = $1 AND account_id = $2 AND window_started_at = $3`

type notificationRepository struct {
	db *postgres.Client
}

func New(db *postgres.Client) repository.Notification {
	return &notificationRepository{db: db}
}

func (r *notificationRepository) Audience(
	ctx context.Context,
	workspaceID, teamID uuid.UUID,
	accountIDs []uuid.UUID,
) ([]uuid.UUID, error) {
	if len(accountIDs) == 0 {
		return nil, nil
	}

	candidates := make([]string, 0, len(accountIDs))
	for _, accountID := range accountIDs {
		candidates = append(candidates, accountID.String())
	}

	scope := any(nil)
	if teamID != uuid.Nil {
		scope = teamID.String()
	}

	rows, err := r.db.Querier(ctx).QueryContext(
		ctx, audienceQuery, workspaceID.String(), scope, candidates,
	)
	if err != nil {
		return nil, fmt.Errorf("read notification audience: %w", err)
	}

	defer func() { _ = rows.Close() }()

	audience := make([]uuid.UUID, 0, len(accountIDs))

	for rows.Next() {
		var accountID string

		if err := rows.Scan(&accountID); err != nil {
			return nil, fmt.Errorf("scan notification audience: %w", err)
		}

		audience = append(audience, parse(accountID))
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read notification audience: %w", err)
	}

	return audience, nil
}

func (r *notificationRepository) Deliver(
	ctx context.Context,
	workspaceID uuid.UUID,
	deliveries []repository.NotificationDelivery,
) error {
	if len(deliveries) == 0 {
		return nil
	}

	events := make([]string, 0, len(deliveries))
	accounts := make([]string, 0, len(deliveries))
	reasons := make([]string, 0, len(deliveries))
	inbox := make([]bool, 0, len(deliveries))
	email := make([]bool, 0, len(deliveries))

	for _, delivery := range deliveries {
		events = append(events, delivery.EventID.String())
		accounts = append(accounts, delivery.AccountID.String())
		reasons = append(reasons, string(delivery.Reason))
		inbox = append(inbox, delivery.Channels.Inbox)
		email = append(email, delivery.Channels.Email)
	}

	if _, err := r.db.Querier(ctx).ExecContext(
		ctx, deliverQuery,
		events, accounts, workspaceID.String(), reasons, inbox, email,
	); err != nil {
		return fmt.Errorf("deliver notifications: %w", err)
	}

	return nil
}

func (r *notificationRepository) ListInbox(
	ctx context.Context,
	workspaceID, accountID uuid.UUID,
	page entity.NotificationPage,
) ([]entity.Notification, error) {
	cursorAt := time.Time{}
	cursorID := uuid.Nil.String()

	if page.Cursor != nil {
		cursorAt = page.Cursor.LastEventAt
		cursorID = page.Cursor.SubjectID.String()
	}

	return r.grouped(
		ctx, inboxQuery, workspaceID, accountID,
		workspaceID.String(), accountID.String(),
		page.Filter == entity.NotificationFilterUnread,
		page.Cursor != nil, cursorAt, cursorID, page.Limit,
	)
}

func (r *notificationRepository) DigestEntries(
	ctx context.Context,
	workspaceID, accountID uuid.UUID,
	window time.Time,
) ([]entity.Notification, error) {
	return r.grouped(
		ctx, digestEntriesQuery, workspaceID, accountID,
		workspaceID.String(), accountID.String(), window, window.Add(entity.NotificationDigestWindow),
	)
}

func (r *notificationRepository) Unread(ctx context.Context, workspaceID, accountID uuid.UUID) (int, error) {
	var unread int

	if err := r.db.Querier(ctx).QueryRowContext(
		ctx, unreadSubjectsQuery, workspaceID.String(), accountID.String(),
	).Scan(&unread); err != nil {
		return 0, fmt.Errorf("count unread notifications: %w", err)
	}

	return unread, nil
}

func (r *notificationRepository) MarkRead(
	ctx context.Context,
	workspaceID, accountID uuid.UUID,
	subject entity.NotificationSubject,
	at time.Time,
) error {
	if _, err := r.db.Querier(ctx).ExecContext(
		ctx, markReadQuery,
		workspaceID.String(), accountID.String(),
		string(subject.Kind), subject.ID.String(), at,
	); err != nil {
		return fmt.Errorf("mark notification read: %w", err)
	}

	return nil
}

func (r *notificationRepository) MarkAllRead(
	ctx context.Context,
	workspaceID, accountID uuid.UUID,
	at time.Time,
) error {
	if _, err := r.db.Querier(ctx).ExecContext(
		ctx, markAllReadQuery, workspaceID.String(), accountID.String(), at,
	); err != nil {
		return fmt.Errorf("mark notifications read: %w", err)
	}

	return nil
}

func (r *notificationRepository) Snooze(
	ctx context.Context,
	workspaceID, accountID uuid.UUID,
	subject entity.NotificationSubject,
	until time.Time,
) error {
	if _, err := r.db.Querier(ctx).ExecContext(
		ctx, snoozeQuery,
		workspaceID.String(), accountID.String(),
		string(subject.Kind), subject.ID.String(), until,
	); err != nil {
		return fmt.Errorf("snooze notification: %w", err)
	}

	return nil
}

func (r *notificationRepository) DigestRecipients(
	ctx context.Context,
	window time.Time,
) ([]entity.NotificationDigestClaim, error) {
	rows, err := r.db.Querier(ctx).QueryContext(
		ctx, digestRecipientsQuery, window, window.Add(entity.NotificationDigestWindow),
	)
	if err != nil {
		return nil, fmt.Errorf("read digest recipients: %w", err)
	}

	defer func() { _ = rows.Close() }()

	claims := make([]entity.NotificationDigestClaim, 0)

	for rows.Next() {
		var (
			claim              entity.NotificationDigestClaim
			workspace, account string
		)

		if err := rows.Scan(&workspace, &account, &claim.Email, &claim.Timezone); err != nil {
			return nil, fmt.Errorf("scan digest recipient: %w", err)
		}

		claim.WorkspaceID = parse(workspace)
		claim.AccountID = parse(account)
		claim.Window = window

		claims = append(claims, claim)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read digest recipients: %w", err)
	}

	return claims, nil
}

func (r *notificationRepository) ClaimDigest(
	ctx context.Context,
	claim entity.NotificationDigestClaim,
) (bool, error) {
	rows, err := r.db.Querier(ctx).QueryContext(
		ctx, claimDigestQuery,
		claim.WorkspaceID.String(), claim.AccountID.String(), claim.Window,
	)
	if err != nil {
		return false, fmt.Errorf("claim digest: %w", err)
	}

	defer func() { _ = rows.Close() }()

	claimed := rows.Next()

	if err := rows.Err(); err != nil {
		return false, fmt.Errorf("claim digest: %w", err)
	}

	return claimed, nil
}

func (r *notificationRepository) RecordDigestOutcome(
	ctx context.Context,
	claim entity.NotificationDigestClaim,
	failure error,
) error {
	query := digestSentQuery
	args := []any{claim.WorkspaceID.String(), claim.AccountID.String(), claim.Window}

	if failure != nil {
		query = digestFailedQuery
		args = append(args, failure.Error())
	}

	if _, err := r.db.Querier(ctx).ExecContext(ctx, query, args...); err != nil {
		return fmt.Errorf("record digest outcome: %w", err)
	}

	return nil
}

func (r *notificationRepository) grouped(
	ctx context.Context,
	query string,
	workspaceID, accountID uuid.UUID,
	args ...any,
) ([]entity.Notification, error) {
	rows, err := r.db.Querier(ctx).QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("read notifications: %w", err)
	}

	defer func() { _ = rows.Close() }()

	notifications := make([]entity.Notification, 0)

	for rows.Next() {
		var (
			notification            entity.Notification
			subjectKind, subject    string
			kind, actor, actorKind  string
			reasons                 string
			readThrough, snoozedAt  *time.Time
			title, reference, teamK string
		)

		if err := rows.Scan(
			&subjectKind, &subject, &kind, &actor, &actorKind, &notification.ActorName,
			&title, &reference, &teamK, &reasons,
			&notification.UnreadCount, &notification.LastEventAt, &readThrough, &snoozedAt,
		); err != nil {
			return nil, fmt.Errorf("scan notification: %w", err)
		}

		notification.WorkspaceID = workspaceID
		notification.AccountID = accountID
		notification.Subject = entity.NotificationSubject{
			Kind: entity.NotificationSubjectKind(subjectKind),
			ID:   parse(subject),
		}
		notification.Kind = entity.NotificationKind(kind)
		notification.Actor = parse(actor)
		notification.ActorKind = entity.ActorKind(actorKind)
		notification.Title = title
		notification.Reference = reference
		notification.TeamKey = teamK
		notification.Reason = strongest(reasons)

		if readThrough != nil {
			notification.ReadThrough = *readThrough
		}

		if snoozedAt != nil {
			notification.SnoozedUntil = *snoozedAt
		}

		notifications = append(notifications, notification)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read notifications: %w", err)
	}

	return notifications, nil
}

func strongest(reasons string) entity.NotificationReason {
	strongest := entity.NotificationReasonFollowing

	for _, raw := range strings.Fields(reasons) {
		strongest = strongest.Strongest(entity.NotificationReason(raw))
	}

	return strongest
}

func parse(raw string) uuid.UUID {
	parsed, err := uuid.Parse(raw)
	if err != nil {
		return uuid.Nil
	}

	return parsed
}
