package internal

import (
	"testing"
)

func TestCompareBranchVersion(t *testing.T) {
	tests := []struct {
		branch1 string
		branch2 string
		want    int
	}{
		{"release/1.0.0", "release/1.0.1", -1},
		{"release/1.0.1", "release/1.0.0", 1},
		{"release/1.0.0", "release/1.0.0", 0},
		{"release/1.0.0", "release/1.0", 0},
		{"release/1.0", "release/1.0.0", 0},
		{"release/1.0.0", "release/2.0.0", -1},
		{"release/2.0.0", "release/1.0.0", 1},
		{"release/1.0.0-alpha", "release/1.0.0-beta", -1},
		{"release/1.0.0-beta", "release/1.0.0-alpha", 1},
		{"release/1.0.0-alpha", "release/1.0.0-alpha", 0},
	}

	for _, tt := range tests {
		t.Run(tt.branch1+"_"+tt.branch2, func(t *testing.T) {
			if got := compareBranchVersion(tt.branch1, tt.branch2); got != tt.want {
				t.Errorf("compareBranchVersion(%s, %s) = %d, want %d", tt.branch1, tt.branch2, got, tt.want)
			}
		})
	}
}
