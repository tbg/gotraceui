package ptrace

import "testing"

func TestStateForGoWaitingReason(t *testing.T) {
	tests := []struct {
		reason string
		want   SchedulingState
	}{
		{"chan send", StateBlockedSend},
		{"chan receive", StateBlockedRecv},
		{"network", StateBlockedNet},
		{"runtime.GoSched", StateInactive},
		{"runtime.Gosched", StateInactive},
		{"select", StateBlockedSelect},
		{"sleep", StateInactive},
		{"sync", StateBlockedSync},
		{"sync.(*Cond).Wait", StateBlockedCond},
		{"system goroutine wait", StateInactive},
		{"GC mark assist wait for work", StateBlockedGC},
		{"GC background sweeper wait", StateInactive},
		{"preempted", StateWaitingPreempted},
		{"forever", StateStuck},
		{"wait for debug call", StateBlocked},
		{"wait until GC ends", StateBlockedGC},
		{"", StateBlocked},
	}
	for _, tt := range tests {
		name := tt.reason
		if name == "" {
			name = "(empty)"
		}
		t.Run(name, func(t *testing.T) {
			got := stateForGoWaitingReason(tt.reason)
			if got != tt.want {
				t.Errorf("stateForGoWaitingReason(%q) = %v, want %v", tt.reason, got, tt.want)
			}
		})
	}
}
