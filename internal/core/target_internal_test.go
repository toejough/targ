package core

import (
	"testing"

	. "github.com/onsi/gomega"
)

func TestParallelCap(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		n     int
		procs int
		want  int
	}{
		{name: "TenProcsEightTargets", n: 8, procs: 10, want: 5},
		{name: "FourProcsFloorsAtTwo", n: 8, procs: 4, want: 2},
		{name: "TwoProcsFloorsAtTwo", n: 8, procs: 2, want: 2},
		{name: "OneProcFloorsAtTwo", n: 8, procs: 1, want: 2},
		{name: "SingleTargetCapsAtOne", n: 1, procs: 10, want: 1},
		{name: "ZeroTargetsCapsAtZero", n: 0, procs: 10, want: 0},
		{name: "ManyProcsCapsAtN", n: 8, procs: 32, want: 8},
		{name: "OddProcsIntegerDivision", n: 3, procs: 7, want: 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			g.Expect(parallelCap(tt.n, tt.procs)).To(Equal(tt.want))
		})
	}
}
