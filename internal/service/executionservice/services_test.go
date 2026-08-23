package executionservice_test

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"testing"

	"go.uber.org/mock/gomock"

	"github.com/usenorn/norn/internal/entity"
	channelv1 "github.com/usenorn/norn/pkg/channel/v1"
)

func TestWhatAMachineIsRunningIsRecordedWithItsPortAndItsProbe(t *testing.T) {
	h := newHarness(t)
	h.holding()

	message := h.message(channelv1.Service{
		Name:     "api",
		State:    channelv1.ServiceHealthy,
		Probe:    channelv1.ProbeLog,
		Port:     4310,
		Reason:   "it wrote a line matching listening on",
		Occurred: at(0),
	})

	if err := h.service.Reported(context.Background(), h.runner, message); err != nil {
		t.Fatalf("record what a run is running: %v", err)
	}

	api, ok := h.known("api")
	if !ok {
		t.Fatal("the service was not recorded, so the run's services panel has nothing to show")
	}

	if api.State != entity.ExecutionServiceHealthy || api.Probe != entity.ExecutionServiceProbeLog ||
		api.Port != 4310 {
		t.Fatalf(
			"the service came back as %s on port %d checked by %q, which is not what the machine "+
				"reported",
			api.State, api.Port, api.Probe,
		)
	}
}

func TestAServiceTransitionAlsoLandsOnTheRunsTimeline(t *testing.T) {
	h := newHarness(t)
	h.holding()

	message := h.message(channelv1.Service{
		Name:     "api",
		State:    channelv1.ServiceUnhealthy,
		Port:     4310,
		Reason:   "it stopped on its own with exit code 9",
		Occurred: at(0),
	})

	if err := h.service.Reported(context.Background(), h.runner, message); err != nil {
		t.Fatalf("record what a run is running: %v", err)
	}

	if len(h.timeline) != 1 {
		t.Fatalf(
			"a service changing state put %d lines on the timeline, so somebody watching the run "+
				"would not see it happen",
			len(h.timeline),
		)
	}

	entry := h.timeline[0]

	if entry.Kind != entity.ExecutionEventService {
		t.Fatalf("the timeline line came back as %q rather than a service line", entry.Kind)
	}

	if entry.Reason == "" || !containsAll(entry.Reason, "api", "unhealthy", "exit code 9") {
		t.Fatalf(
			"the timeline reads %q, which does not say which service, how it is, or why",
			entry.Reason,
		)
	}

	if len(h.published) != 1 || h.published[0].Kind != entity.EventExecutionEvent {
		t.Fatal(
			"nothing went out on the event stream, so an open execution screen would not move " +
				"until somebody reloaded it",
		)
	}
}

func TestALaterReportMovesTheServiceAndKeepsThePortItAlreadyHeld(t *testing.T) {
	h := newHarness(t)
	h.holding()

	starting := h.message(channelv1.Service{
		Name:     "api",
		State:    channelv1.ServiceStarting,
		Port:     4310,
		Occurred: at(0),
	})
	if err := h.service.Reported(context.Background(), h.runner, starting); err != nil {
		t.Fatalf("record the first report: %v", err)
	}

	healthy := h.message(channelv1.Service{
		Name:     "api",
		State:    channelv1.ServiceHealthy,
		Occurred: at(30),
	})
	if err := h.service.Reported(context.Background(), h.runner, healthy); err != nil {
		t.Fatalf("record the later report: %v", err)
	}

	api, _ := h.known("api")

	if api.State != entity.ExecutionServiceHealthy {
		t.Fatalf("the service is still %s after the machine said it came good", api.State)
	}

	if api.Port != 4310 {
		t.Fatalf(
			"the port came back as %d. A port is held by the run rather than by the process, so a "+
				"report that does not name one must not take it away from anything already told "+
				"where to find it",
			api.Port,
		)
	}
}

func TestAnOlderReportDoesNotWalkAServiceBackwards(t *testing.T) {
	h := newHarness(t)
	h.holding()

	healthy := h.message(channelv1.Service{
		Name:     "api",
		State:    channelv1.ServiceHealthy,
		Port:     4310,
		Occurred: at(30),
	})
	if err := h.service.Reported(context.Background(), h.runner, healthy); err != nil {
		t.Fatalf("record the current report: %v", err)
	}

	stale := h.message(channelv1.Service{
		Name:     "api",
		State:    channelv1.ServiceStarting,
		Occurred: at(0),
	})
	if err := h.service.Reported(context.Background(), h.runner, stale); err != nil {
		t.Fatalf("a report that arrived late should be dropped rather than fail: %v", err)
	}

	api, _ := h.known("api")

	if api.State != entity.ExecutionServiceHealthy {
		t.Fatalf(
			"a report from before the one on record put the service back to %s. Messages can "+
				"arrive out of order after a reconnect, so the panel would show a healthy service "+
				"as still starting",
			api.State,
		)
	}

	if len(h.timeline) != 1 {
		t.Fatalf(
			"the dropped report still put a line on the timeline, so the run would read as though " +
				"the service had restarted",
		)
	}
}

func TestTheSameReportArrivingTwiceIsRecordedOnce(t *testing.T) {
	h := newHarness(t)
	h.holding()

	payload := channelv1.Service{
		Name:     "api",
		State:    channelv1.ServiceHealthy,
		Port:     4310,
		Occurred: at(0),
	}

	first := h.messageWithID("msg-1", payload)
	if err := h.service.Reported(context.Background(), h.runner, first); err != nil {
		t.Fatalf("record the report: %v", err)
	}

	replay := h.messageWithID("msg-1", payload)
	if err := h.service.Reported(context.Background(), h.runner, replay); err != nil {
		t.Fatalf("a replayed report should settle rather than fail: %v", err)
	}

	if len(h.timeline) != 1 {
		t.Fatalf(
			"a message replayed after a reconnect put %d lines on the timeline. The spool replays "+
				"whatever was not acknowledged, so the same news must not read as it happening "+
				"twice",
			len(h.timeline),
		)
	}
}

func TestARunMayNotNameMoreServicesThanItIsAllowed(t *testing.T) {
	h := newHarness(t)
	h.holding()

	for index := range entity.ExecutionServicesMax {
		message := h.message(channelv1.Service{
			Name:     "svc" + strconv.Itoa(index),
			State:    channelv1.ServiceStarting,
			Occurred: at(0),
		})

		if err := h.service.Reported(context.Background(), h.runner, message); err != nil {
			t.Fatalf("record service %d: %v", index, err)
		}
	}

	crowded := h.message(channelv1.Service{
		Name:     "one-too-many",
		State:    channelv1.ServiceStarting,
		Occurred: at(0),
	})

	err := h.service.Reported(context.Background(), h.runner, crowded)
	if !errors.Is(err, entity.ErrExecutionServiceCrowded) {
		t.Fatalf(
			"a run naming more services than it may was answered with %v. A machine looping on "+
				"start would otherwise fill the table one row at a time",
			err,
		)
	}
}

func TestOneAlreadyOnRecordStillReportsWhenTheRunIsFull(t *testing.T) {
	h := newHarness(t)
	h.holding()

	for index := range entity.ExecutionServicesMax {
		message := h.message(channelv1.Service{
			Name:     "svc" + strconv.Itoa(index),
			State:    channelv1.ServiceStarting,
			Occurred: at(0),
		})

		if err := h.service.Reported(context.Background(), h.runner, message); err != nil {
			t.Fatalf("record service %d: %v", index, err)
		}
	}

	later := h.message(channelv1.Service{
		Name:     "svc0",
		State:    channelv1.ServiceHealthy,
		Occurred: at(30),
	})

	if err := h.service.Reported(context.Background(), h.runner, later); err != nil {
		t.Fatalf(
			"a service already on record could not report once the run was full: %v. The cap is "+
				"on how many services a run has, not on how often they may change state",
			err,
		)
	}

	first, _ := h.known("svc0")
	if first.State != entity.ExecutionServiceHealthy {
		t.Fatalf("svc0 is still %s after it said it came good", first.State)
	}
}

func TestAServiceNameTheMachineCouldNotHaveUsedIsRefused(t *testing.T) {
	h := newHarness(t)
	h.holding()

	message := h.message(channelv1.Service{
		Name:     "Not A Name",
		State:    channelv1.ServiceStarting,
		Occurred: at(0),
	})

	if err := h.service.Reported(context.Background(), h.runner, message); err == nil {
		t.Fatal(
			"a service name the supervisor itself would refuse was accepted, so the panel would " +
				"show a row no machine could have started",
		)
	}
}

func TestAStateThatIsNotOneOfTheFourIsRefused(t *testing.T) {
	h := newHarness(t)
	h.holding()

	message := h.message(channelv1.Service{
		Name:     "api",
		State:    "flapping",
		Occurred: at(0),
	})

	if err := h.service.Reported(context.Background(), h.runner, message); err == nil {
		t.Fatal("a state outside the four a service can be in was stored rather than refused")
	}
}

func TestTheServicesOfARunAreOnlyReadableBySomebodyWhoMaySeeTheRun(t *testing.T) {
	h := newHarness(t)

	refused := errors.New("not yours")

	h.runs.EXPECT().
		Visible(gomock.Any(), h.workspaceID, h.execution.ID).
		Return(entity.Execution{}, refused)

	if _, err := h.service.ForExecution(
		context.Background(), h.workspaceID, h.execution.ID,
	); !errors.Is(err, refused) {
		t.Fatalf(
			"reading the services of a run answered %v rather than the refusal reading the run "+
				"itself gave",
			err,
		)
	}
}

func TestTheServicesOfARunComeBackForSomebodyWhoMaySeeIt(t *testing.T) {
	h := newHarness(t)
	h.holding()
	h.visible()

	message := h.message(channelv1.Service{
		Name:     "api",
		State:    channelv1.ServiceHealthy,
		Port:     4310,
		Occurred: at(0),
	})
	if err := h.service.Reported(context.Background(), h.runner, message); err != nil {
		t.Fatalf("record the report: %v", err)
	}

	running, err := h.service.ForExecution(context.Background(), h.workspaceID, h.execution.ID)
	if err != nil {
		t.Fatalf("read the services of a run: %v", err)
	}

	if len(running) != 1 || running[0].Name != "api" {
		t.Fatalf("the run came back with %d services rather than the one it is running", len(running))
	}
}

func containsAll(text string, wanted ...string) bool {
	for _, want := range wanted {
		if !strings.Contains(text, want) {
			return false
		}
	}

	return true
}
