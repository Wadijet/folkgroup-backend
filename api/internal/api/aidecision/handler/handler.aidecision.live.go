// Package aidecisionhdl — Live timeline + WebSocket cho AI Decision (backend only).
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

// HandleTraceTimeline GET /ai-decision/traces/:traceId/timeline — replay sự kiện đã buffer (canonical).
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

// HandleTraceLiveWS GET /ai-decision/traces/:traceId/live — WebSocket: gửi replay rồi stream tiếp.
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

		// Đăng ký trước replay để không mất event publish trong lúc gửi replay.
		liveCh, cancel := decisionlive.Subscribe(*orgID, traceID)
		defer cancel()

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
		if err := drainLiveChSkipAlreadyReplayed(conn, liveCh, maxSeq, false); err != nil {
			logrus.WithError(err).Debug("AI Decision live: drain sau replay thất bại")
			return
		}

		// Goroutine đọc từ client + xử lý control frame; goroutine ping giữ kết động và gia hạn read deadline qua Pong.
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

// drainLiveChSkipAlreadyReplayed đọc hết buffer liveCh sau replay; bỏ qua event trùng seq đã gửi trong replay.
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

// HandleOrgLiveTimeline GET /ai-decision/org-live/timeline — replay stream theo tổ chức (mọi trace).
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

// HandleOrgLivePersistedEvents GET /ai-decision/org-live/persisted-events — chỉ đọc decision_org_live_events (Mongo), không fallback RAM; cần bật AI_DECISION_LIVE_ORG_PERSIST.
// Query: page, limit (mặc 50, tối đa 100), traceId?, decisionCaseId?, fromCreatedMs?, toCreatedMs? (Unix ms). Sort createdAt giảm dần.
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

// HandleOrgLiveWS GET /ai-decision/org-live — WebSocket: replay buffer org rồi stream mọi trace.
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

		// Đăng ký trước replay — tránh mất toàn bộ event publish trong lúc gửi replay (race subscribe sau).
		liveCh, cancel := decisionlive.SubscribeOrg(*orgID)
		defer cancel()

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
		if err := drainLiveChSkipAlreadyReplayed(conn, liveCh, maxFeed, true); err != nil {
			logrus.WithError(err).Debug("AI Decision org-live: drain sau replay thất bại")
			return
		}

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
