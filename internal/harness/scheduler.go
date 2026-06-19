package harness

import (
	"context"
	"fmt"
	"time"

	"github.com/allofher/agent-kate-reid/internal/strudel"
)

// TickMode selects how the scheduler spaces its ticks.
type TickMode string

const (
	// TickCycles aligns ticks to Strudel cycle boundaries (every N cycles,
	// period derived from the current cps).
	TickCycles TickMode = "cycles"
	// TickWallclock ticks at a fixed wall-clock interval regardless of tempo.
	TickWallclock TickMode = "wallclock"
)

// OpResult records the outcome of applying one queued operation.
type OpResult struct {
	Op    Op
	State strudel.State
	Err   error
}

// TickReport is delivered to the agent loop after every tick.
type TickReport struct {
	At      time.Time
	Applied []OpResult    // empty when nothing was queued
	State   strudel.State // post-apply session state
}

// Config holds the scheduler's tick parameters.
type Config struct {
	Mode       TickMode
	Cycles     float64       // cycles mode: apply every N cycles
	Interval   time.Duration // wallclock mode: apply every Interval
	DefaultCPS float64       // cycles mode: anchor tempo before Kate sets one
}

// Applier is the subset of the Strudel client the scheduler drives.
// *strudel.Client satisfies it.
type Applier interface {
	EvalPattern(ctx context.Context, channel, code string) (strudel.State, error)
	StopChannel(ctx context.Context, channel string) (strudel.State, error)
	StopAll(ctx context.Context) (strudel.State, error)
	SetCPS(ctx context.Context, cps float64) (strudel.State, error)
	GetState(ctx context.Context) (strudel.State, error)
}

// Scheduler drains the queue at deterministic intervals and applies the
// operations to the Strudel server, emitting a TickReport per tick.
type Scheduler struct {
	client Applier
	queue  *Queue
	cfg    Config

	cps     float64 // current tempo the harness believes is in effect
	reports chan TickReport
}

// NewScheduler returns a Scheduler ready to Run.
func NewScheduler(client Applier, queue *Queue, cfg Config) *Scheduler {
	cps := cfg.DefaultCPS
	if cps <= 0 {
		cps = 0.5 // 120 bpm
	}
	return &Scheduler{
		client:  client,
		queue:   queue,
		cfg:     cfg,
		cps:     cps,
		reports: make(chan TickReport, 1),
	}
}

// Reports returns the channel on which TickReports are delivered. The channel
// has capacity 1; if the agent is mid-turn when two ticks elapse, the older
// unread report is replaced by the newer one (Kate only ever needs the latest
// state).
func (s *Scheduler) Reports() <-chan TickReport {
	return s.reports
}

// period returns the current tick interval.
func (s *Scheduler) period() time.Duration {
	if s.cfg.Mode == TickWallclock {
		return s.cfg.Interval
	}
	// cycles mode: one cycle lasts 1/cps seconds.
	return time.Duration(float64(time.Second) * s.cfg.Cycles / s.cps)
}

// Run ticks until ctx is cancelled. Call it in its own goroutine.
func (s *Scheduler) Run(ctx context.Context) {
	timer := time.NewTimer(s.period())
	defer timer.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
		}

		report := s.tick(ctx)
		if ctx.Err() != nil {
			return
		}

		// Replace any unread report with the fresh one.
		select {
		case s.reports <- report:
		default:
			select {
			case <-s.reports:
			default:
			}
			s.reports <- report
		}

		timer.Reset(s.period())
	}
}

// tick drains the queue, applies each op in order, and reads back the final
// session state.
func (s *Scheduler) tick(ctx context.Context) TickReport {
	report := TickReport{At: time.Now()}

	for _, op := range s.queue.Drain() {
		res := OpResult{Op: op}
		res.State, res.Err = s.apply(ctx, op)
		if op.Kind == OpSetCPS && res.Err == nil {
			s.cps = op.CPS // re-anchor cycle-aligned ticks to the new tempo
		}
		report.Applied = append(report.Applied, res)
	}

	state, err := s.client.GetState(ctx)
	if err != nil {
		// Surface the read failure on the report rather than dropping it.
		msg := fmt.Sprintf("get state: %v", err)
		state.Error = &msg
	}
	report.State = state
	return report
}

func (s *Scheduler) apply(ctx context.Context, op Op) (strudel.State, error) {
	switch op.Kind {
	case OpEval:
		return s.client.EvalPattern(ctx, op.Channel, op.Code)
	case OpStopChannel:
		return s.client.StopChannel(ctx, op.Channel)
	case OpStopAll:
		return s.client.StopAll(ctx)
	case OpSetCPS:
		return s.client.SetCPS(ctx, op.CPS)
	default:
		return strudel.State{}, fmt.Errorf("harness: unknown op kind %q", op.Kind)
	}
}
