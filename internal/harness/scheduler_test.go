package harness

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/allofher/agent-kate-reid/internal/strudel"
)

// fakeApplier records applied ops and returns canned state.
type fakeApplier struct {
	mu    sync.Mutex
	calls []string
}

func (f *fakeApplier) record(call string) (strudel.State, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, call)
	return strudel.State{Playing: true}, nil
}

func (f *fakeApplier) EvalPattern(_ context.Context, ch, _ string) (strudel.State, error) {
	return f.record("eval:" + ch)
}
func (f *fakeApplier) StopChannel(_ context.Context, ch string) (strudel.State, error) {
	return f.record("stop:" + ch)
}
func (f *fakeApplier) StopAll(context.Context) (strudel.State, error) {
	return f.record("stop_all")
}
func (f *fakeApplier) SetCPS(_ context.Context, _ float64) (strudel.State, error) {
	return f.record("set_cps")
}
func (f *fakeApplier) GetState(context.Context) (strudel.State, error) {
	return f.record("get_state")
}

func TestSchedulerAppliesQueueOnTick(t *testing.T) {
	fake := &fakeApplier{}
	queue := NewQueue()
	s := NewScheduler(fake, queue, Config{
		Mode:     TickWallclock,
		Interval: 10 * time.Millisecond,
	})

	queue.Enqueue(Op{Kind: OpSetCPS, CPS: 0.75})
	queue.Enqueue(Op{Kind: OpEval, Channel: "bass", Code: `note("c2")`})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go s.Run(ctx)

	select {
	case report := <-s.Reports():
		if len(report.Applied) != 2 {
			t.Fatalf("expected 2 applied ops, got %d", len(report.Applied))
		}
		if report.Applied[0].Op.Kind != OpSetCPS || report.Applied[1].Op.Kind != OpEval {
			t.Errorf("unexpected apply order: %+v", report.Applied)
		}
		if !report.State.Playing {
			t.Errorf("expected post-apply state from GetState")
		}
	case <-time.After(time.Second):
		t.Fatal("no tick report within 1s")
	}
}

func TestSchedulerEmptyTickStillReports(t *testing.T) {
	fake := &fakeApplier{}
	s := NewScheduler(fake, NewQueue(), Config{
		Mode:     TickWallclock,
		Interval: 10 * time.Millisecond,
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go s.Run(ctx)

	select {
	case report := <-s.Reports():
		if len(report.Applied) != 0 {
			t.Errorf("expected no applied ops, got %+v", report.Applied)
		}
	case <-time.After(time.Second):
		t.Fatal("no tick report within 1s")
	}
}

func TestSchedulerCyclesPeriodFollowsCPS(t *testing.T) {
	s := NewScheduler(&fakeApplier{}, NewQueue(), Config{
		Mode:       TickCycles,
		Cycles:     4,
		DefaultCPS: 0.5,
	})
	if got, want := s.period(), 8*time.Second; got != want {
		t.Errorf("period at 0.5 cps = %v, want %v", got, want)
	}
	s.cps = 1.0
	if got, want := s.period(), 4*time.Second; got != want {
		t.Errorf("period at 1.0 cps = %v, want %v", got, want)
	}
}
