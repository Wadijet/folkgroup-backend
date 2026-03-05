// Package reporthdl chứa HTTP handler cho domain Report (trend, recompute).
// File: basehdl.report.go - giữ tên cấu trúc cũ (basehdl.<domain>.<entity>.go).
package reporthdl

import (
	"fmt"
	"strings"
	"time"

	basehdl "meta_commerce/internal/api/base/handler"
	crmvc "meta_commerce/internal/api/crm/service"
	reportdto "meta_commerce/internal/api/report/dto"
	reportsvc "meta_commerce/internal/api/report/service"
	"meta_commerce/internal/common"

	"github.com/gofiber/fiber/v3"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// ReportHandler xử lý API báo cáo theo chu kỳ: GET trend, POST recompute, dashboard customers.
type ReportHandler struct {
	ReportService     *reportsvc.ReportService
	CrmCustomerService *crmvc.CrmCustomerService
}

// NewReportHandler tạo mới ReportHandler.
func NewReportHandler() (*ReportHandler, error) {
	svc, err := reportsvc.NewReportService()
	if err != nil {
		return nil, fmt.Errorf("tạo ReportService: %w", err)
	}
	crmSvc, err := crmvc.NewCrmCustomerService()
	if err != nil {
		return nil, fmt.Errorf("tạo CrmCustomerService: %w", err)
	}
	return &ReportHandler{
		ReportService:     svc,
		CrmCustomerService: crmSvc,
	}, nil
}

// HandleOrderPeriodMovementsFromSnapshots xử lý GET /reports/order/period-movements-from-snapshots — CHÍNH: order phát sinh từ report_snapshots.
// Query: from=dd-mm-yyyy&to=dd-mm-yyyy. Không cần reportKey (domain order cố định).
func (h *ReportHandler) HandleOrderPeriodMovementsFromSnapshots(c fiber.Ctx) error {
	return basehdl.SafeHandlerWrapper(c, func() error {
		fromStr := c.Query("from")
		toStr := c.Query("to")
		if fromStr == "" || toStr == "" {
			c.Status(common.StatusBadRequest).JSON(fiber.Map{
				"code": common.ErrCodeValidationInput.Code, "message": "Thiếu from hoặc to (dd-mm-yyyy). Ví dụ: ?from=01-01-2025&to=31-01-2025", "status": "error",
			})
			return nil
		}
		orgID := getActiveOrganizationID(c)
		if orgID == nil || orgID.IsZero() {
			c.Status(common.StatusBadRequest).JSON(fiber.Map{
				"code": common.ErrCodeValidationInput.Code, "message": "Vui lòng chọn tổ chức (active organization)", "status": "error",
			})
			return nil
		}
		fromT, err := time.Parse(reportdto.ReportDateFormat, fromStr)
		if err != nil {
			c.Status(common.StatusBadRequest).JSON(fiber.Map{
				"code": common.ErrCodeValidationFormat.Code, "message": "from không đúng định dạng dd-mm-yyyy", "status": "error",
			})
			return nil
		}
		toT, err := time.Parse(reportdto.ReportDateFormat, toStr)
		if err != nil {
			c.Status(common.StatusBadRequest).JSON(fiber.Map{
				"code": common.ErrCodeValidationFormat.Code, "message": "to không đúng định dạng dd-mm-yyyy", "status": "error",
			})
			return nil
		}
		if fromT.After(toT) {
			c.Status(common.StatusBadRequest).JSON(fiber.Map{
				"code": common.ErrCodeValidationInput.Code, "message": "from phải nhỏ hơn hoặc bằng to", "status": "error",
			})
			return nil
		}
		startMs, endMs := dayRangeToMs(fromT, toT)
		reportKeyOrder := reportsvc.GetReportKeyOrderForDomain("order")
		list, err := h.ReportService.FindSnapshotsForTrendByDayRange(c.Context(), *orgID, startMs, endMs, reportKeyOrder)
		if err != nil {
			c.Status(common.StatusInternalServerError).JSON(fiber.Map{
				"code": common.ErrCodeDatabase.Code, "message": "Lỗi truy vấn báo cáo order", "status": "error",
			})
			return nil
		}
		c.Status(common.StatusOK).JSON(fiber.Map{
			"code": common.StatusOK, "message": "Thành công", "data": list,
			"meta": fiber.Map{"dataSource": "snapshots", "domain": "order", "description": "Số phát sinh order trong kỳ từ report_snapshots"},
			"status": "success",
		})
		return nil
	})
}

// HandleAdsDailyTrendFromSnapshots xử lý GET /reports/ads/period-movements-from-snapshots — ads_daily trend từ report_snapshots.
// Query: from=dd-mm-yyyy&to=dd-mm-yyyy&adAccountId=xxx (adAccountId optional).
func (h *ReportHandler) HandleAdsDailyTrendFromSnapshots(c fiber.Ctx) error {
	return basehdl.SafeHandlerWrapper(c, func() error {
		fromStr := c.Query("from")
		toStr := c.Query("to")
		if fromStr == "" || toStr == "" {
			c.Status(common.StatusBadRequest).JSON(fiber.Map{
				"code": common.ErrCodeValidationInput.Code, "message": "Thiếu from hoặc to (dd-mm-yyyy). Ví dụ: ?from=01-01-2025&to=31-01-2025", "status": "error",
			})
			return nil
		}
		orgID := getActiveOrganizationID(c)
		if orgID == nil || orgID.IsZero() {
			c.Status(common.StatusBadRequest).JSON(fiber.Map{
				"code": common.ErrCodeValidationInput.Code, "message": "Vui lòng chọn tổ chức (active organization)", "status": "error",
			})
			return nil
		}
		fromT, err := time.Parse(reportdto.ReportDateFormat, fromStr)
		if err != nil {
			c.Status(common.StatusBadRequest).JSON(fiber.Map{
				"code": common.ErrCodeValidationFormat.Code, "message": "from không đúng định dạng dd-mm-yyyy", "status": "error",
			})
			return nil
		}
		toT, err := time.Parse(reportdto.ReportDateFormat, toStr)
		if err != nil {
			c.Status(common.StatusBadRequest).JSON(fiber.Map{
				"code": common.ErrCodeValidationFormat.Code, "message": "to không đúng định dạng dd-mm-yyyy", "status": "error",
			})
			return nil
		}
		if fromT.After(toT) {
			c.Status(common.StatusBadRequest).JSON(fiber.Map{
				"code": common.ErrCodeValidationInput.Code, "message": "from phải nhỏ hơn hoặc bằng to", "status": "error",
			})
			return nil
		}
		startMs, endMs := dayRangeToMs(fromT, toT)
		adAccountId := strings.TrimSpace(c.Query("adAccountId"))
		list, err := h.ReportService.FindSnapshotsForAdsTrendByDayRange(c.Context(), *orgID, startMs, endMs, adAccountId)
		if err != nil {
			c.Status(common.StatusInternalServerError).JSON(fiber.Map{
				"code": common.ErrCodeDatabase.Code, "message": "Lỗi truy vấn báo cáo ads_daily", "status": "error",
			})
			return nil
		}
		c.Status(common.StatusOK).JSON(fiber.Map{
			"code": common.StatusOK, "message": "Thành công", "data": list,
			"meta": fiber.Map{"dataSource": "snapshots", "domain": "ads", "reportKey": "ads_daily", "description": "Snapshots ads_daily trong kỳ từ report_snapshots"},
			"status": "success",
		})
		return nil
	})
}

// HandleOrderPeriodMovementsFromDb xử lý GET /reports/order/period-movements-from-db — PHỤ: order phát sinh từ DB (aggregate pc_pos_orders, đối chiếu).
// Query: from=dd-mm-yyyy&to=dd-mm-yyyy. Trả về format giống period-movements-from-snapshots (reportKey=order_*).
func (h *ReportHandler) HandleOrderPeriodMovementsFromDb(c fiber.Ctx) error {
	return basehdl.SafeHandlerWrapper(c, func() error {
		fromStr := c.Query("from")
		toStr := c.Query("to")
		if fromStr == "" || toStr == "" {
			c.Status(common.StatusBadRequest).JSON(fiber.Map{
				"code": common.ErrCodeValidationInput.Code, "message": "Thiếu from hoặc to (dd-mm-yyyy). Ví dụ: ?from=01-01-2025&to=31-01-2025", "status": "error",
			})
			return nil
		}
		orgID := getActiveOrganizationID(c)
		if orgID == nil || orgID.IsZero() {
			c.Status(common.StatusBadRequest).JSON(fiber.Map{
				"code": common.ErrCodeValidationInput.Code, "message": "Vui lòng chọn tổ chức (active organization)", "status": "error",
			})
			return nil
		}
		fromT, err := time.Parse(reportdto.ReportDateFormat, fromStr)
		if err != nil {
			c.Status(common.StatusBadRequest).JSON(fiber.Map{
				"code": common.ErrCodeValidationFormat.Code, "message": "from không đúng định dạng dd-mm-yyyy", "status": "error",
			})
			return nil
		}
		toT, err := time.Parse(reportdto.ReportDateFormat, toStr)
		if err != nil {
			c.Status(common.StatusBadRequest).JSON(fiber.Map{
				"code": common.ErrCodeValidationFormat.Code, "message": "to không đúng định dạng dd-mm-yyyy", "status": "error",
			})
			return nil
		}
		if fromT.After(toT) {
			c.Status(common.StatusBadRequest).JSON(fiber.Map{
				"code": common.ErrCodeValidationInput.Code, "message": "from phải nhỏ hơn hoặc bằng to", "status": "error",
			})
			return nil
		}
		startMs, endMs := dayRangeToMs(fromT, toT)
		list, err := h.ReportService.GetOrderTrendFromDb(c.Context(), *orgID, startMs, endMs)
		if err != nil {
			c.Status(common.StatusInternalServerError).JSON(fiber.Map{
				"code": common.ErrCodeDatabase.Code, "message": "Lỗi truy vấn báo cáo order từ DB", "status": "error",
			})
			return nil
		}
		c.Status(common.StatusOK).JSON(fiber.Map{
			"code": common.StatusOK, "message": "Thành công", "data": list,
			"meta": fiber.Map{"dataSource": "db", "domain": "order", "description": "Số phát sinh order trong kỳ aggregate trực tiếp từ pc_pos_orders"},
			"status": "success",
		})
		return nil
	})
}

// HandleRecompute xử lý POST /reports/recompute — loại báo cáo qua body reportKey.
// URL: POST /api/v1/reports/recompute, body: {"reportKey":"order_daily","from":"dd-mm-yyyy","to":"dd-mm-yyyy"}
func (h *ReportHandler) HandleRecompute(c fiber.Ctx) error {
	return basehdl.SafeHandlerWrapper(c, func() error {
		var body reportdto.ReportRecomputeBody
		if err := c.Bind().Body(&body); err != nil {
			c.Status(common.StatusBadRequest).JSON(fiber.Map{
				"code": common.ErrCodeValidationFormat.Code, "message": "Body không hợp lệ (cần reportKey, from, to)", "status": "error",
			})
			return nil
		}
		if body.ReportKey == "" {
			c.Status(common.StatusBadRequest).JSON(fiber.Map{
				"code": common.ErrCodeValidationInput.Code, "message": "Thiếu reportKey trong body (vd: order_daily)", "status": "error",
			})
			return nil
		}
		if body.From == "" || body.To == "" {
			c.Status(common.StatusBadRequest).JSON(fiber.Map{
				"code": common.ErrCodeValidationInput.Code, "message": "Thiếu from hoặc to (dd-mm-yyyy)", "status": "error",
			})
			return nil
		}
		orgID := getActiveOrganizationID(c)
		if orgID == nil || orgID.IsZero() {
			c.Status(common.StatusBadRequest).JSON(fiber.Map{
				"code": common.ErrCodeValidationInput.Code, "message": "Vui lòng chọn tổ chức (active organization)", "status": "error",
			})
			return nil
		}

		fromT, err := time.Parse(reportdto.ReportDateFormat, body.From)
		if err != nil {
			c.Status(common.StatusBadRequest).JSON(fiber.Map{
				"code": common.ErrCodeValidationFormat.Code, "message": "from không đúng định dạng dd-mm-yyyy", "status": "error",
			})
			return nil
		}
		toT, err := time.Parse(reportdto.ReportDateFormat, body.To)
		if err != nil {
			c.Status(common.StatusBadRequest).JSON(fiber.Map{
				"code": common.ErrCodeValidationFormat.Code, "message": "to không đúng định dạng dd-mm-yyyy", "status": "error",
			})
			return nil
		}
		if fromT.After(toT) {
			c.Status(common.StatusBadRequest).JSON(fiber.Map{
				"code": common.ErrCodeValidationInput.Code, "message": "from phải nhỏ hơn hoặc bằng to", "status": "error",
			})
			return nil
		}

		def, err := h.ReportService.LoadDefinition(c.Context(), body.ReportKey)
		if err != nil {
			c.Status(common.StatusNotFound).JSON(fiber.Map{
				"code": common.ErrCodeDatabaseQuery.Code, "message": "Không tìm thấy báo cáo với reportKey này", "status": "error",
			})
			return nil
		}
		// Chặn chu kỳ customer/order bị tắt (vd: customer_monthly, order_weekly — tính on-demand khi xem).
		if reportsvc.IsCustomerReportKeyDisabled(body.ReportKey) {
			c.Status(common.StatusBadRequest).JSON(fiber.Map{
				"code": common.ErrCodeValidationInput.Code, "message": "Chu kỳ báo cáo này đã bị tắt tạm thời", "status": "error",
			})
			return nil
		}
		if reportsvc.IsOrderReportKeyDisabled(body.ReportKey) {
			c.Status(common.StatusBadRequest).JSON(fiber.Map{
				"code": common.ErrCodeValidationInput.Code, "message": "Chu kỳ báo cáo này đã bị tắt tạm thời", "status": "error",
			})
			return nil
		}
		if reportsvc.IsAdsReportKeyDisabled(body.ReportKey) {
			c.Status(common.StatusBadRequest).JSON(fiber.Map{
				"code": common.ErrCodeValidationInput.Code, "message": "Chu kỳ báo cáo này đã bị tắt tạm thời", "status": "error",
			})
			return nil
		}

		ctx := c.Context()
		count := 0
		switch def.PeriodType {
		case "day":
			days := int(toT.Sub(fromT).Hours()/24) + 1
			if days > 31 {
				c.Status(common.StatusBadRequest).JSON(fiber.Map{
					"code": common.ErrCodeValidationInput.Code, "message": "Khoảng từ from đến to tối đa 31 ngày", "status": "error",
				})
				return nil
			}
			for d := fromT; !d.After(toT); d = d.AddDate(0, 0, 1) {
				periodKey := d.Format("2006-01-02")
				if err := h.ReportService.Compute(ctx, body.ReportKey, periodKey, *orgID, ""); err != nil {
					c.Status(common.StatusInternalServerError).JSON(fiber.Map{
						"code": common.ErrCodeDatabase.Code, "message": "Lỗi tính báo cáo, vui lòng thử lại sau", "status": "error",
					})
					return nil
				}
				count++
			}
		case "week":
			// Lùi về thứ Hai của tuần chứa from
			weekday := int(fromT.Weekday())
			if weekday == 0 {
				weekday = 7
			}
			monday := fromT.AddDate(0, 0, -(weekday - 1))
			weeks := 0
			for d := monday; !d.After(toT) && weeks < 12; d = d.AddDate(0, 0, 7) {
				periodKey := d.Format("2006-01-02")
				if err := h.ReportService.Compute(ctx, body.ReportKey, periodKey, *orgID, ""); err != nil {
					c.Status(common.StatusInternalServerError).JSON(fiber.Map{
						"code": common.ErrCodeDatabase.Code, "message": "Lỗi tính báo cáo, vui lòng thử lại sau", "status": "error",
					})
					return nil
				}
				count++
				weeks++
			}
		case "month":
			d := time.Date(fromT.Year(), fromT.Month(), 1, 0, 0, 0, 0, fromT.Location())
			for (d.Before(toT) || d.Equal(toT)) && count < 12 {
				periodKey := d.Format("2006-01")
				if err := h.ReportService.Compute(ctx, body.ReportKey, periodKey, *orgID, ""); err != nil {
					c.Status(common.StatusInternalServerError).JSON(fiber.Map{
						"code": common.ErrCodeDatabase.Code, "message": "Lỗi tính báo cáo, vui lòng thử lại sau", "status": "error",
					})
					return nil
				}
				count++
				d = d.AddDate(0, 1, 0)
			}
		case "year":
			d := time.Date(fromT.Year(), 1, 1, 0, 0, 0, 0, fromT.Location())
			for (d.Before(toT) || d.Equal(toT)) && count < 5 {
				periodKey := d.Format("2006")
				if err := h.ReportService.Compute(ctx, body.ReportKey, periodKey, *orgID, ""); err != nil {
					c.Status(common.StatusInternalServerError).JSON(fiber.Map{
						"code": common.ErrCodeDatabase.Code, "message": "Lỗi tính báo cáo, vui lòng thử lại sau", "status": "error",
					})
					return nil
				}
				count++
				d = d.AddDate(1, 0, 0)
			}
		default:
			c.Status(common.StatusBadRequest).JSON(fiber.Map{
				"code": common.ErrCodeValidationInput.Code, "message": "Loại chu kỳ báo cáo chưa hỗ trợ", "status": "error",
			})
			return nil
		}

		c.Status(common.StatusOK).JSON(fiber.Map{
			"code": common.StatusOK, "message": "Đã tính lại báo cáo",
			"data": fiber.Map{"processedPeriods": count}, "status": "success",
		})
		return nil
	})
}

// formatTrendRange chuyển khoảng thời gian sang format periodKey cho query (day/week: YYYY-MM-DD, month: YYYY-MM, year: YYYY).
func formatTrendRange(fromT, toT time.Time, periodType string) (fromStr, toStr string) {
	switch periodType {
	case "month":
		return fromT.Format("2006-01"), toT.Format("2006-01")
	case "year":
		return fromT.Format("2006"), toT.Format("2006")
	default:
		return fromT.Format("2006-01-02"), toT.Format("2006-01-02")
	}
}

// getReportDomainFromKey trích domain từ reportKey (order|customer). Trả về rỗng nếu không hợp lệ.
func getReportDomainFromKey(reportKey string) string {
	if strings.HasPrefix(reportKey, "customer") {
		return "customer"
	}
	if strings.HasPrefix(reportKey, "order") {
		return "order"
	}
	return ""
}

// dayRangeToMs chuyển khoảng ngày [fromT, toT] thành startMs, endMs (đơn vị cơ sở ngày).
func dayRangeToMs(fromT, toT time.Time) (startMs, endMs int64) {
	loc, _ := time.LoadLocation("Asia/Ho_Chi_Minh")
	if loc == nil {
		loc = time.UTC
	}
	start := time.Date(fromT.Year(), fromT.Month(), fromT.Day(), 0, 0, 0, 0, loc)
	end := time.Date(toT.Year(), toT.Month(), toT.Day(), 23, 59, 59, 999999999, loc)
	return start.UnixMilli(), end.UnixMilli()
}

func getActiveOrganizationID(c fiber.Ctx) *primitive.ObjectID {
	orgIDStr, ok := c.Locals("active_organization_id").(string)
	if !ok || orgIDStr == "" {
		return nil
	}
	orgID, err := primitive.ObjectIDFromHex(orgIDStr)
	if err != nil {
		return nil
	}
	return &orgID
}
