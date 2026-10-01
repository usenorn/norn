package channelv1_test

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	channelv1 "github.com/usenorn/norn/pkg/channel/v1"
)

func TestARefusalNamesTheMessageItRefusesAndCarriesItsReasonAcrossTheWire(t *testing.T) {
	sent := channelv1.Refuse("01ASK", "validation failed: options: too_long", time.Now())

	raw, err := json.Marshal(sent)
	if err != nil {
		t.Fatal(err)
	}

	var read channelv1.Envelope
	if err := json.Unmarshal(raw, &read); err != nil {
		t.Fatal(err)
	}

	if !read.Refusing() || read.Acknowledging() || read.AckID != "01ASK" {
		t.Fatalf("the refusal came across as %+v", read)
	}

	if read.Reason() != "validation failed: options: too_long" {
		t.Fatalf("the reason came across as %q", read.Reason())
	}
}

func TestARefusalReasonIsCutBackRatherThanGrowingTheFrame(t *testing.T) {
	reason := channelv1.Refuse("01ASK", strings.Repeat("x", channelv1.RefusalReasonMax+50), time.Now()).Reason()

	if len([]rune(reason)) != channelv1.RefusalReasonMax {
		t.Fatalf("a long reason went out at %d runes", len([]rune(reason)))
	}
}
