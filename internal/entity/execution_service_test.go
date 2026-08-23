package entity_test

import (
	"errors"
	"testing"
	"time"

	"github.com/usenorn/norn/internal/entity"
)

func TestOnlyTheFourStatesAServiceCanBeInAreValid(t *testing.T) {
	cases := map[entity.ExecutionServiceState]bool{
		entity.ExecutionServiceStarting:  true,
		entity.ExecutionServiceHealthy:   true,
		entity.ExecutionServiceUnhealthy: true,
		entity.ExecutionServiceStopped:   true,
		"":                               false,
		"flapping":                       false,
		"running":                        false,
	}

	for state, wanted := range cases {
		if state.Valid() != wanted {
			t.Errorf("%q reported valid=%t, wanted %t", state, state.Valid(), wanted)
		}
	}
}

func TestOnlyAStartingOrHealthyServiceCountsAsLive(t *testing.T) {
	cases := map[entity.ExecutionServiceState]bool{
		entity.ExecutionServiceStarting:  true,
		entity.ExecutionServiceHealthy:   true,
		entity.ExecutionServiceUnhealthy: false,
		entity.ExecutionServiceStopped:   false,
	}

	for state, wanted := range cases {
		running := entity.ExecutionService{State: state}

		if running.Live() != wanted {
			t.Errorf(
				"a %s service reported live=%t, wanted %t. An unhealthy service still has a "+
					"process, but nothing should be sent to it",
				state, running.Live(), wanted,
			)
		}
	}
}

func TestAServiceWithNoProbeIsStillADescribableService(t *testing.T) {
	cases := map[entity.ExecutionServiceProbe]bool{
		entity.ExecutionServiceProbeNone: true,
		entity.ExecutionServiceProbeHTTP: true,
		entity.ExecutionServiceProbeTCP:  true,
		entity.ExecutionServiceProbeLog:  true,
		"ping":                           false,
	}

	for probe, wanted := range cases {
		if probe.Valid() != wanted {
			t.Errorf("%q reported valid=%t, wanted %t", probe, probe.Valid(), wanted)
		}
	}
}

func TestAServiceIsRefusedWhenTheMachineCouldNotHaveNamedItThat(t *testing.T) {
	cases := map[string]bool{
		"api":                                true,
		"web-2":                              true,
		"worker_queue":                       true,
		"":                                   false,
		"API":                                false,
		"has space":                          false,
		"-leading":                           false,
		"waytoolongwaytoolongwaytoolongxxxx": false,
	}

	for name, wanted := range cases {
		running := entity.ExecutionService{
			Name:       name,
			State:      entity.ExecutionServiceHealthy,
			ReportedAt: time.Now().UTC(),
		}

		err := entity.ValidateExecutionService("service", running)

		if wanted && err != nil {
			t.Errorf("%q was refused with %v, and it is a name the supervisor allows", name, err)
		}

		if !wanted && err == nil {
			t.Errorf("%q was accepted, and it is not a name the supervisor would have used", name)
		}
	}
}

func TestAPortOutsideTheRangeAMachineCouldBindIsRefused(t *testing.T) {
	cases := map[int]bool{
		0:     true,
		1:     true,
		4310:  true,
		65535: true,
		-1:    false,
		65536: false,
	}

	for port, wanted := range cases {
		running := entity.ExecutionService{
			Name:       "api",
			State:      entity.ExecutionServiceHealthy,
			Port:       port,
			ReportedAt: time.Now().UTC(),
		}

		err := entity.ValidateExecutionService("service", running)

		if wanted && err != nil {
			t.Errorf("port %d was refused with %v", port, err)
		}

		if !wanted && err == nil {
			t.Errorf("port %d was accepted, and nothing can listen on it", port)
		}
	}
}

func TestAServiceRefusalNamesTheFieldThatWasWrong(t *testing.T) {
	running := entity.ExecutionService{
		Name:       "api",
		State:      "flapping",
		ReportedAt: time.Now().UTC(),
	}

	err := entity.ValidateExecutionService("service", running)

	var invalid entity.ValidationError
	if !errors.As(err, &invalid) {
		t.Fatalf("a bad state came back as %v rather than a validation error", err)
	}

	for _, field := range invalid.Fields {
		if field.Field == "service.state" {
			return
		}
	}

	t.Fatalf("the refusal named %+v, none of which is the state that was wrong", invalid.Fields)
}
