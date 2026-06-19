package harness

import "testing"

func TestQueueLastWriteWinsPerChannel(t *testing.T) {
	q := NewQueue()
	q.Enqueue(Op{Kind: OpEval, Channel: "bass", Code: `note("c2 e2")`})
	q.Enqueue(Op{Kind: OpEval, Channel: "bass", Code: `note("c2 g2")`})

	ops := q.Drain()
	if len(ops) != 1 {
		t.Fatalf("expected 1 op, got %d", len(ops))
	}
	if ops[0].Code != `note("c2 g2")` {
		t.Errorf("expected last write to win, got %q", ops[0].Code)
	}
}

func TestQueueStopReplacesEval(t *testing.T) {
	q := NewQueue()
	q.Enqueue(Op{Kind: OpEval, Channel: "bass", Code: `note("c2")`})
	q.Enqueue(Op{Kind: OpStopChannel, Channel: "bass"})

	ops := q.Drain()
	if len(ops) != 1 || ops[0].Kind != OpStopChannel {
		t.Fatalf("expected single stop_channel op, got %+v", ops)
	}
}

func TestQueueStopAllClearsEverything(t *testing.T) {
	q := NewQueue()
	q.Enqueue(Op{Kind: OpEval, Channel: "bass", Code: `note("c2")`})
	q.Enqueue(Op{Kind: OpSetCPS, CPS: 0.75})
	q.Enqueue(Op{Kind: OpStopAll})

	ops := q.Drain()
	if len(ops) != 1 || ops[0].Kind != OpStopAll {
		t.Fatalf("expected single stop_all op, got %+v", ops)
	}
}

func TestQueueOpsAfterStopAllSurvive(t *testing.T) {
	q := NewQueue()
	q.Enqueue(Op{Kind: OpStopAll})
	q.Enqueue(Op{Kind: OpEval, Channel: "drums", Code: `s("bd sd")`})

	ops := q.Drain()
	if len(ops) != 2 {
		t.Fatalf("expected 2 ops, got %d: %+v", len(ops), ops)
	}
	if ops[0].Kind != OpStopAll || ops[1].Kind != OpEval {
		t.Errorf("expected stop_all then eval, got %+v", ops)
	}
}

func TestQueueDrainOrder(t *testing.T) {
	q := NewQueue()
	q.Enqueue(Op{Kind: OpEval, Channel: "drums", Code: `s("bd")`})
	q.Enqueue(Op{Kind: OpSetCPS, CPS: 0.6})
	q.Enqueue(Op{Kind: OpEval, Channel: "bass", Code: `note("c2")`})

	ops := q.Drain()
	if len(ops) != 3 {
		t.Fatalf("expected 3 ops, got %d", len(ops))
	}
	// set_cps first, then channel ops in first-enqueue order.
	if ops[0].Kind != OpSetCPS {
		t.Errorf("expected set_cps first, got %+v", ops[0])
	}
	if ops[1].Channel != "drums" || ops[2].Channel != "bass" {
		t.Errorf("expected channel order drums,bass; got %+v", ops[1:])
	}
}

func TestQueueDrainEmpties(t *testing.T) {
	q := NewQueue()
	q.Enqueue(Op{Kind: OpEval, Channel: "bass", Code: `note("c2")`})
	q.Drain()

	if q.Len() != 0 {
		t.Errorf("expected empty queue after drain, len = %d", q.Len())
	}
	if ops := q.Drain(); ops != nil {
		t.Errorf("expected nil from draining empty queue, got %+v", ops)
	}
}
