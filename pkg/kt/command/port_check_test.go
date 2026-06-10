package command

import "testing"

func TestShouldCheckLocalPorts(t *testing.T) {
	tests := []struct {
		name string
		skip bool
		want bool
	}{
		{name: "default checks local ports", skip: false, want: true},
		{name: "skip flag disables local port check", skip: true, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shouldCheckLocalPorts(tt.skip); got != tt.want {
				t.Fatalf("shouldCheckLocalPorts(%v) = %v, want %v", tt.skip, got, tt.want)
			}
		})
	}
}
