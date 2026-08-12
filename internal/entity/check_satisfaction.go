package entity

import (
	"bytes"
	"slices"
	"time"

	"github.com/google/uuid"
)

const CheckTimeLimitDefault = 30 * 24 * time.Hour

type CheckState string

const (
	CheckStateUnproven CheckState = "unproven"
	CheckStateProven   CheckState = "proven"
	CheckStateFailed   CheckState = "failed"
	CheckStateWaived   CheckState = "waived"
	CheckStateGap      CheckState = "gap"
)

func CheckStates() []CheckState {
	return []CheckState{
		CheckStateUnproven,
		CheckStateProven,
		CheckStateFailed,
		CheckStateWaived,
		CheckStateGap,
	}
}

func (s CheckState) Valid() bool {
	return slices.Contains(CheckStates(), s)
}

func (s CheckState) Settled() bool {
	return s == CheckStateProven || s == CheckStateWaived || s == CheckStateGap
}

type EvidenceExpiry string

const (
	EvidenceLive      EvidenceExpiry = ""
	EvidenceTimedOut  EvidenceExpiry = "time_limit"
	EvidenceHeadMoved EvidenceExpiry = "head_moved"
)

func (e EvidenceExpiry) Expired() bool {
	return e != EvidenceLive
}

type EvidenceHorizon struct {
	Now   time.Time
	Heads map[uuid.UUID]string
}

func HeadsOf(links []CodeLink) map[uuid.UUID]string {
	heads := make(map[uuid.UUID]string, len(links))

	for _, link := range links {
		if link.HeadSHA != "" {
			heads[link.ID] = link.HeadSHA
		}
	}

	return heads
}

func (h EvidenceHorizon) Moved(evidence Evidence) bool {
	if evidence.CodeLinkID == uuid.Nil || evidence.CommitSHA == "" {
		return false
	}

	head, known := h.Heads[evidence.CodeLinkID]

	return known && head != evidence.CommitSHA
}

type EvidenceRecord struct {
	Evidence Evidence
	Expiry   EvidenceExpiry
}

type CheckReport struct {
	Check    Check
	State    CheckState
	Evidence []EvidenceRecord
}

func (c Check) Window() time.Duration {
	if c.TimeLimit == nil || *c.TimeLimit <= 0 {
		return CheckTimeLimitDefault
	}

	return *c.TimeLimit
}

func (c Check) Expiry(evidence Evidence, horizon EvidenceHorizon) EvidenceExpiry {
	if !evidence.Verdict.Proves() {
		return EvidenceLive
	}

	if horizon.Moved(evidence) {
		return EvidenceHeadMoved
	}

	if !horizon.Now.Before(evidence.ReceivedAt.Add(c.Window())) {
		return EvidenceTimedOut
	}

	return EvidenceLive
}

func NewCheckReport(check Check, evidence []Evidence, horizon EvidenceHorizon) CheckReport {
	records := make([]EvidenceRecord, 0, len(evidence))

	for _, piece := range evidence {
		if piece.CheckID != check.ID {
			continue
		}

		records = append(records, EvidenceRecord{
			Evidence: piece,
			Expiry:   check.Expiry(piece, horizon),
		})
	}

	slices.SortFunc(records, func(a, b EvidenceRecord) int {
		if order := a.Evidence.ReceivedAt.Compare(b.Evidence.ReceivedAt); order != 0 {
			return order
		}

		return bytes.Compare(a.Evidence.ID[:], b.Evidence.ID[:])
	})

	return CheckReport{Check: check, State: check.StateFrom(records), Evidence: records}
}

func (c Check) StateFrom(records []EvidenceRecord) CheckState {
	switch c.Resolution {
	case CheckResolutionWaived:
		return CheckStateWaived
	case CheckResolutionGap:
		return CheckStateGap
	}

	proof, proven := c.newestProof(records)
	refusal, refuted := newestRefusal(records)

	if refuted && (!proven || !refusal.ReceivedAt.Before(proof.ReceivedAt)) {
		return CheckStateFailed
	}

	if !proven {
		return CheckStateUnproven
	}

	if c.Method.NeedsBothDirections() && !refutedBefore(records, proof) {
		return CheckStateUnproven
	}

	return CheckStateProven
}

func (c Check) newestProof(records []EvidenceRecord) (Evidence, bool) {
	var newest Evidence

	found := false

	for _, record := range records {
		if record.Expiry.Expired() || !record.Evidence.Verdict.Proves() {
			continue
		}

		if c.Method.NeedsAttestation() && !record.Evidence.Attested() {
			continue
		}

		if !found || record.Evidence.ReceivedAt.After(newest.ReceivedAt) {
			newest, found = record.Evidence, true
		}
	}

	return newest, found
}

func newestRefusal(records []EvidenceRecord) (Evidence, bool) {
	var newest Evidence

	found := false

	for _, record := range records {
		if !record.Evidence.Verdict.Disproves() {
			continue
		}

		if !found || record.Evidence.ReceivedAt.After(newest.ReceivedAt) {
			newest, found = record.Evidence, true
		}
	}

	return newest, found
}

func refutedBefore(records []EvidenceRecord, proof Evidence) bool {
	for _, record := range records {
		if record.Evidence.Verdict.Disproves() &&
			record.Evidence.ReceivedAt.Before(proof.ReceivedAt) {
			return true
		}
	}

	return false
}

type CheckAwaiting string

const (
	CheckAwaitingNothing        CheckAwaiting = ""
	CheckAwaitingCorrection     CheckAwaiting = "correction"
	CheckAwaitingFreshProof     CheckAwaiting = "fresh_proof"
	CheckAwaitingAttestation    CheckAwaiting = "attestation"
	CheckAwaitingPriorFailure   CheckAwaiting = "prior_failure"
	CheckAwaitingPositiveResult CheckAwaiting = "positive_result"
	CheckAwaitingEvidence       CheckAwaiting = "evidence"
)

func (r CheckReport) Awaiting() CheckAwaiting {
	switch r.State {
	case CheckStateProven, CheckStateWaived, CheckStateGap:
		return CheckAwaitingNothing
	case CheckStateFailed:
		return CheckAwaitingCorrection
	}

	if _, proven := r.Check.newestProof(r.Evidence); proven {
		return CheckAwaitingPriorFailure
	}

	if r.Check.Method.NeedsAttestation() && r.unattested() {
		return CheckAwaitingAttestation
	}

	if r.Expired() {
		return CheckAwaitingFreshProof
	}

	if r.RestsOnAbsence() {
		return CheckAwaitingPositiveResult
	}

	return CheckAwaitingEvidence
}

func (r CheckReport) unattested() bool {
	for _, record := range r.Evidence {
		if !record.Expiry.Expired() &&
			record.Evidence.Verdict.Proves() &&
			!record.Evidence.Attested() {
			return true
		}
	}

	return false
}

func (r CheckReport) Blocks() bool {
	return r.Check.Approval == CheckApprovalApproved && !r.State.Settled()
}

func (r CheckReport) Unratified() bool {
	return r.Check.Approval == CheckApprovalPending && r.Check.Resolution == CheckResolutionNone
}

func (r CheckReport) RestsOnAbsence() bool {
	if r.State != CheckStateUnproven {
		return false
	}

	for _, record := range r.Evidence {
		if record.Evidence.Verdict.RestsOnAbsence() {
			return true
		}
	}

	return false
}

func (r CheckReport) Expired() bool {
	if r.State != CheckStateUnproven {
		return false
	}

	for _, record := range r.Evidence {
		if record.Expiry.Expired() {
			return true
		}
	}

	return false
}

func ReportChecks(checks []Check, evidence []Evidence, horizon EvidenceHorizon) []CheckReport {
	reports := make([]CheckReport, 0, len(checks))

	for _, check := range checks {
		reports = append(reports, NewCheckReport(check, evidence, horizon))
	}

	return reports
}

func UnratifiedChecks(reports []CheckReport) []Check {
	unratified := make([]Check, 0, len(reports))

	for _, report := range reports {
		if report.Unratified() {
			unratified = append(unratified, report.Check)
		}
	}

	return unratified
}

func BlockingChecks(reports []CheckReport) []Check {
	blocking := make([]Check, 0, len(reports))

	for _, report := range reports {
		if report.Blocks() {
			blocking = append(blocking, report.Check)
		}
	}

	return blocking
}

type CheckSummary struct {
	Total            int
	Proven           int
	Unproven         int
	Failed           int
	Waived           int
	Gaps             int
	Expired          int
	Unapproved       int
	Blocking         int
	RestingOnAbsence int
}

func Summarise(reports []CheckReport) CheckSummary {
	summary := CheckSummary{Total: len(reports)}

	for _, report := range reports {
		switch report.State {
		case CheckStateProven:
			summary.Proven++
		case CheckStateUnproven:
			summary.Unproven++
		case CheckStateFailed:
			summary.Failed++
		case CheckStateWaived:
			summary.Waived++
		case CheckStateGap:
			summary.Gaps++
		}

		if report.Check.Approval != CheckApprovalApproved {
			summary.Unapproved++
		}

		if report.Blocks() {
			summary.Blocking++
		}

		if report.RestsOnAbsence() {
			summary.RestingOnAbsence++
		}

		if report.Expired() {
			summary.Expired++
		}
	}

	return summary
}
