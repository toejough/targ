package core

import (
	"bytes"
	"context"
	"sync"
	"sync/atomic"
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

func TestRunGroupParallelAllBoundsConcurrency(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	var (
		buf     bytes.Buffer
		mu      sync.Mutex
		cur     atomic.Int32
		maxSeen atomic.Int32
		ran     atomic.Int32
	)

	makeTarget := func(name string) *Target {
		return Targ(func() {
			c := cur.Add(1)

			mu.Lock()
			if c > maxSeen.Load() {
				maxSeen.Store(c)
			}
			mu.Unlock()

			ran.Add(1)
			cur.Add(-1)
		}).Name(name)
	}

	targets := []*Target{
		makeTarget("t1"), makeTarget("t2"), makeTarget("t3"), makeTarget("t4"), makeTarget("t5"),
	}
	ctx := WithExecInfo(context.Background(), ExecInfo{Output: &buf})

	err := runGroupParallelAll(ctx, targets, 1)

	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(ran.Load()).To(Equal(int32(5)), "all targets must run")
	g.Expect(maxSeen.Load()).To(Equal(int32(1)), "cap 1 must serialize execution")
}

func TestRunGroupParallelBoundsConcurrency(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	var (
		buf     bytes.Buffer
		mu      sync.Mutex
		cur     atomic.Int32
		maxSeen atomic.Int32
		ran     atomic.Int32
	)

	makeTarget := func(name string) *Target {
		return Targ(func() {
			c := cur.Add(1)

			mu.Lock()
			if c > maxSeen.Load() {
				maxSeen.Store(c)
			}
			mu.Unlock()

			ran.Add(1)
			cur.Add(-1)
		}).Name(name)
	}

	targets := []*Target{makeTarget("t1"), makeTarget("t2"), makeTarget("t3"), makeTarget("t4")}
	ctx := WithExecInfo(context.Background(), ExecInfo{Output: &buf})

	err := runGroupParallel(ctx, targets, 1)

	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(ran.Load()).To(Equal(int32(4)), "all targets must run")
	g.Expect(maxSeen.Load()).To(Equal(int32(1)), "cap 1 must serialize execution")
}

func TestRunGroupParallelSkipsOnCanceledContext(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	var (
		buf bytes.Buffer
		ran atomic.Int32
	)

	makeTarget := func(name string) *Target {
		return Targ(func() {
			ran.Add(1)
		}).Name(name)
	}

	targets := []*Target{makeTarget("t1"), makeTarget("t2"), makeTarget("t3")}

	ctx, cancel := context.WithCancel(
		WithExecInfo(context.Background(), ExecInfo{Output: &buf}),
	)
	cancel() // canceled before the group starts

	err := runGroupParallel(ctx, targets, 1)

	g.Expect(err).To(HaveOccurred())
	g.Expect(ran.Load()).To(Equal(int32(0)), "no target may start on a canceled context")
}
