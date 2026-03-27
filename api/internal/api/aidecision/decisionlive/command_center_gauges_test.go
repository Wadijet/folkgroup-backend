package decisionlive

import "testing"

func TestAdjustGaugePublish_chuyenPhase(t *testing.T) {
	org := "aaaaaaaaaaaaaaaaaaaaaaaa"
	tr := "trace-g-1"
	adjustGaugeOnLivePublish(org, tr, PhaseQueued)
	g := gaugeSnapshotForOrg(org)
	if g[PhaseQueued] != 1 || g[PhaseConsuming] != 0 {
		t.Fatalf("sau queued: %+v", g)
	}
	adjustGaugeOnLivePublish(org, tr, PhaseConsuming)
	g = gaugeSnapshotForOrg(org)
	if g[PhaseQueued] != 0 || g[PhaseConsuming] != 1 {
		t.Fatalf("sau consuming: %+v", g)
	}
	adjustGaugeOnLivePublish(org, tr, PhaseDone)
	g = gaugeSnapshotForOrg(org)
	if g[PhaseConsuming] != 0 || g[PhaseDone] != 0 {
		t.Fatalf("sau terminal done gauge phase phải 0: %+v", g)
	}
}

func TestRecordConsumerWorkEnd_thaTraceTreo(t *testing.T) {
	org := "bbbbbbbbbbbbbbbbbbbbbbbb"
	tr := "trace-g-2"
	recordConsumerWorkBeginGauge(org, tr)
	adjustGaugeOnLivePublish(org, tr, PhaseQueued)
	recordConsumerWorkEndGauge(org, tr)
	g := gaugeSnapshotForOrg(org)
	if g[GaugeKeyWorkerHeld] != 0 {
		t.Fatalf("worker_held phải 0: %+v", g)
	}
	if g[PhaseQueued] != 0 {
		t.Fatalf("queued treo phải được thả: %+v", g)
	}
}

func TestRecordConsumerWorkBeginEnd_khongTrace(t *testing.T) {
	org := "cccccccccccccccccccccccc"
	recordConsumerWorkBeginGauge(org, "")
	recordConsumerWorkEndGauge(org, "")
	g := gaugeSnapshotForOrg(org)
	if g[GaugeKeyWorkerHeld] != 0 || g[PhaseConsuming] != 0 {
		t.Fatalf("không trace: sau 1 vòng begin/end gauge sạch: %+v", g)
	}
}
