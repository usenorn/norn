package agentcapability

import (
	"bytes"
	"context"
	"strings"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/service"
)

func (s *capabilities) ResolveSkillSource(
	ctx context.Context,
	workspaceID uuid.UUID,
	source string,
) (entity.AgentSkillDiscovery, error) {
	if _, err := s.decide(ctx, workspaceID, entity.ActionManage); err != nil {
		return entity.AgentSkillDiscovery{}, err
	}

	location, err := entity.ParseAgentSkillLocation(source)
	if err != nil {
		return entity.AgentSkillDiscovery{}, err
	}

	return s.sources.Discover(ctx, location)
}

func (s *capabilities) ImportSkill(
	ctx context.Context,
	owner service.CapabilityOwner,
	input service.ImportSkillInput,
) (entity.AgentSkill, error) {
	decision, err := s.owns(ctx, owner)
	if err != nil {
		return entity.AgentSkill{}, err
	}

	location, err := entity.ParseAgentSkillLocation(input.Source)
	if err != nil {
		return entity.AgentSkill{}, err
	}

	discovery, err := s.sources.Discover(ctx, location)
	if err != nil {
		return entity.AgentSkill{}, err
	}

	candidate, ok := discovery.Pick(location)
	if input.Path != nil {
		candidate, ok = candidateAt(discovery, strings.Trim(*input.Path, "/"))
	}

	if !ok {
		return entity.AgentSkill{}, entity.NewValidationError(
			entity.FieldError{Field: "path", Code: entity.ValidationCodeRequired},
		)
	}

	bundle, err := s.sources.Fetch(ctx, location.Repository, discovery.Revision, candidate.Path)
	if err != nil {
		return entity.AgentSkill{}, err
	}

	return s.store(ctx, decision, owner, bundle, entity.AgentSkillGitHub, entity.AgentSkillOrigin{
		Repository: location.Repository,
		Path:       candidate.Path,
		Ref:        location.Ref,
		Revision:   discovery.Revision,
	})
}

func candidateAt(discovery entity.AgentSkillDiscovery, directory string) (entity.AgentSkillCandidate, bool) {
	for _, candidate := range discovery.Candidates {
		if candidate.Path == directory {
			return candidate, true
		}
	}

	return entity.AgentSkillCandidate{}, false
}

func (s *capabilities) WriteSkill(
	ctx context.Context,
	owner service.CapabilityOwner,
	draft service.SkillDraft,
) (entity.AgentSkill, error) {
	decision, err := s.owns(ctx, owner)
	if err != nil {
		return entity.AgentSkill{}, err
	}

	bundle, err := draftBundle(draft)
	if err != nil {
		return entity.AgentSkill{}, err
	}

	return s.store(ctx, decision, owner, bundle, entity.AgentSkillManual, entity.AgentSkillOrigin{})
}

func (s *capabilities) store(
	ctx context.Context,
	decision entity.Decision,
	owner service.CapabilityOwner,
	bundle entity.AgentSkillBundle,
	source entity.AgentSkillSource,
	origin entity.AgentSkillOrigin,
) (entity.AgentSkill, error) {
	manifest, err := bundle.Validate()
	if err != nil {
		return entity.AgentSkill{}, err
	}

	owned, err := s.skills.CountOwned(ctx, owner.WorkspaceID, owner.AgentID)
	if err != nil {
		return entity.AgentSkill{}, err
	}

	if owned >= entity.AgentSkillsPerOwner {
		return entity.AgentSkill{}, entity.ErrAgentSkillLimitReached
	}

	if err := s.skillNameFree(ctx, owner, manifest.Name, uuid.Nil); err != nil {
		return entity.AgentSkill{}, err
	}

	skill := entity.AgentSkill{
		ID:           uuid.New(),
		WorkspaceID:  owner.WorkspaceID,
		AgentID:      owner.AgentID,
		Name:         manifest.Name,
		Description:  manifest.Description,
		Source:       source,
		Origin:       origin,
		Instructions: manifest.Instructions,
		ContentHash:  bundle.Hash(),
		SizeBytes:    bundle.Size(),
		FileCount:    len(bundle.Files),
		CreatedBy:    decision.Actor.AccountID,
	}

	skill.ObjectKey = entity.AgentSkillObjectKey(skill.WorkspaceID, skill.ID, skill.ContentHash)

	if err := s.upload(ctx, skill.ObjectKey, bundle); err != nil {
		return entity.AgentSkill{}, err
	}

	created, err := s.skills.Create(ctx, skill)
	if err != nil {
		_ = s.blobs.Delete(ctx, skill.ObjectKey)

		return entity.AgentSkill{}, err
	}

	s.record(ctx, created.WorkspaceID, entity.AuditAgentSkillAdded, auditSkillKind, created.ID, created.Name, created.AgentID)

	return created, nil
}

func (s *capabilities) upload(ctx context.Context, key string, bundle entity.AgentSkillBundle) error {
	archive, err := pack(bundle)
	if err != nil {
		return err
	}

	return s.blobs.Put(ctx, key, entity.AgentSkillBundleContentType, bytes.NewReader(archive), int64(len(archive)))
}

func (s *capabilities) skillNameFree(
	ctx context.Context,
	owner service.CapabilityOwner,
	name string,
	except uuid.UUID,
) error {
	if owner.AgentID == nil {
		return nil
	}

	existing, err := s.skills.ListByAgent(ctx, owner.WorkspaceID, *owner.AgentID)
	if err != nil {
		return err
	}

	for _, skill := range existing {
		if skill.ID != except && strings.EqualFold(skill.Name, name) {
			return entity.ErrAgentSkillNameTaken
		}
	}

	return nil
}

func (s *capabilities) editable(ctx context.Context, workspaceID, skillID uuid.UUID) (entity.AgentSkill, error) {
	skill, err := s.skills.Get(ctx, workspaceID, skillID)
	if err != nil {
		return entity.AgentSkill{}, err
	}

	if _, err := s.owns(ctx, service.CapabilityOwner{WorkspaceID: workspaceID, AgentID: skill.AgentID}); err != nil {
		return entity.AgentSkill{}, err
	}

	return skill, nil
}

func (s *capabilities) RewriteSkill(
	ctx context.Context,
	workspaceID, skillID uuid.UUID,
	draft service.SkillDraft,
) (entity.AgentSkill, error) {
	skill, err := s.editable(ctx, workspaceID, skillID)
	if err != nil {
		return entity.AgentSkill{}, err
	}

	if skill.Source != entity.AgentSkillManual {
		return entity.AgentSkill{}, entity.ErrAgentSkillImported
	}

	bundle, err := draftBundle(draft)
	if err != nil {
		return entity.AgentSkill{}, err
	}

	return s.replace(ctx, skill, bundle, skill.Origin)
}

func (s *capabilities) PullSkill(ctx context.Context, workspaceID, skillID uuid.UUID) (entity.AgentSkill, error) {
	skill, err := s.editable(ctx, workspaceID, skillID)
	if err != nil {
		return entity.AgentSkill{}, err
	}

	if skill.Source != entity.AgentSkillGitHub {
		return entity.AgentSkill{}, entity.ErrAgentSkillNotImported
	}

	location := entity.AgentSkillLocation{
		Repository: skill.Origin.Repository,
		Ref:        skill.Origin.Ref,
		Path:       skill.Origin.Path,
	}

	discovery, err := s.sources.Discover(ctx, location)
	if err != nil {
		return entity.AgentSkill{}, err
	}

	if discovery.Revision == skill.Origin.Revision {
		return skill, nil
	}

	bundle, err := s.sources.Fetch(ctx, location.Repository, discovery.Revision, skill.Origin.Path)
	if err != nil {
		return entity.AgentSkill{}, err
	}

	origin := skill.Origin
	origin.Revision = discovery.Revision

	return s.replace(ctx, skill, bundle, origin)
}

func (s *capabilities) replace(
	ctx context.Context,
	skill entity.AgentSkill,
	bundle entity.AgentSkillBundle,
	origin entity.AgentSkillOrigin,
) (entity.AgentSkill, error) {
	manifest, err := bundle.Validate()
	if err != nil {
		return entity.AgentSkill{}, err
	}

	owner := service.CapabilityOwner{WorkspaceID: skill.WorkspaceID, AgentID: skill.AgentID}
	if err := s.skillNameFree(ctx, owner, manifest.Name, skill.ID); err != nil {
		return entity.AgentSkill{}, err
	}

	previous := skill.ObjectKey

	skill.Name = manifest.Name
	skill.Description = manifest.Description
	skill.Instructions = manifest.Instructions
	skill.Origin = origin
	skill.ContentHash = bundle.Hash()
	skill.SizeBytes = bundle.Size()
	skill.FileCount = len(bundle.Files)
	skill.ObjectKey = entity.AgentSkillObjectKey(skill.WorkspaceID, skill.ID, skill.ContentHash)

	if skill.ObjectKey != previous {
		if err := s.upload(ctx, skill.ObjectKey, bundle); err != nil {
			return entity.AgentSkill{}, err
		}
	}

	replaced, err := s.skills.Replace(ctx, skill)
	if err != nil {
		if skill.ObjectKey != previous {
			_ = s.blobs.Delete(ctx, skill.ObjectKey)
		}

		return entity.AgentSkill{}, err
	}

	if skill.ObjectKey != previous {
		_ = s.blobs.Delete(ctx, previous)
	}

	s.record(ctx, replaced.WorkspaceID, entity.AuditAgentSkillUpdated, auditSkillKind, replaced.ID, replaced.Name, replaced.AgentID)

	return replaced, nil
}

func (s *capabilities) DeleteSkill(ctx context.Context, workspaceID, skillID uuid.UUID) error {
	skill, err := s.editable(ctx, workspaceID, skillID)
	if err != nil {
		return err
	}

	if err := s.skills.Delete(ctx, workspaceID, skillID); err != nil {
		return err
	}

	_ = s.blobs.Delete(ctx, skill.ObjectKey)

	s.record(ctx, workspaceID, entity.AuditAgentSkillRemoved, auditSkillKind, skill.ID, skill.Name, skill.AgentID)

	return nil
}

func (s *capabilities) AttachSkill(ctx context.Context, workspaceID, agentID, skillID uuid.UUID) error {
	decision, err := s.agent(ctx, workspaceID, agentID, entity.ActionManage)
	if err != nil {
		return err
	}

	skill, err := s.skills.Get(ctx, workspaceID, skillID)
	if err != nil {
		return err
	}

	if !skill.InLibrary() {
		return entity.ErrAgentCapabilityNotLibrary
	}

	if err := s.skillNameFree(ctx, service.CapabilityOwner{WorkspaceID: workspaceID, AgentID: &agentID}, skill.Name, skill.ID); err != nil {
		return err
	}

	if err := s.skills.Attach(ctx, entity.AgentCapabilityAttachment{
		WorkspaceID:  workspaceID,
		AgentID:      agentID,
		CapabilityID: skillID,
		AttachedBy:   decision.Actor.AccountID,
	}); err != nil {
		return err
	}

	s.record(ctx, workspaceID, entity.AuditAgentCapabilityAttached, auditSkillKind, skill.ID, skill.Name, &agentID)

	return nil
}

func (s *capabilities) DetachSkill(ctx context.Context, workspaceID, agentID, skillID uuid.UUID) error {
	if _, err := s.agent(ctx, workspaceID, agentID, entity.ActionManage); err != nil {
		return err
	}

	skill, err := s.skills.Get(ctx, workspaceID, skillID)
	if err != nil {
		return err
	}

	if err := s.skills.Detach(ctx, workspaceID, agentID, skillID); err != nil {
		return err
	}

	s.record(ctx, workspaceID, entity.AuditAgentCapabilityDetached, auditSkillKind, skill.ID, skill.Name, &agentID)

	return nil
}
