package imports_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/service"
)

const (
	sourceKey      = "lin_api_01HZXK"
	sourceSettings = `{"team":"CORE"}`
	laterSettings  = `{"team":"PLATFORM"}`
)

func TestARunPastStagingWillNotHaveItsSourceChangedUnderneathIt(t *testing.T) {
	h := newHarness(t).backed().allow(entity.Decision{})

	h.configured(sourceKey, json.RawMessage(sourceSettings))
	h.at(entity.ImportMapped)

	_, err := h.imports.Configure(context.Background(), h.run().WorkspaceID, h.run().ID,
		service.ConfigureImportInput{
			Secret:   "lin_api_somebody_elses",
			Settings: json.RawMessage(laterSettings),
		})

	if !errors.Is(err, entity.ErrImportStatusTransition) {
		t.Fatalf(
			"configuring a mapped run returned %v, want the transition refusal. The rows already "+
				"staged were read with the key and the selection this run was given, and a run whose "+
				"configuration no longer explains its own rows can neither be resumed nor re-read.",
			err,
		)
	}

	if h.world.secret != sourceKey {
		t.Errorf("the stored key became %q despite the refusal", h.world.secret)
	}

	if string(h.run().Settings) != sourceSettings {
		t.Errorf("the stored settings became %q despite the refusal", h.run().Settings)
	}
}

func TestSavingTheSettingsAgainDoesNotAskForTheKeyASecondTime(t *testing.T) {
	h := newHarness(t).backed().allow(entity.Decision{})

	if _, err := h.imports.Configure(context.Background(), h.run().WorkspaceID, h.run().ID,
		service.ConfigureImportInput{
			Secret:   sourceKey,
			Settings: json.RawMessage(sourceSettings),
		}); err != nil {
		t.Fatalf("configure: %v", err)
	}

	run, err := h.imports.Configure(context.Background(), h.run().WorkspaceID, h.run().ID,
		service.ConfigureImportInput{
			Settings:          json.RawMessage(laterSettings),
			UnknownReferences: entity.ImportUnknownFail,
		})
	if err != nil {
		t.Fatalf("reconfigure: %v", err)
	}

	if h.world.secret != sourceKey {
		t.Fatalf(
			"saving the settings without the key left the stored key as %q. The key is never read "+
				"back to whoever is configuring the run, so an empty field means they did not retype "+
				"it rather than that they want it cleared — and clearing it strands a staging pass "+
				"that has no way to ask for it again.",
			h.world.secret,
		)
	}

	if !run.SourceSecretSet {
		t.Error("the run no longer reports that a key is stored, so the wizard would ask for it again")
	}

	if string(run.Settings) != laterSettings {
		t.Errorf("the settings read back as %q, want %q", run.Settings, laterSettings)
	}

	if run.UnknownReferences != entity.ImportUnknownFail {
		t.Errorf("the unknown-reference policy read back as %q, want fail", run.UnknownReferences)
	}
}

func TestEveryPageIsFetchedWithTheConfigurationTheRunWasGiven(t *testing.T) {
	h := newHarness(t).backed()

	h.configured(sourceKey, json.RawMessage(sourceSettings))

	held := []entity.ImportRecord{
		fetched(t, "issue-one", "", service.ImportIssuePayload{Title: "One", Team: sourceTeam}),
		fetched(t, "issue-two", "", service.ImportIssuePayload{Title: "Two", Team: sourceTeam}),
		fetched(t, "issue-three", "", service.ImportIssuePayload{Title: "Three", Team: sourceTeam}),
		fetched(t, "issue-four", "", service.ImportIssuePayload{Title: "Four", Team: sourceTeam}),
	}

	asked := make([]service.ImportSourceConfig, 0, len(held))

	h.offering(&scriptedSource{
		answer: func(request service.ImportFetchRequest) (service.ImportFetchPage, error) {
			asked = append(asked, request.Config)

			return pageOf(held, request), nil
		},
	})

	if err := h.runner.RunStage(context.Background(), stagePayload(h)); err != nil {
		t.Fatalf("run stage: %v", err)
	}

	if len(asked) < 2 {
		t.Fatalf("the source was asked for %d pages, so this proves nothing about later ones", len(asked))
	}

	for page, config := range asked {
		if config.Secret != sourceKey {
			t.Errorf(
				"page %d was fetched with the key %q rather than the run's own. A source is handed "+
					"its credentials on every call because staging runs in slices across workers, and "+
					"a page fetched without them either fails or, worse, reads somebody else's data.",
				page, config.Secret,
			)
		}

		if string(config.Settings) != sourceSettings {
			t.Errorf(
				"page %d was fetched with the settings %q rather than %q, so the team selection made "+
					"before staging stops applying part way through",
				page, config.Settings, sourceSettings,
			)
		}
	}
}

func TestAnImportCannotStageWhileTheInstanceCannotOpenItsStoredKey(t *testing.T) {
	h := newHarness(t).backed().withoutAnEncryptionKey()

	h.offering(newStaticSource(t))

	err := h.runner.RunStage(context.Background(), stagePayload(h))

	if !errors.Is(err, entity.ErrImportEncryptionKeyMissing) {
		t.Fatalf("run stage returned %v, want the missing encryption key", err)
	}

	if h.run().Status != entity.ImportFailed {
		t.Fatalf(
			"an instance that cannot open its own stored key left the run at %q. Retrying changes "+
				"nothing until an operator configures a key, and a run stuck in staging tells nobody "+
				"that is what has to happen.",
			h.run().Status,
		)
	}

	if !strings.Contains(h.run().PhaseError, "encryption key") {
		t.Errorf(
			"the run recorded %q. The phase error is all whoever started the import is shown, so a "+
				"missing instance key has to be named there rather than left as a worker log line.",
			h.run().PhaseError,
		)
	}

	if len(h.world.records) != 0 {
		t.Errorf("staged %d records without ever opening the source key", len(h.world.records))
	}
}

func TestASourceThatCannotBeAskedWhatItHoldsIsNotAnError(t *testing.T) {
	h := newHarness(t).backed().allow(entity.Decision{})

	h.offering(newStaticSource(t))

	catalogue, err := h.imports.Catalogue(context.Background(), h.run().WorkspaceID, h.run().ID)
	if err != nil {
		t.Fatalf(
			"asking a source that answers no such question returned %v. A CSV has a header and a "+
				"tracker has teams, but a source with neither is a perfectly ordinary import — the "+
				"step is skipped rather than failed.",
			err,
		)
	}

	if len(catalogue.Scopes) != 0 || len(catalogue.Columns) != 0 || len(catalogue.Notes) != 0 {
		t.Errorf("a source that cannot be probed answered with %+v, want nothing at all", catalogue)
	}

	h.wroteNothing("asking a source what it holds")
}

func TestASourceRefusingToSayWhatItHoldsIsCarriedBackToTheCaller(t *testing.T) {
	h := newHarness(t).backed().allow(entity.Decision{})

	h.configured(sourceKey, json.RawMessage(sourceSettings))

	source := &probingSource{
		staticSource: newStaticSource(t),
		refusal: entity.ImportSourceRefusedError{
			Resource: entity.ImportTeam,
			Reason:   "the token cannot list teams",
		},
	}

	h.offering(source)

	_, err := h.imports.Catalogue(context.Background(), h.run().WorkspaceID, h.run().ID)

	var refused entity.ImportSourceRefusedError

	if !errors.As(err, &refused) {
		t.Fatalf(
			"asking the source returned %v, want its own refusal. A key that cannot list teams "+
				"cannot import them either, and saying so here is what stops somebody discovering "+
				"it after a staging run they have to unpick.",
			err,
		)
	}

	if len(source.asked) != 1 || source.asked[0].Secret != sourceKey {
		t.Errorf("the source was asked with %+v, want exactly one question carrying the run's key", source.asked)
	}
}

func TestASourceIsAskedWhatItHoldsWithTheConfigurationTheRunWasGiven(t *testing.T) {
	h := newHarness(t).backed().allow(entity.Decision{})

	h.configured(sourceKey, json.RawMessage(sourceSettings))

	source := &probingSource{
		staticSource: newStaticSource(t),
		catalogue: entity.ImportCatalogue{
			Scopes: []entity.ImportScope{{Key: sourceTeam, Name: "Core", Detail: "412 issues"}},
			Notes:  []string{"Archived issues are not carried."},
		},
	}

	h.offering(source)

	catalogue, err := h.imports.Catalogue(context.Background(), h.run().WorkspaceID, h.run().ID)
	if err != nil {
		t.Fatalf("catalogue: %v", err)
	}

	if len(catalogue.Scopes) != 1 || catalogue.Scopes[0].Key != sourceTeam {
		t.Errorf("the catalogue read back as %+v, want the source's own scopes", catalogue)
	}

	if len(source.asked) != 1 {
		t.Fatalf("the source was asked %d times, want once", len(source.asked))
	}

	if string(source.asked[0].Settings) != sourceSettings {
		t.Errorf(
			"the source was asked with the settings %q rather than %q; a probe answers for the "+
				"configuration being assembled, so asking with anything else describes a different run",
			source.asked[0].Settings, sourceSettings,
		)
	}

	h.wroteNothing("asking a source what it holds")
}
