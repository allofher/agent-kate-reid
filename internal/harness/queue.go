// Package harness quantizes Kate's actions: tool calls enqueue operations,
// and a scheduler applies the queue to the Strudel server at deterministic
// intervals so changes land on musical boundaries.
package harness

import "sync"

// OpKind identifies a queued operation.
type OpKind string

const (
	OpEval        OpKind = "eval_pattern"
	OpStopChannel OpKind = "stop_channel"
	OpStopAll     OpKind = "stop_all"
	OpSetCPS      OpKind = "set_cps"
)

// Op is one queued operation awaiting the next tick.
type Op struct {
	Kind    OpKind
	Channel string  // eval_pattern / stop_channel
	Code    string  // eval_pattern
	CPS     float64 // set_cps
}

// Queue collects Kate's pending operations between ticks.
//
// Semantics within a window:
//   - eval_pattern / stop_channel: last write wins per channel
//   - set_cps: last write wins (single slot)
//   - stop_all: clears everything queued before it
type Queue struct {
	mu sync.Mutex

	// channelOps maps channel name → pending op, with order preserving
	// first-enqueue order so drains are deterministic.
	channelOps map[string]Op
	order      []string

	cps     *float64
	stopAll bool
}

// NewQueue returns an empty Queue.
func NewQueue() *Queue {
	return &Queue{channelOps: make(map[string]Op)}
}

// Enqueue adds op to the queue, applying last-write-wins semantics.
func (q *Queue) Enqueue(op Op) {
	q.mu.Lock()
	defer q.mu.Unlock()

	switch op.Kind {
	case OpStopAll:
		// stop_all supersedes everything queued before it.
		q.channelOps = make(map[string]Op)
		q.order = nil
		q.cps = nil
		q.stopAll = true

	case OpSetCPS:
		cps := op.CPS
		q.cps = &cps

	case OpEval, OpStopChannel:
		if _, seen := q.channelOps[op.Channel]; !seen {
			q.order = append(q.order, op.Channel)
		}
		q.channelOps[op.Channel] = op
	}
}

// Drain removes and returns all pending operations in application order:
// stop_all first (if queued), then set_cps, then channel ops in first-enqueue
// order. Returns nil when nothing is pending.
func (q *Queue) Drain() []Op {
	q.mu.Lock()
	defer q.mu.Unlock()

	var ops []Op
	if q.stopAll {
		ops = append(ops, Op{Kind: OpStopAll})
	}
	if q.cps != nil {
		ops = append(ops, Op{Kind: OpSetCPS, CPS: *q.cps})
	}
	for _, ch := range q.order {
		ops = append(ops, q.channelOps[ch])
	}

	q.channelOps = make(map[string]Op)
	q.order = nil
	q.cps = nil
	q.stopAll = false
	return ops
}

// Len reports how many operations are pending.
func (q *Queue) Len() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	n := len(q.channelOps)
	if q.cps != nil {
		n++
	}
	if q.stopAll {
		n++
	}
	return n
}
