package ptrace

import (
	"testing"

	exptrace "golang.org/x/exp/trace"
)

func TestSpliceRanges(t *testing.T) {
	// Helper to create a simple active span.
	active := func(start, end int) Span {
		return Span{
			Start:      exptrace.Time(start),
			End:        exptrace.Time(end),
			StartEvent: 1,
			EndEvent:   2,
			State:      StateActive,
			Kind:       SpanKindStateTransition,
		}
	}

	// Helper to create a range span (from the Ranges map).
	rng := func(start, end int) Span {
		return Span{
			Start:      exptrace.Time(start),
			End:        exptrace.Time(end),
			StartEvent: 10,
			EndEvent:   11,
		}
	}

	// Helper to create a non-active span.
	blocked := func(start, end int) Span {
		return Span{
			Start:      exptrace.Time(start),
			End:        exptrace.Time(end),
			StartEvent: 1,
			EndEvent:   2,
			State:      StateBlockedSend,
			Kind:       SpanKindStateTransition,
		}
	}

	tests := []struct {
		name      string
		spans     []Span
		ranges    []Span
		state     SchedulingState
		wantCount int
		check     func(t *testing.T, result []Span)
	}{
		{
			name:      "empty ranges",
			spans:     []Span{active(0, 100)},
			ranges:    nil,
			state:     StateGCMarkAssist,
			wantCount: 1,
		},
		{
			name:      "range fully within single active span",
			spans:     []Span{active(0, 100)},
			ranges:    []Span{rng(20, 60)},
			state:     StateGCMarkAssist,
			wantCount: 3, // prefix + middle + suffix
			check: func(t *testing.T, result []Span) {
				// Prefix [0, 20)
				if result[0].Start != 0 || result[0].End != 20 || result[0].State != StateActive {
					t.Errorf("prefix: got %v", result[0])
				}
				// Middle [20, 60)
				if result[1].Start != 20 || result[1].End != 60 || result[1].State != StateGCMarkAssist {
					t.Errorf("middle: got %v", result[1])
				}
				if result[1].Kind != SpanKindCustom {
					t.Errorf("middle kind: got %v, want SpanKindCustom", result[1].Kind)
				}
				if result[1].StartEvent != 10 || result[1].EndEvent != 11 {
					t.Errorf("middle events: got start=%d end=%d, want 10/11", result[1].StartEvent, result[1].EndEvent)
				}
				// Suffix [60, 100)
				if result[2].Start != 60 || result[2].End != 100 || result[2].State != StateActive {
					t.Errorf("suffix: got %v", result[2])
				}
			},
		},
		{
			name:      "range exactly matching span",
			spans:     []Span{active(0, 100)},
			ranges:    []Span{rng(0, 100)},
			state:     StateGCSweep,
			wantCount: 1,
			check: func(t *testing.T, result []Span) {
				if result[0].State != StateGCSweep {
					t.Errorf("got state %v, want StateGCSweep", result[0].State)
				}
			},
		},
		{
			name:      "range starting at span start (no prefix)",
			spans:     []Span{active(0, 100)},
			ranges:    []Span{rng(0, 60)},
			state:     StateGCMarkAssist,
			wantCount: 2, // middle + suffix
			check: func(t *testing.T, result []Span) {
				if result[0].State != StateGCMarkAssist || result[0].Start != 0 || result[0].End != 60 {
					t.Errorf("middle: got %v", result[0])
				}
				if result[1].State != StateActive || result[1].Start != 60 || result[1].End != 100 {
					t.Errorf("suffix: got %v", result[1])
				}
			},
		},
		{
			name:      "range ending at span end (no suffix)",
			spans:     []Span{active(0, 100)},
			ranges:    []Span{rng(40, 100)},
			state:     StateGCMarkAssist,
			wantCount: 2, // prefix + middle
			check: func(t *testing.T, result []Span) {
				if result[0].State != StateActive || result[0].Start != 0 || result[0].End != 40 {
					t.Errorf("prefix: got %v", result[0])
				}
				if result[1].State != StateGCMarkAssist || result[1].Start != 40 || result[1].End != 100 {
					t.Errorf("middle: got %v", result[1])
				}
			},
		},
		{
			name: "range spanning multiple consecutive active spans",
			spans: []Span{
				active(0, 50),
				blocked(50, 80),
				active(80, 150),
			},
			ranges:    []Span{rng(20, 120)},
			state:     StateGCMarkAssist,
			wantCount: 5, // prefix + middle(from span0) + blocked + prefix(from span2 clamped) + suffix
			check: func(t *testing.T, result []Span) {
				// [0,20) active prefix
				if result[0].State != StateActive || result[0].Start != 0 || result[0].End != 20 {
					t.Errorf("result[0]: got start=%d end=%d state=%v", result[0].Start, result[0].End, result[0].State)
				}
				// [20,50) GC mark assist (clamped to span end)
				if result[1].State != StateGCMarkAssist || result[1].Start != 20 || result[1].End != 50 {
					t.Errorf("result[1]: got start=%d end=%d state=%v", result[1].Start, result[1].End, result[1].State)
				}
				// [50,80) blocked, reclassified to StateBlockedGC because it overlaps the range
				if result[2].State != StateBlockedGC || result[2].Start != 50 || result[2].End != 80 {
					t.Errorf("result[2]: got start=%d end=%d state=%v", result[2].Start, result[2].End, result[2].State)
				}
				// [80,120) GC mark assist (continues from range)
				if result[3].State != StateGCMarkAssist || result[3].Start != 80 || result[3].End != 120 {
					t.Errorf("result[3]: got start=%d end=%d state=%v", result[3].Start, result[3].End, result[3].State)
				}
				// [120,150) active suffix
				if result[4].State != StateActive || result[4].Start != 120 || result[4].End != 150 {
					t.Errorf("result[4]: got start=%d end=%d state=%v", result[4].Start, result[4].End, result[4].State)
				}
			},
		},
		{
			name:      "range overlapping blocked span reclassifies to BlockedGC",
			spans:     []Span{blocked(0, 100)},
			ranges:    []Span{rng(20, 60)},
			state:     StateGCMarkAssist,
			wantCount: 1,
			check: func(t *testing.T, result []Span) {
				if result[0].State != StateBlockedGC {
					t.Errorf("expected StateBlockedGC, got %v", result[0].State)
				}
			},
		},
		{
			name:  "multiple ranges within same span",
			spans: []Span{active(0, 100)},
			ranges: []Span{
				rng(10, 30),
				rng(50, 70),
			},
			state:     StateGCSweep,
			wantCount: 5, // prefix + range1 + gap + range2 + suffix
			check: func(t *testing.T, result []Span) {
				// [0,10) active
				if result[0].State != StateActive || result[0].Start != 0 || result[0].End != 10 {
					t.Errorf("result[0]: got %v", result[0])
				}
				// [10,30) sweep
				if result[1].State != StateGCSweep || result[1].Start != 10 || result[1].End != 30 {
					t.Errorf("result[1]: got %v", result[1])
				}
				// [30,50) active
				if result[2].State != StateActive || result[2].Start != 30 || result[2].End != 50 {
					t.Errorf("result[2]: got %v", result[2])
				}
				// [50,70) sweep
				if result[3].State != StateGCSweep || result[3].Start != 50 || result[3].End != 70 {
					t.Errorf("result[3]: got %v", result[3])
				}
				// [70,100) active
				if result[4].State != StateActive || result[4].Start != 70 || result[4].End != 100 {
					t.Errorf("result[4]: got %v", result[4])
				}
			},
		},
		{
			name: "range overlapping non-blocked non-active span leaves it unchanged",
			spans: []Span{
				{
					Start:      exptrace.Time(0),
					End:        exptrace.Time(100),
					StartEvent: 1,
					EndEvent:   2,
					State:      StateReady,
					Kind:       SpanKindStateTransition,
				},
			},
			ranges:    []Span{rng(20, 60)},
			state:     StateGCMarkAssist,
			wantCount: 1,
			check: func(t *testing.T, result []Span) {
				if result[0].State != StateReady {
					t.Errorf("expected StateReady unchanged, got %v", result[0].State)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := spliceRanges(tt.spans, tt.ranges, tt.state)
			if len(result) != tt.wantCount {
				t.Fatalf("got %d spans, want %d. Spans: %v", len(result), tt.wantCount, result)
			}
			if tt.check != nil {
				tt.check(t, result)
			}
		})
	}
}
