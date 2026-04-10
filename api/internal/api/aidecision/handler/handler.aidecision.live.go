// Package aidecisionhdl — HTTP + WebSocket cho timeline AI Decision (theo trace và theo org).
//
// Luồng đọc timeline một trace (khớp decisionlive.Publish / Timeline / Subscribe):
//
//	REST GET …/traces/:traceId/timeline — chỉ snapshot ring + backfill (không mở WS).
//	WS …/traces/:traceId/live — (1) Subscribe trace (2) Timeline replay (3) gửi replay (4) drain trùng Seq (5) vòng đọc liveCh + ping.
//
// Org-live: GET …/org-live/timeline dùng OrgTimelineForAPI; WS …/org-live tương tự với FeedSeq + aggregate định kỳ.
package aidecisionhdl

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/fasthttp/websocket"
	"github.com/gofiber/fiber/v3"
	"github.com/sirupsen/logrus"
	"github.com/valyala/fasthttp"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"meta_commerce/internal/api/aidecision/decisionlive"
	basehdl "meta_commerce/internal/api/base/handler"
	"meta_commerce/internal/common"
)

var wsUpgrader = websocket.FastHTTPUpgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 4096,
	CheckOrigin: func(_ *fasthttp.RequestCtx) bool {
		// Cho phép cùng origin / dev; production nên siết theo header Origin.
		return true
	},
}

// HandleTraceTimeline GET /ai-decision/traces/:traceId/timeline — Replay timeline một trace từ RAM (decisionlive.Timeline).
//
//	Bước 1 — Xác thực org + traceId.
//	Bước 2 — decisionlive.Timeline: snapshot ring (Publish đã ghi) + backfill.
//	Bước 3 — JSON { traceId, events }.
func HandleTraceTimeline(c fiber.Ctx) error {
	return basehdl.SafeHandlerWrapper(c, func() error {
		orgID := getActiveOrgID(c)
		if orgID == nil {
			c.Status(common.StatusBadRequest).JSON(fiber.Map{
				"code": common.ErrCodeValidationInput.Code, "message": "Chưa chọn tổ chức", "status": "error",
			})
			return nil
		}
		traceID := c.Params("traceId")
		if traceID == "" {
			c.Status(common.StatusBadRequest).JSON(fiber.Map{
				"code": common.ErrCodeValidationFormat.Code, "message": "Thiếu traceId", "status": "error",
			})
			return nil
		}
		events := decisionlive.Timeline(*orgID, traceID)
		c.Status(common.StatusOK).JSON(fiber.Map{
			"code": common.StatusOK, "message": "OK", "status": "success",
			"data": fiber.Map{
				"traceId": traceID,
				"events":  events,
			},
		})
		return nil
	})
}

// HandleTraceLiveWS GET /ai-decision/traces/:traceId/live — WebSocket: replay timeline trace rồi stream các mốc Publish tiếp theo.
//
//	Trong handler upgrade: Subscribe trước (tránh lỗi race với Publish), rồi Timeline, gửi từng event, drain kênh trùng Seq đã replay, sau đó for-select đọc liveCh.
func HandleTraceLiveWS(c fiber.Ctx) error {
	orgID := getActiveOrgID(c)
	if orgID == nil {
		return c.Status(common.StatusBadRequest).JSON(fiber.Map{
			"code": common.ErrCodeValidationInput.Code, "message": "Chưa chọn tổ chức", "status": "error",
		})
	}
	traceID := c.Params("traceId")
	if traceID == "" {
		return c.Status(common.StatusBadRequest).JSON(fiber.Map{
			"code": common.ErrCodeValidationFormat.Code, "message": "Thiếu traceId", "status": "error",
		})
	}

	err := wsUpgrader.Upgrade(c.RequestCtx(), func(conn *websocket.Conn) {
		defer conn.Close()
		_ = conn.SetReadDeadline(time.Now().Add(90 * time.Second))
		conn.SetPongHandler(func(string) error {
			_ = conn.SetReadDeadline(time.Now().Add(120 * time.Second))
			return nil
		})

		// Bước WS 1 — Subscribe trace trước replay (cùng kênh Publish bước 6a); tránh mất mốc publish xen giữa replay.
		liveCh, cancel := decisionlive.Subscribe(*orgID, traceID)
		defer cancel()

		// Bước WS 2–3 — Snapshot timeline (ring) và gửi replay; ghi maxSeq để drain bỏ trùng.
		events := decisionlive.Timeline(*orgID, traceID)
		var maxSeq int64
		for _, ev := range events {
			if ev.Seq > maxSeq {
				maxSeq = ev.Seq
			}
			if err := writeLiveJSON(conn, ev); err != nil {
				logrus.WithError(err).Debug("AI Decision live: gửi replay thất bại")
				return
			}
		}
		// Bước WS 4 — Đọc non-blocking liveCh: bỏ event đã có trong replay (Seq ≤ maxSeq), gửi các mốc mới xen kẽ.
		if err := drainLiveChSkipAlreadyReplayed(conn, liveCh, maxSeq, false); err != nil {
			logrus.WithError(err).Debug("AI Decision live: drain sau replay thất bại")
			return
		}

		// Bước WS 5 — Ping + đọc control từ client; vòng for-select nhận tiếp từ liveCh (stream realtime).
		done := make(chan struct{})
		startLiveWSPing(conn, done)
		go func() {
			for {
				if _, _, err := conn.ReadMessage(); err != nil {
					close(done)
					return
				}
			}
		}()

		for {
			select {
			case <-done:
				return
			case ev, ok := <-liveCh:
				if !ok {
					return
				}
				if err := writeLiveJSON(conn, ev); err != nil {
					logrus.WithError(err).Debug("AI Decision live: gửi stream thất bại")
					return
				}
			}
		}
	})
	if err != nil {
		logrus.WithError(err).Warn("AI Decision live: WebSocket upgrade thất bại")
		return err
	}
	return nil
}

func writeLiveJSON(conn *websocket.Conn, ev decisionlive.DecisionLiveEvent) error {
	decisionlive.SanitizeDecisionLiveEventJSON(&ev)
	b, err := json.Marshal(ev)
	if err != nil {
		return err
	}
	return conn.WriteMessage(websocket.TextMessage, b)
}

// drainLiveChSkipAlreadyReplayed — Sau replay timeline: quét non-blocking liveCh; bỏ mốc trùng Seq (trace) hoặc FeedSeq (org) đã gửi.
func drainLiveChSkipAlreadyReplayed(conn *websocket.Conn, liveCh <-chan decisionlive.DecisionLiveEvent, maxSeq int64, useFeedSeq bool) error {
	for {
		select {
		case ev := <-liveCh:
			if useFeedSeq {
				if ev.FeedSeq > 0 && maxSeq > 0 && ev.FeedSeq <= maxSeq {
					continue
				}
			} else {
				if ev.Seq > 0 && maxSeq > 0 && ev.Seq <= maxSeq {
					continue
				}
			}
			if err := writeLiveJSON(conn, ev); err != nil {
				return err
			}
		default:
			return nil
		}
	}
}

// writeCommandCenterAggregate gửi aggregate command center qua WS — queueDepth chỉ từ RAM (đồng bộ Mongo lúc khởi động + mỗi ~5 phút).
func writeCommandCenterAggregate(conn *websocket.Conn, orgID primitive.ObjectID) error {
	if orgID.IsZero() {
		return nil
	}
	snap := decisionlive.BuildCommandCenterSnapshot(context.Background(), orgID, false)
	env := fiber.Map{
		"type":            "aggregate",
		"schemaVersion":   1,
		"stream":          decisionlive.StreamAIDecision,
		"commandCenter":   true,
		"payload":         snap,
	}
	b, err := json.Marshal(env)
	if err != nil {
		return err
	}
	return conn.WriteMessage(websocket.TextMessage, b)
}

// startLiveWSPing gửi Ping định kỳ để phía client trả Pong — gia hạn read deadline (tránh kết nối chỉ-nhận bị đóng im lặng).
func startLiveWSPing(conn *websocket.Conn, done <-chan struct{}) {
	go func() {
		ticker := time.NewTicker(25 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				_ = conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
				if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
					return
				}
			}
		}
	}()
}

// HandleOrgLiveTimeline GET /ai-decision/org-live/timeline — Replay org-live (OrgTimelineForAPI: Mongo nếu persist bật, không thì RAM).
func HandleOrgLiveTimeline(c fiber.Ctx) error {
	return basehdl.SafeHandlerWrapper(c, func() error {
		orgID := getActiveOrgID(c)
		if orgID == nil {
			c.Status(common.StatusBadRequest).JSON(fiber.Map{
				"code": common.ErrCodeValidationInput.Code, "message": "Chưa chọn tổ chức", "status": "error",
			})
			return nil
		}
		events := decisionlive.OrgTimelineForAPI(c.Context(), *orgID)
		c.Status(common.StatusOK).JSON(fiber.Map{
			"code": common.StatusOK, "message": "OK", "status": "success",
			"data": fiber.Map{
				"events": events,
			},
		})
		return nil
	})
}

// HandleOrgLivePersistedEvents GET /ai-decision/org-live/persisted-events — Chỉ đọc Mongo decision_org_live_events (không RAM).
//
//	Bước 1 — Kiểm tra org; parse page/limit/query lọc (traceId, decisionCaseId, from/to createdAt ms).
//	Bước 2 — ListPersistedOrgLiveEventsFromMongo: filter + sort createdAt giảm + phân trang.
//	Bước 3 — Trả events (từ json.Unmarshal payload) + pagination; persist tắt → 400 ErrOrgLivePersistDisabled.
// Cần AI_DECISION_LIVE_ORG_PERSIST (hoặc config tương đương) bật.
func HandleOrgLivePersistedEvents(c fiber.Ctx) error {
	return basehdl.SafeHandlerWrapper(c, func() error {
		orgID := getActiveOrgID(c)
		if orgID == nil {
			c.Status(common.StatusBadRequest).JSON(fiber.Map{
				"code": common.ErrCodeValidationInput.Code, "message": "Chưa chọn tổ chức", "status": "error",
			})
			return nil
		}
		page := queryPositiveInt(c, "page", 1)
		limit := queryPositiveInt(c, "limit", 50)
		f := decisionlive.PersistedOrgLiveListFilter{
			OwnerOrgID:     *orgID,
			Page:           page,
			Limit:          limit,
			TraceID:        c.Query("traceId"),
			DecisionCaseID: c.Query("decisionCaseId"),
		}
		if v, ok := queryInt64Ptr(c, "fromCreatedMs"); ok {
			f.FromCreatedMs = v
		}
		if v, ok := queryInt64Ptr(c, "toCreatedMs"); ok {
			f.ToCreatedMs = v
		}
		items, total, err := decisionlive.ListPersistedOrgLiveEventsFromMongo(c.Context(), f)
		if err != nil {
			if errors.Is(err, decisionlive.ErrOrgLivePersistDisabled) {
				c.Status(common.StatusBadRequest).JSON(fiber.Map{
					"code":    common.ErrCodeValidationInput.Code,
					"message": "Org-live chưa bật ghi Mongo — đặt AI_DECISION_LIVE_ORG_PERSIST=true (hoặc env tương đương) để dùng API này.",
					"status":  "error",
				})
				return nil
			}
			errCode, msg, statusCode := common.GetErrorResponseInfo(err, "Không đọc được org-live từ Mongo")
			c.Status(statusCode).JSON(fiber.Map{"code": errCode, "message": msg, "status": "error"})
			return nil
		}
		c.Status(common.StatusOK).JSON(fiber.Map{
			"code": common.StatusOK, "message": "OK", "status": "success",
			"data": fiber.Map{
				"events": items,
				"pagination": fiber.Map{
					"page": page, "limit": limit, "total": total,
					"totalPages": paginationTotalPages(total, limit),
				},
				"source": "mongodb_decision_org_live_events",
			},
		})
		return nil
	})
}

// HandleOrgLiveMetrics GET /ai-decision/org-live/metrics — dự phòng HTTP: cùng nguồn RAM với WS (reconcile Mongo lần đầu + định kỳ).
func HandleOrgLiveMetrics(c fiber.Ctx) error {
	return basehdl.SafeHandlerWrapper(c, func() error {
		orgID := getActiveOrgID(c)
		if orgID == nil {
			c.Status(common.StatusBadRequest).JSON(fiber.Map{
				"code": common.ErrCodeValidationInput.Code, "message": "Chưa chọn tổ chức", "status": "error",
			})
			return nil
		}
		snap := decisionlive.BuildCommandCenterSnapshot(c.Context(), *orgID, false)
		c.Status(common.StatusOK).JSON(fiber.Map{
			"code": common.StatusOK, "message": "OK", "status": "success",
			"data": snap,
		})
		return nil
	})
}

// HandleOrgLiveWS GET /ai-decision/org-live — WS org: replay OrgTimelineForAPI → drain FeedSeq trùng → aggregate định kỳ + stream liveCh.
func HandleOrgLiveWS(c fiber.Ctx) error {
	orgID := getActiveOrgID(c)
	if orgID == nil {
		return c.Status(common.StatusBadRequest).JSON(fiber.Map{
			"code": common.ErrCodeValidationInput.Code, "message": "Chưa chọn tổ chức", "status": "error",
		})
	}

	err := wsUpgrader.Upgrade(c.RequestCtx(), func(conn *websocket.Conn) {
		defer conn.Close()
		_ = conn.SetReadDeadline(time.Now().Add(90 * time.Second))
		conn.SetPongHandler(func(string) error {
			_ = conn.SetReadDeadline(time.Now().Add(120 * time.Second))
			return nil
		})

		// Bước org WS 1 — SubscribeOrg trước replay (kênh Publish bước 6b).
		liveCh, cancel := decisionlive.SubscribeOrg(*orgID)
		defer cancel()

		// Bước org WS 2–3 — Replay (Mongo/RAM) + gửi client; maxFeed cho drain.
		var maxFeed int64
		for _, ev := range decisionlive.OrgTimelineForAPI(context.Background(), *orgID) {
			if ev.FeedSeq > maxFeed {
				maxFeed = ev.FeedSeq
			}
			if err := writeLiveJSON(conn, ev); err != nil {
				logrus.WithError(err).Debug("AI Decision org-live: gửi replay thất bại")
				return
			}
		}
		// Bước org WS 4 — Drain trùng FeedSeq đã replay.
		if err := drainLiveChSkipAlreadyReplayed(conn, liveCh, maxFeed, true); err != nil {
			logrus.WithError(err).Debug("AI Decision org-live: drain sau replay thất bại")
			return
		}

		// Bước org WS 5 — Một bản aggregate trung tâm chỉ huy ngay sau replay.
		if err := writeCommandCenterAggregate(conn, *orgID); err != nil {
			logrus.WithError(err).Debug("AI Decision org-live: gửi aggregate ban đầu thất bại")
		}

		done := make(chan struct{})
		startLiveWSPing(conn, done)
		go func() {
			for {
				if _, _, err := conn.ReadMessage(); err != nil {
					close(done)
					return
				}
			}
		}()

		// Bước org WS 6 — Vòng select: aggregate định kỳ + stream từ liveCh (mọi trace).
		aggTicker := time.NewTicker(decisionlive.WSCommandCenterAggregateInterval())
		defer aggTicker.Stop()

		for {
			select {
			case <-done:
				return
			case <-aggTicker.C:
				if err := writeCommandCenterAggregate(conn, *orgID); err != nil {
					logrus.WithError(err).Debug("AI Decision org-live: gửi aggregate định kỳ thất bại")
					return
				}
			case ev, ok := <-liveCh:
				if !ok {
					return
				}
				if err := writeLiveJSON(conn, ev); err != nil {
					logrus.WithError(err).Debug("AI Decision org-live: gửi stream thất bại")
					return
				}
			}
		}
	})
	if err != nil {
		logrus.WithError(err).Warn("AI Decision org-live: WebSocket upgrade thất bại")
		return err
	}
	return nil
}
