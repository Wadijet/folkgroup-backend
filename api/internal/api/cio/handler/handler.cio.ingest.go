// Package ciohdl — HTTP handlers cho CIO Ingest Hub (điểm vào thống nhất cho Agent/sync).
package ciohdl

import (
	"encoding/json"
	"fmt"
	"strings"

	fbhdl "meta_commerce/internal/api/fb/handler"
	metahdl "meta_commerce/internal/api/meta/handler"
	"meta_commerce/internal/api/middleware"
	pchdl "meta_commerce/internal/api/pc/handler"
	"meta_commerce/internal/common"

	"github.com/gofiber/fiber/v3"
)

// CioIngestRequest body cho POST /cio/ingest — một endpoint, phân nhánh theo domain.
//
//	filter: tùy chọn — cùng ý nghĩa với query ?filter={} (JSON). Body filter được merge đè lên query.
//	data: payload giống hệt body từng domain (order, pos_*, interaction_*, fb_customer, meta_*, …).
//
// Domain "order" (pc_pos_orders): filter cần orderId (và shopId nếu cần); ownerOrganizationId lấy từ JWT khi không gửi.
// data tối thiểu: { "posData": <order từ Pancake POS> } — backend extract flatten + uid/sourceIds/links (4 lớp).
//
// Luồng: merge filter (?filter= + body.filter) → ProcessMergedFilter trên handler domain → SyncUpsertOneFromParts / UpsertMessagesFromParts
// (handler gọi *Service.RunSyncUpsertOneFromJSON / RunUpsertMessagesFromJSON — logic nghiệp vụ nằm ở service).
type CioIngestRequest struct {
	Domain string `json:"domain"`
	Filter map[string]interface{} `json:"filter,omitempty"`
	Data   json.RawMessage        `json:"data"`
}

// CioIngestHandler điều phối ingest đa domain qua một URL.
type CioIngestHandler struct {
	pcOrder       *pchdl.PcPosOrderHandler
	pcShop        *pchdl.PcPosShopHandler
	pcWarehouse   *pchdl.PcPosWarehouseHandler
	pcProduct     *pchdl.PcPosProductHandler
	pcVariation   *pchdl.PcPosVariationHandler
	pcCategory    *pchdl.PcPosCategoryHandler
	pcCustomer    *pchdl.PcPosCustomerHandler
	fbConv        *fbhdl.FbConversationHandler
	fbMsg         *fbhdl.FbMessageHandler
	fbCustomer    *fbhdl.FbCustomerHandler
	metaAdAccount *metahdl.MetaAdAccountHandler
	metaCampaign  *metahdl.MetaCampaignHandler
	metaAdSet     *metahdl.MetaAdSetHandler
	metaAd        *metahdl.MetaAdHandler
	metaAdInsight *metahdl.MetaAdInsightHandler
}

// NewCioIngestHandler khởi tạo handler CIO ingest.
func NewCioIngestHandler() (*CioIngestHandler, error) {
	pcOrder, err := pchdl.NewPcPosOrderHandler()
	if err != nil {
		return nil, fmt.Errorf("cio ingest: %w", err)
	}
	pcShop, err := pchdl.NewPcPosShopHandler()
	if err != nil {
		return nil, fmt.Errorf("cio ingest: %w", err)
	}
	pcWarehouse, err := pchdl.NewPcPosWarehouseHandler()
	if err != nil {
		return nil, fmt.Errorf("cio ingest: %w", err)
	}
	pcProduct, err := pchdl.NewPcPosProductHandler()
	if err != nil {
		return nil, fmt.Errorf("cio ingest: %w", err)
	}
	pcVariation, err := pchdl.NewPcPosVariationHandler()
	if err != nil {
		return nil, fmt.Errorf("cio ingest: %w", err)
	}
	pcCategory, err := pchdl.NewPcPosCategoryHandler()
	if err != nil {
		return nil, fmt.Errorf("cio ingest: %w", err)
	}
	pcCustomer, err := pchdl.NewPcPosCustomerHandler()
	if err != nil {
		return nil, fmt.Errorf("cio ingest: %w", err)
	}
	fbConv, err := fbhdl.NewFbConversationHandler()
	if err != nil {
		return nil, fmt.Errorf("cio ingest: %w", err)
	}
	fbMsg, err := fbhdl.NewFbMessageHandler()
	if err != nil {
		return nil, fmt.Errorf("cio ingest: %w", err)
	}
	fbCustomer, err := fbhdl.NewFbCustomerHandler()
	if err != nil {
		return nil, fmt.Errorf("cio ingest: %w", err)
	}
	metaAdAccount, err := metahdl.NewMetaAdAccountHandler()
	if err != nil {
		return nil, fmt.Errorf("cio ingest: %w", err)
	}
	metaCampaign, err := metahdl.NewMetaCampaignHandler()
	if err != nil {
		return nil, fmt.Errorf("cio ingest: %w", err)
	}
	metaAdSet, err := metahdl.NewMetaAdSetHandler()
	if err != nil {
		return nil, fmt.Errorf("cio ingest: %w", err)
	}
	metaAd, err := metahdl.NewMetaAdHandler()
	if err != nil {
		return nil, fmt.Errorf("cio ingest: %w", err)
	}
	metaAdInsight, err := metahdl.NewMetaAdInsightHandler()
	if err != nil {
		return nil, fmt.Errorf("cio ingest: %w", err)
	}
	return &CioIngestHandler{
		pcOrder:       pcOrder,
		pcShop:        pcShop,
		pcWarehouse:   pcWarehouse,
		pcProduct:     pcProduct,
		pcVariation:   pcVariation,
		pcCategory:    pcCategory,
		pcCustomer:    pcCustomer,
		fbConv:        fbConv,
		fbMsg:         fbMsg,
		fbCustomer:    fbCustomer,
		metaAdAccount: metaAdAccount,
		metaCampaign:  metaCampaign,
		metaAdSet:     metaAdSet,
		metaAd:        metaAd,
		metaAdInsight: metaAdInsight,
	}, nil
}

// canonicalCioDomain chuẩn hóa alias (message/conversation/pc_pos_*) về domain nội bộ thống nhất.
func canonicalCioDomain(domain string) string {
	switch domain {
	case "conversation":
		return "interaction_conversation"
	case "message":
		return "interaction_message"
	case "pc_pos_customer":
		return "pos_customer"
	default:
		return domain
	}
}

// permissionForCioDomain map domain → permission giống các route cũ (tránh nới quyền).
func permissionForCioDomain(domain string) string {
	domain = canonicalCioDomain(domain)
	switch domain {
	case "order":
		return "PcPosOrder.Update"
	case "pos_shop":
		return "PcPosShop.Update"
	case "pos_warehouse":
		return "PcPosWarehouse.Update"
	case "pos_product":
		return "PcPosProduct.Update"
	case "pos_variation":
		return "PcPosVariation.Update"
	case "pos_category":
		return "PcPosCategory.Update"
	case "pos_customer":
		return "PcPosCustomer.Update"
	case "interaction_conversation":
		return "FbConversation.Update"
	case "interaction_message":
		return "FbMessage.Update"
	case "fb_customer":
		return "FbCustomer.Update"
	case "meta_ad_account":
		return "MetaAdAccount.Update"
	case "meta_campaign":
		return "MetaCampaign.Update"
	case "meta_adset":
		return "MetaAdSet.Update"
	case "meta_ad":
		return "MetaAd.Update"
	case "meta_ad_insight":
		return "MetaAdInsight.Update"
	case "ads", "ads_sync", "meta_ads", "crm":
		return ""
	default:
		return ""
	}
}

func isSupportedCioDomain(domain string) bool {
	domain = canonicalCioDomain(domain)
	switch domain {
	case "order",
		"pos_shop", "pos_warehouse", "pos_product", "pos_variation", "pos_category", "pos_customer",
		"interaction_conversation", "interaction_message",
		"fb_customer",
		"ads", "ads_sync", "meta_ads", "crm",
		"meta_ad_account", "meta_campaign", "meta_adset", "meta_ad", "meta_ad_insight":
		return true
	default:
		return false
	}
}

// mergeIngestFilterMap trộn ?filter= (URL) với body.filter của CIO → map thô (chưa normalize).
func mergeIngestFilterMap(c fiber.Ctx, bodyFilter map[string]interface{}) (map[string]interface{}, error) {
	filterStr := c.Query("filter", "{}")
	var qFilter map[string]interface{}
	if err := json.Unmarshal([]byte(filterStr), &qFilter); err != nil {
		return nil, common.NewError(
			common.ErrCodeValidationFormat,
			fmt.Sprintf("Filter (query) không đúng định dạng JSON: %v. Giá trị nhận được: %s", err, filterStr),
			common.StatusBadRequest,
			err,
		)
	}
	return mergeStringKeyMaps(qFilter, bodyFilter), nil
}

func mergeStringKeyMaps(base, over map[string]interface{}) map[string]interface{} {
	out := make(map[string]interface{})
	for k, v := range base {
		out[k] = v
	}
	for k, v := range over {
		out[k] = v
	}
	return out
}

// HandleIngest POST /api/v1/cio/ingest — điểm vào duy nhất (CIO Hub).
func (h *CioIngestHandler) HandleIngest(c fiber.Ctx) error {
	c.Set("Content-Type", "application/json; charset=utf-8")

	var req CioIngestRequest
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return common.NewError(common.ErrCodeValidationFormat, "Body CIO ingest phải là JSON hợp lệ", common.StatusBadRequest, err)
	}
	rawDomain := strings.TrimSpace(strings.ToLower(req.Domain))
	domain := canonicalCioDomain(rawDomain)
	if domain == "" {
		return common.NewError(common.ErrCodeValidationFormat, "Thiếu field domain (vd: order, pos_shop, interaction_conversation, fb_customer, ...)", common.StatusBadRequest, nil)
	}
	if !isSupportedCioDomain(rawDomain) {
		return common.NewError(common.ErrCodeValidationFormat, "domain không hợp lệ", common.StatusBadRequest, nil)
	}

	perm := permissionForCioDomain(rawDomain)
	if !middleware.EnforceActiveRolePermission(c, perm) {
		return nil
	}

	if len(req.Data) == 0 && !domainAllowsEmptyData(domain) {
		return common.NewError(common.ErrCodeValidationFormat, "Thiếu field data cho domain này", common.StatusBadRequest, nil)
	}

	mergedFilter, err := mergeIngestFilterMap(c, req.Filter)
	if err != nil {
		return err
	}

	switch domain {
	case "order":
		filter, perr := h.pcOrder.ProcessMergedFilter(mergedFilter)
		if perr != nil {
			return perr
		}
		return h.pcOrder.SyncUpsertOneFromParts(c, filter, req.Data)
	case "pos_shop":
		filter, perr := h.pcShop.ProcessMergedFilter(mergedFilter)
		if perr != nil {
			return perr
		}
		return h.pcShop.SyncUpsertOneFromParts(c, filter, req.Data)
	case "pos_warehouse":
		filter, perr := h.pcWarehouse.ProcessMergedFilter(mergedFilter)
		if perr != nil {
			return perr
		}
		return h.pcWarehouse.SyncUpsertOneFromParts(c, filter, req.Data)
	case "pos_product":
		filter, perr := h.pcProduct.ProcessMergedFilter(mergedFilter)
		if perr != nil {
			return perr
		}
		return h.pcProduct.SyncUpsertOneFromParts(c, filter, req.Data)
	case "pos_variation":
		filter, perr := h.pcVariation.ProcessMergedFilter(mergedFilter)
		if perr != nil {
			return perr
		}
		return h.pcVariation.SyncUpsertOneFromParts(c, filter, req.Data)
	case "pos_category":
		filter, perr := h.pcCategory.ProcessMergedFilter(mergedFilter)
		if perr != nil {
			return perr
		}
		return h.pcCategory.SyncUpsertOneFromParts(c, filter, req.Data)
	case "pos_customer":
		filter, perr := h.pcCustomer.ProcessMergedFilter(mergedFilter)
		if perr != nil {
			return perr
		}
		return h.pcCustomer.SyncUpsertOneFromParts(c, filter, req.Data)
	case "interaction_conversation":
		filter, perr := h.fbConv.ProcessMergedFilter(mergedFilter)
		if perr != nil {
			return perr
		}
		return h.fbConv.SyncUpsertOneFromParts(c, filter, req.Data)
	case "interaction_message":
		return h.fbMsg.UpsertMessagesFromParts(c, req.Data)
	case "fb_customer":
		filter, perr := h.fbCustomer.ProcessMergedFilter(mergedFilter)
		if perr != nil {
			return perr
		}
		return h.fbCustomer.SyncUpsertOneFromParts(c, filter, req.Data)
	case "ads", "ads_sync", "meta_ads":
		return HandleIngestAdsStub(c)
	case "crm":
		return HandleIngestCrmStub(c)
	case "meta_ad_account":
		cioRewriteRequestBody(c, req.Data)
		return h.metaAdAccount.HandleSyncUpsertOne(c)
	case "meta_campaign":
		cioRewriteRequestBody(c, req.Data)
		return h.metaCampaign.HandleSyncUpsertOne(c)
	case "meta_adset":
		cioRewriteRequestBody(c, req.Data)
		return h.metaAdSet.HandleSyncUpsertOne(c)
	case "meta_ad":
		cioRewriteRequestBody(c, req.Data)
		return h.metaAd.HandleSyncUpsertOne(c)
	case "meta_ad_insight":
		cioRewriteRequestBody(c, req.Data)
		return h.metaAdInsight.HandleSyncUpsertOne(c)
	default:
		return common.NewError(common.ErrCodeValidationFormat, "domain không hỗ trợ", common.StatusBadRequest, nil)
	}
}

func domainAllowsEmptyData(domain string) bool {
	switch domain {
	case "ads", "ads_sync", "meta_ads", "crm":
		return true
	default:
		return false
	}
}

// cioRewriteRequestBody gán body thô cho request (Meta sync-upsert đọc trực tiếp JSON, không qua envelope CIO).
func cioRewriteRequestBody(c fiber.Ctx, data json.RawMessage) {
	if len(data) > 0 {
		c.Request().SetBody(append([]byte(nil), data...))
	} else {
		c.Request().ResetBody()
	}
}

// HandleIngestAdsStub — domain ads tổng hợp chưa triển khai; sync entity Meta dùng domain meta_* trong CIO.
func HandleIngestAdsStub(c fiber.Ctx) error {
	c.Set("Content-Type", "application/json; charset=utf-8")
	return c.Status(fiber.StatusNotImplemented).JSON(fiber.Map{
		"code":    fiber.StatusNotImplemented,
		"status":  "error",
		"message": "CIO ingest ads chưa triển khai — dùng các endpoint Meta/Ads sync hiện tại",
		"hint":    "POST /api/v1/cio/ingest với domain meta_ad_account | meta_campaign | meta_adset | meta_ad | meta_ad_insight",
	})
}

// HandleIngestCrmStub — Phase 1: CRM dùng bulk/sync hiện có.
func HandleIngestCrmStub(c fiber.Ctx) error {
	c.Set("Content-Type", "application/json; charset=utf-8")
	return c.Status(fiber.StatusNotImplemented).JSON(fiber.Map{
		"code":    fiber.StatusNotImplemented,
		"status":  "error",
		"message": "CIO ingest CRM chưa triển khai — dùng CRM bulk / pending ingest",
		"hint":    "POST /api/v1/crm-bulk-jobs, crm-pending-merge, ...",
	})
}
