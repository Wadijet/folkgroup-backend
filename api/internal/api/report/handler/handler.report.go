// Package reporthdl chứa HTTP handler cho domain Report (trend, recompute).
// File: basehdl.report.go - giữ tên cấu trúc cũ (basehdl.<domain>.<entity>.go).
package reporthdl

import (
	"fmt"
	"time"

	reportdto "meta_commerce/internal/api/report/dto"
	reportsvc "meta_commerce/internal/api/report/service"
	crmvc "meta_commerce/internal/api/crm/service"
	basehdl "meta_commerce/internal/api/base/handler"
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

// HandleTrend xử lý GET /reports/trend — loại báo cáo qua query reportKey.
// URL: GET /api/v1/reports/trend?reportKey=order_daily&from=dd-mm-yyyy&to=dd-mm-yyyy
func (h *ReportHandler) HandleTrend(c fiber.Ctx) error {
	return basehdl.SafeHandlerWrapper(c, func() error {
		var q reportdto.ReportTrendQuery
		_ = c.Bind().Query(&q)
		if q.ReportKey == "" {
			q.ReportKey = c.Query("reportKey")
		}
		if q.From == "" {
			q.From = c.Query("from")
		}
		if q.To == "" {
			q.To = c.Query("to")
		}
		if q.ReportKey == "" {
			c.Status(common.StatusBadRequest).JSON(fiber.Map{
				"code": common.ErrCodeValidationInput.Code, "message": "Thiếu reportKey (query: reportKey=order_daily)", "status": "error",
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
		if q.From == "" || q.To == "" {
			c.Status(common.StatusBadRequest).JSON(fiber.Map{
				"code": common.ErrCodeValidationInput.Code, "message": "Thiếu from hoặc to (dd-mm-yyyy). Ví dụ: ?reportKey=order_daily&from=01-01-2025&to=31-01-2025", "status": "error",
			})
			return nil
		}

		fromT, err := time.Parse(reportdto.ReportDateFormat, q.From)
		if err != nil {
			c.Status(common.StatusBadRequest).JSON(fiber.Map{
				"code": common.ErrCodeValidationFormat.Code, "message": "from không đúng định dạng dd-mm-yyyy", "status": "error",
			})
			return nil
		}
		toT, err := time.Parse(reportdto.ReportDateFormat, q.To)
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

		def, err := h.ReportService.LoadDefinition(c.Context(), q.ReportKey)
		if err != nil {
			c.Status(common.StatusNotFound).JSON(fiber.Map{
				"code": common.ErrCodeDatabaseQuery.Code, "message": "Không tìm thấy báo cáo với reportKey này", "status": "error",
			})
			return nil
		}

		fromStr, toStr := formatTrendRange(fromT, toT, def.PeriodType)

		list, err := h.ReportService.FindSnapshotsForTrend(c.Context(), q.ReportKey, *orgID, fromStr, toStr)
		if err != nil {
			c.Status(common.StatusInternalServerError).JSON(fiber.Map{
				"code": common.ErrCodeDatabase.Code, "message": "Lỗi truy vấn báo cáo", "status": "error",
			})
			return nil
		}
		c.Status(common.StatusOK).JSON(fiber.Map{
			"code": common.StatusOK, "message": "Thành công", "data": list, "status": "success",
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
				if err := h.ReportService.Compute(ctx, body.ReportKey, periodKey, *orgID); err != nil {
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
				if err := h.ReportService.Compute(ctx, body.ReportKey, periodKey, *orgID); err != nil {
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
				if err := h.ReportService.Compute(ctx, body.ReportKey, periodKey, *orgID); err != nil {
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
				if err := h.ReportService.Compute(ctx, body.ReportKey, periodKey, *orgID); err != nil {
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
