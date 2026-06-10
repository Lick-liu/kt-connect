package transmission

import "testing"

func TestShouldExitAfterReconnectFailure(t *testing.T) {
	tests := []struct {
		name     string
		failures int
		want     bool
	}{
		{name: "below threshold keeps retrying", failures: maxTunnelReconnectFailures - 1, want: false},
		{name: "at threshold exits", failures: maxTunnelReconnectFailures, want: true},
		{name: "above threshold exits", failures: maxTunnelReconnectFailures + 1, want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shouldExitAfterReconnectFailure(tt.failures); got != tt.want {
				t.Fatalf("shouldExitAfterReconnectFailure(%d) = %v, want %v", tt.failures, got, tt.want)
			}
		})
	}
}

func TestNextReconnectFailures(t *testing.T) {
	tests := []struct {
		name            string
		currentFailures int
		stable          bool
		want            int
	}{
		{name: "short failure increments", currentFailures: 1, stable: false, want: 2},
		{name: "stable tunnel resets failures", currentFailures: 2, stable: true, want: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := nextReconnectFailures(tt.currentFailures, tt.stable); got != tt.want {
				t.Fatalf("nextReconnectFailures(%d, %v) = %d, want %d", tt.currentFailures, tt.stable, got, tt.want)
			}
		})
	}
}
