package imports

import (
	"github.com/usenorn/norn/internal/entity"
)

type recorder struct {
	phase     entity.ImportPhase
	lines     []entity.ImportReportLine
	ledger    []entity.ImportLedgerEntry
	applied   []entity.ImportRecord
	restamped []entity.ImportLedgerEntry
}

func newRecorder(phase entity.ImportPhase) *recorder {
	return &recorder{phase: phase}
}

func (r *recorder) note(
	resource entity.ImportResource,
	externalID, subject string,
	outcome entity.ImportOutcome,
	detail map[string]string,
) {
	r.lines = append(r.lines, entity.ImportReportLine{
		Phase:      r.phase,
		Resource:   resource,
		Subject:    truncate(subject, entity.ImportSubjectMax),
		ExternalID: externalID,
		Outcome:    outcome,
		Detail:     detail,
	})
}

func (r *recorder) skipped(resource entity.ImportResource, externalID, subject, reason string) {
	r.note(resource, externalID, subject, entity.ImportOutcomeSkipped, map[string]string{"reason": reason})
}

func (r *recorder) failed(resource entity.ImportResource, externalID, subject, reason string) {
	r.note(resource, externalID, subject, entity.ImportOutcomeFailed, map[string]string{"reason": reason})
}

func (r *recorder) unattributed(resource entity.ImportResource, externalID, subject, sourceUser string) {
	r.note(resource, externalID, subject, entity.ImportOutcomeUnattributed, map[string]string{"source_user": sourceUser})
}

func (r *recorder) adjusted(resource entity.ImportResource, externalID, subject, was, became string) {
	r.note(resource, externalID, subject, entity.ImportOutcomeAdjusted, map[string]string{"was": was, "became": became})
}

func (r *recorder) dropped(resource entity.ImportResource, externalID, subject, reason string) {
	r.note(resource, externalID, subject, entity.ImportOutcomeDropped, map[string]string{"reason": reason})
}

func (r *recorder) created(entry entity.ImportLedgerEntry) {
	r.ledger = append(r.ledger, entry)
}

func (r *recorder) settled(record entity.ImportRecord) {
	r.applied = append(r.applied, record)
}

func (r *recorder) restamp(entry entity.ImportLedgerEntry) {
	r.restamped = append(r.restamped, entry)
}

// baseline is the version this run last left a row at: what the ledger recorded, unless
// this chunk has since moved the row itself and not yet committed the new number. Any
// version above it belongs to somebody outside the run.
func (r *recorder) baseline(entry entity.ImportLedgerEntry) int {
	version := entry.VersionAtCreate

	for _, stamped := range r.restamped {
		if stamped.Resource != entry.Resource || stamped.CreatedID != entry.CreatedID {
			continue
		}

		if stamped.VersionAtCreate > version {
			version = stamped.VersionAtCreate
		}
	}

	return version
}

func (r *recorder) reset() {
	r.lines = nil
	r.ledger = nil
	r.applied = nil
	r.restamped = nil
}

func truncate(value string, limit int) string {
	if len(value) <= limit {
		return value
	}

	return value[:limit]
}
