package eventstream

import "testing"

func TestAStreamIdentifierOrdersByTimeThenSequence(t *testing.T) {
	for _, probe := range []struct {
		candidate, cursor string
		later             bool
	}{
		{"100-0", "99-0", true},
		{"99-0", "100-0", false},
		{"100-1", "100-0", true},
		{"100-0", "100-1", false},
		{"100-0", "100-0", false},
		{"1785930728804-0", "1785930728803-9", true},
	} {
		if got := after(probe.candidate, probe.cursor); got != probe.later {
			t.Errorf("after(%q, %q) = %v, want %v", probe.candidate, probe.cursor, got, probe.later)
		}
	}
}

func TestOnlyACursorTheStreamHasOutrunCountsAsALapse(t *testing.T) {
	// The oldest surviving entry being newer than the cursor means everything between them was
	// trimmed. The other direction is an ordinary reconnect and must replay, not resync: getting
	// this backwards makes every reconnect look like a lapse and silently drops the replay.
	if !after("200-0", "100-0") {
		t.Fatal("a cursor the stream has outrun was not recognised as a lapse")
	}

	if after("100-0", "200-0") {
		t.Fatal(
			"an ordinary reconnect was treated as a lapse. The client would be told to reload " +
				"every single time it reconnected, and would never receive the replay it asked for.",
		)
	}
}
