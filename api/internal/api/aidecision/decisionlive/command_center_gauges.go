package decisionlive

import (
	"strings"
	"sync"
)

// GaugeKeyWorkerHeld — gauge: số event đang trong processEvent (đã lease, chưa RecordConsumerCompletion).
const GaugeKeyWorkerHeld = "worker_held"

// Giới hạn số trace đang theo dõi phase (tránh map phình); evict sẽ hạ gauge tương ứng.
const maxTraceGaugeTracking = 8000

type orgPhaseGaugeBuf struct {
	mu             sync.Mutex
	byPhase        map[string]int64
	traceLastPhase map[string]string // traceID -> phase hiện tại trên gauge
}

var memPhaseGauge sync.Map // orgHex -> *orgPhaseGaugeBuf

func gaugeBufForOrg(orgHex string) *orgPhaseGaugeBuf {
	if orgHex == "" {
		return nil
	}
	v, _ := memPhaseGauge.LoadOrStore(orgHex, &orgPhaseGaugeBuf{
		byPhase:        make(map[string]int64),
		traceLastPhase: make(map[string]string),
	})
	return v.(*orgPhaseGaugeBuf)
}

func gaugeSafeDec(m map[string]int64, k string) {
	if m == nil || k == "" {
		return
	}
	m[k]--
	if m[k] < 0 {
		m[k] = 0
	}
}

func gaugeSafeInc(m map[string]int64, k string) {
	if m == nil || k == "" {
		return
	}
	m[k]++
}

func isGaugeTerminalPhase(p string) bool {
	switch p {
	case PhaseDone, PhaseError, PhaseEmpty, PhaseSkipped:
		return true
	case PhaseQueueDone, PhaseQueueError:
		return true
	default:
		return false
	}
}

// recordConsumerWorkBeginGauge: mọi event sau lease — worker_held++; không trace thì consuming++ (gauge).
func recordConsumerWorkBeginGauge(orgHex, traceID string) {
	buf := gaugeBufForOrg(orgHex)
	if buf == nil {
		return
	}
	buf.mu.Lock()
	defer buf.mu.Unlock()
	if buf.byPhase == nil {
		buf.byPhase = make(map[string]int64)
	}
	gaugeSafeInc(buf.byPhase, GaugeKeyWorkerHeld)
	if strings.TrimSpace(traceID) == "" {
		gaugeSafeInc(buf.byPhase, PhaseConsuming)
	}
}

// recordConsumerWorkEndGauge: worker_held--; không trace thì consuming--.
// Có trace: nếu trace vẫn còn trong map (không có Publish terminal) thì hạ gauge phase đó — tránh treo sau lỗi sớm.
func recordConsumerWorkEndGauge(orgHex, traceID string) {
	buf := gaugeBufForOrg(orgHex)
	if buf == nil {
		return
	}
	tid := strings.TrimSpace(traceID)
	buf.mu.Lock()
	defer buf.mu.Unlock()
	if buf.byPhase == nil {
		buf.byPhase = make(map[string]int64)
	}
	gaugeSafeDec(buf.byPhase, GaugeKeyWorkerHeld)
	if tid == "" {
		gaugeSafeDec(buf.byPhase, PhaseConsuming)
		return
	}
	if buf.traceLastPhase == nil {
		return
	}
	old, ok := buf.traceLastPhase[tid]
	if ok && old != "" {
		gaugeSafeDec(buf.byPhase, old)
		delete(buf.traceLastPhase, tid)
	}
}

// adjustGaugeOnLivePublish cập nhật gauge theo chuyển phase của một trace (mỗi Publish một bước).
// Gọi từ RecordCommandCenterPublish — cumulative đã tăng riêng.
func adjustGaugeOnLivePublish(orgHex, traceID, newPhase string) {
	if orgHex == "" || strings.TrimSpace(traceID) == "" {
		return
	}
	tid := strings.TrimSpace(traceID)
	newPhase = strings.TrimSpace(newPhase)
	if newPhase == "" {
		newPhase = "unknown"
	}
	buf := gaugeBufForOrg(orgHex)
	if buf == nil {
		return
	}
	buf.mu.Lock()
	defer buf.mu.Unlock()
	if buf.byPhase == nil {
		buf.byPhase = make(map[string]int64)
	}
	if buf.traceLastPhase == nil {
		buf.traceLastPhase = make(map[string]string)
	}
	old := buf.traceLastPhase[tid]
	if newPhase == old {
		return
	}
	terminal := isGaugeTerminalPhase(newPhase)
	if old != "" {
		gaugeSafeDec(buf.byPhase, old)
	}
	if !terminal {
		gaugeSafeInc(buf.byPhase, newPhase)
		buf.traceLastPhase[tid] = newPhase
	} else {
		delete(buf.traceLastPhase, tid)
	}
	trimTraceGaugeMap(buf)
}

func trimTraceGaugeMap(buf *orgPhaseGaugeBuf) {
	for len(buf.traceLastPhase) > maxTraceGaugeTracking {
		var victim string
		for t := range buf.traceLastPhase {
			victim = t
			break
		}
		old := buf.traceLastPhase[victim]
		delete(buf.traceLastPhase, victim)
		if old != "" {
			gaugeSafeDec(buf.byPhase, old)
		}
	}
}

// gaugeSnapshotForOrg bản sao gauge cho snapshot (không âm).
func gaugeSnapshotForOrg(orgHex string) map[string]int64 {
	out := defaultGaugeByPhase()
	if orgHex == "" {
		return out
	}
	v, ok := memPhaseGauge.Load(orgHex)
	if !ok {
		return out
	}
	buf := v.(*orgPhaseGaugeBuf)
	buf.mu.Lock()
	defer buf.mu.Unlock()
	for k, n := range buf.byPhase {
		if n < 0 {
			n = 0
		}
		out[k] = n
	}
	return out
}

func defaultGaugeByPhase() map[string]int64 {
	m := make(map[string]int64)
	m[GaugeKeyWorkerHeld] = 0
	for k := range defaultPhaseCounts() {
		m[k] = 0
	}
	return m
}
