// Package client - HTTP client gọi Meta Graph API (Marketing API).
package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	// GraphAPIBase URL base cho Meta Graph API
	GraphAPIBase = "https://graph.facebook.com"
	// GraphAPIVersion phiên bản API (v21.0 ổn định)
	GraphAPIVersion = "v21.0"

	// MetaFieldsAdAccount fields tối đa cho Ad Account (càng nhiều càng tốt).
	// Bỏ has_page_authorized_adaccount — field này yêu cầu page_id khi gọi me/adaccounts
	MetaFieldsAdAccount = "id,name,account_id,account_status,age,amount_spent,balance,business_name,business_city,business_country_code,business_state,business_street,business_street2,business_zip,created_time,currency,default_dsa_beneficiary,default_dsa_payor,disable_reason,end_advertiser,end_advertiser_name,has_migrated_permissions,io_number,is_attribution_spec_system_default,is_direct_deals_enabled,is_in_3ds_authorization_enabled_market,is_notifications_enabled,is_personal,is_prepay_account,is_tax_id_required,media_agency,min_campaign_group_spend_cap,min_daily_budget,offsite_pixels_tos_accepted,opportunity_score,owner,partner,spend_cap,tax_id,tax_id_status,tax_id_type,timezone_id,timezone_name,timezone_offset_hours_utc,business{id,name}"

	// MetaFieldsCampaign fields tối đa cho Campaign.
	MetaFieldsCampaign = "id,name,account_id,adlabels,bid_strategy,boosted_object_id,budget_remaining,buying_type,campaign_group_active_time,can_create_brand_lift_study,can_use_spend_cap,configured_status,created_time,daily_budget,effective_status,has_secondary_skadnetwork_reporting,is_adset_budget_sharing_enabled,is_budget_schedule_enabled,is_skadnetwork_attribution,issues_info,last_budget_toggling_time,lifetime_budget,objective,pacing_type,primary_attribution,promoted_object,smart_promotion_type,source_campaign_id,special_ad_categories,special_ad_category,special_ad_category_country,spend_cap,start_time,status,updated_time"

	// MetaFieldsAdSet fields tối đa cho AdSet.
	// Bỏ contextual_bundling_spec — yêu cầu GK contextual_bundle_test_api_accounts
	// Bỏ name trùng (đã có ở đầu)
	MetaFieldsAdSet = "id,name,account_id,adlabels,adset_schedule,asset_feed_id,attribution_spec,bid_adjustments,bid_amount,bid_constraints,bid_info,bid_strategy,billing_event,brand_safety_config,budget_remaining,campaign_active_time,campaign_attribution,campaign_id,configured_status,created_time,creative_sequence,daily_budget,daily_min_spend_target,daily_spend_cap,destination_type,dsa_beneficiary,dsa_payor,effective_status,end_time,frequency_control_specs,instagram_user_id,is_dynamic_creative,is_incremental_attribution_enabled,issues_info,learning_stage_info,lifetime_budget,lifetime_imps,lifetime_min_spend_target,lifetime_spend_cap,min_budget_spend_percentage,multi_optimization_goal_weight,optimization_goal,optimization_sub_event,pacing_type,promoted_object,recurring_budget_semantics,source_adset_id,start_time,status,targeting,updated_time"

	// MetaFieldsAd fields tối đa cho Ad.
	// Bỏ name trùng (đã có ở đầu và trong creative{id,name})
	MetaFieldsAd = "id,name,account_id,ad_active_time,ad_review_feedback,ad_schedule_end_time,ad_schedule_start_time,adlabels,adset_id,bid_amount,campaign_id,configured_status,conversion_domain,created_time,creative{id,name},effective_status,issues_info,last_updated_by_app_id,preview_shareable_link,source_ad_id,status,tracking_specs,updated_time"

	// MetaFieldsInsights fields tối đa cho Insights (không dùng breakdown).
	// Lưu ý: Nhiều field phụ thuộc breakdown; Meta bỏ qua field không khả dụng thay vì lỗi.
	// Bỏ activity_recency — Meta API trả lỗi 100: không hợp lệ cho fields param.
	MetaFieldsInsights = "account_currency,account_id,account_name,actions,action_values,ad_click_actions,ad_id,ad_impression_actions,ad_name,adset_id,adset_name,attribution_setting,auction_bid,auction_competitiveness,auction_max_competitor_bid,campaign_id,campaign_name,canvas_avg_view_percent,canvas_avg_view_time,clicks,conversion_values,conversions,cost_per_action_type,cost_per_ad_click,cost_per_conversion,cost_per_inline_link_click,cost_per_inline_post_engagement,cost_per_outbound_click,cost_per_unique_click,cost_per_unique_inline_link_click,cost_per_unique_outbound_click,cpc,cpm,ctr,date_start,date_stop,frequency,impressions,inline_link_click_ctr,inline_link_clicks,inline_post_engagement,reach,spend,unique_actions,unique_clicks,unique_inline_link_clicks,unique_outbound_clicks,video_avg_time_watched_actions,video_p25_watched_actions,video_p50_watched_actions,video_p75_watched_actions,video_p100_watched_actions"
)

// MetaGraphClient client gọi Meta Graph API.
type MetaGraphClient struct {
	httpClient  *http.Client
	accessToken string
	baseURL     string
}

// MetaAPIError lỗi trả về từ Meta API.
type MetaAPIError struct {
	Message      string `json:"message"`
	Type         string `json:"type"`
	Code         int    `json:"code"`
	ErrorSubcode int    `json:"error_subcode"`
	FBTraceID    string `json:"fbtrace_id"`
}

// MetaErrorResponse wrapper cho error response.
type MetaErrorResponse struct {
	Error MetaAPIError `json:"error"`
}

// MetaRateLimitError lỗi rate limit (429 hoặc error 17/613) từ Meta API.
// Cho phép caller detect và retry với backoff.
type MetaRateLimitError struct {
	Message    string
	RetryAfter time.Duration // Thời gian nên chờ trước khi retry (từ header Retry-After hoặc 0)
}

func (e *MetaRateLimitError) Error() string {
	if e.RetryAfter > 0 {
		return e.Message + " (Retry-After: " + e.RetryAfter.String() + ")"
	}
	return e.Message
}

// MetaUsageInfo thông tin sử dụng rate limit từ response headers của Meta.
// X-Ad-Account-Usage: acc_id_util_pct (0-100), reset_time_duration (giây), ads_api_access_tier.
// Dùng để theo dõi điểm và điều chỉnh throttle.
type MetaUsageInfo struct {
	// AccIDUtilPct % sử dụng điểm ad account (0-100). Throttling thường bắt đầu ~75%.
	AccIDUtilPct float64
	// ResetTimeDuration giây còn lại trước khi điểm decay/reset.
	ResetTimeDuration int
	// AdsAPIAccessTier "development" (60 điểm) hoặc "standard" (9000 điểm).
	AdsAPIAccessTier string
}

// MetaAPIResponse response đầy đủ từ Meta API (body + usage headers).
type MetaAPIResponse struct {
	Body  []byte
	Usage *MetaUsageInfo
}

// PagingResponse phân trang từ Meta API.
type PagingResponse struct {
	Cursors struct {
		Before string `json:"before"`
		After  string `json:"after"`
	} `json:"cursors"`
	Next string `json:"next"`
}

// MetaFieldDef định nghĩa một field từ metadata (metadata=1).
type MetaFieldDef struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Type        string `json:"type"`
}

// MetaMetadata metadata trả về khi gọi node với metadata=1.
// Cho phép discovery danh sách fields có sẵn cho object type đó.
// Lưu ý: Chỉ hoạt động với node (object đơn), không hoạt động với edge (collection).
type MetaMetadata struct {
	Metadata struct {
		Fields []MetaFieldDef `json:"fields"`
	} `json:"metadata"`
}

// NewMetaGraphClient tạo client mới.
// accessToken: User token hoặc System User token với ads_read, ads_management.
func NewMetaGraphClient(accessToken string) *MetaGraphClient {
	if accessToken == "" {
		return nil
	}
	return &MetaGraphClient{
		httpClient:  &http.Client{Timeout: 30 * time.Second},
		accessToken: accessToken,
		baseURL:     GraphAPIBase + "/" + GraphAPIVersion,
	}
}

// getInternal thực hiện GET request và trả về body + headers.
func (c *MetaGraphClient) getInternal(ctx context.Context, path string, params map[string]string) ([]byte, http.Header, error) {
	if c == nil || c.accessToken == "" {
		return nil, nil, fmt.Errorf("meta client chưa được cấu hình access token")
	}
	u := c.baseURL + "/" + path
	vals := url.Values{}
	vals.Set("access_token", c.accessToken)
	for k, v := range params {
		vals.Set(k, v)
	}
	fullURL := u + "?" + vals.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fullURL, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("tạo request thất bại: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("gọi Meta API thất bại: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, fmt.Errorf("đọc response thất bại: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		msg := string(body)
		var errResp MetaErrorResponse
		if json.Unmarshal(body, &errResp) == nil && errResp.Error.Message != "" {
			msg = errResp.Error.Message
		}
		// 429 hoặc error 17/613 (ad account rate limit) → MetaRateLimitError
		if resp.StatusCode == http.StatusTooManyRequests {
			retryAfter := parseRetryAfter(resp.Header.Get("Retry-After"))
			return nil, nil, &MetaRateLimitError{Message: fmt.Sprintf("Meta API rate limit (429): %s", msg), RetryAfter: retryAfter}
		}
		if resp.StatusCode == http.StatusBadRequest && isAdAccountRateLimitError(errResp.Error.Code, errResp.Error.ErrorSubcode) {
			retryAfter := parseRetryAfter(resp.Header.Get("Retry-After"))
			if retryAfter == 0 {
				usage := parseAdAccountUsage(resp.Header.Get("X-Ad-Account-Usage"))
				if usage != nil && usage.ResetTimeDuration > 0 {
					retryAfter = time.Duration(usage.ResetTimeDuration) * time.Second
				} else if usage != nil && usage.AdsAPIAccessTier == "development" {
					retryAfter = 300 * time.Second // Dev tier: block 300s
				} else {
					retryAfter = 60 * time.Second // Standard: block 60s
				}
			}
			return nil, nil, &MetaRateLimitError{Message: fmt.Sprintf("Meta API rate limit (code %d): %s", errResp.Error.Code, msg), RetryAfter: retryAfter}
		}
		return nil, nil, fmt.Errorf("Meta API lỗi %d: %s", resp.StatusCode, msg)
	}

	return body, resp.Header.Clone(), nil
}

// isAdAccountRateLimitError kiểm tra error code 17 (subcode 2446079) hoặc 613 (subcode 1487742 hoặc null).
// Meta doc: 17/2446079 "User request limit reached", 613/1487742 "too many calls from this ad-account".
func isAdAccountRateLimitError(code, subcode int) bool {
	if code == 17 && subcode == 2446079 {
		return true
	}
	if code == 613 {
		return subcode == 1487742 || subcode == 0 // 0 khi Meta không trả subcode
	}
	return false
}

// parseAdAccountUsage parse header X-Ad-Account-Usage (JSON: acc_id_util_pct, reset_time_duration, ads_api_access_tier).
// Hỗ trợ format trực tiếp hoặc nested theo ad account id (act_xxx).
func parseAdAccountUsage(header string) *MetaUsageInfo {
	if header == "" {
		return nil
	}
	var raw map[string]interface{}
	if err := json.Unmarshal([]byte(header), &raw); err != nil {
		return nil
	}
	// Nếu có key act_xxx (nested theo ad account), lấy object đầu tiên
	if len(raw) == 1 {
		for _, v := range raw {
			if m, ok := v.(map[string]interface{}); ok {
				raw = m
				break
			}
		}
	}
	info := &MetaUsageInfo{}
	if s, ok := raw["ads_api_access_tier"].(string); ok {
		info.AdsAPIAccessTier = s
	}
	if v := raw["acc_id_util_pct"]; v != nil {
		switch x := v.(type) {
		case float64:
			info.AccIDUtilPct = x
		case int:
			info.AccIDUtilPct = float64(x)
		}
	}
	if v := raw["reset_time_duration"]; v != nil {
		switch x := v.(type) {
		case float64:
			info.ResetTimeDuration = int(x)
		case int:
			info.ResetTimeDuration = x
		}
	}
	return info
}

// Get thực hiện GET request tới Meta Graph API.
// path: đường dẫn (vd: "me/adaccounts", "act_123/campaigns").
// params: query params (vd: map[string]string{"fields": "id,name", "limit": "100"}).
func (c *MetaGraphClient) Get(ctx context.Context, path string, params map[string]string) ([]byte, error) {
	body, _, err := c.getInternal(ctx, path, params)
	return body, err
}

// Post thực hiện POST request tới Meta Graph API (form-urlencoded).
// path: node ID (vd: "123456" cho campaign, adset, ad).
// params: form fields (vd: map[string]string{"status": "PAUSED", "daily_budget": "10000"}).
// Meta Marketing API: update status, budget, v.v.
func (c *MetaGraphClient) Post(ctx context.Context, path string, params map[string]string) ([]byte, error) {
	if c == nil || c.accessToken == "" {
		return nil, fmt.Errorf("meta client chưa được cấu hình access token")
	}
	u := c.baseURL + "/" + path
	vals := url.Values{}
	vals.Set("access_token", c.accessToken)
	for k, v := range params {
		vals.Set(k, v)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, strings.NewReader(vals.Encode()))
	if err != nil {
		return nil, fmt.Errorf("tạo request thất bại: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("gọi Meta API thất bại: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("đọc response thất bại: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		msg := string(body)
		var errResp MetaErrorResponse
		if json.Unmarshal(body, &errResp) == nil && errResp.Error.Message != "" {
			msg = errResp.Error.Message
		}
		if resp.StatusCode == http.StatusTooManyRequests {
			retryAfter := parseRetryAfter(resp.Header.Get("Retry-After"))
			return nil, &MetaRateLimitError{Message: fmt.Sprintf("Meta API rate limit (429): %s", msg), RetryAfter: retryAfter}
		}
		if resp.StatusCode == http.StatusBadRequest && isAdAccountRateLimitError(errResp.Error.Code, errResp.Error.ErrorSubcode) {
			retryAfter := parseRetryAfter(resp.Header.Get("Retry-After"))
			if retryAfter == 0 {
				usage := parseAdAccountUsage(resp.Header.Get("X-Ad-Account-Usage"))
				if usage != nil && usage.ResetTimeDuration > 0 {
					retryAfter = time.Duration(usage.ResetTimeDuration) * time.Second
				} else if usage != nil && usage.AdsAPIAccessTier == "development" {
					retryAfter = 300 * time.Second
				} else {
					retryAfter = 60 * time.Second
				}
			}
			return nil, &MetaRateLimitError{Message: fmt.Sprintf("Meta API rate limit (code %d): %s", errResp.Error.Code, msg), RetryAfter: retryAfter}
		}
		return nil, fmt.Errorf("Meta API lỗi %d: %s", resp.StatusCode, msg)
	}
	return body, nil
}

// GetWithResponse thực hiện GET và trả về body + usage từ headers (X-Ad-Account-Usage).
// Dùng khi cần theo dõi điểm rate limit.
func (c *MetaGraphClient) GetWithResponse(ctx context.Context, path string, params map[string]string) (*MetaAPIResponse, error) {
	body, headers, err := c.getInternal(ctx, path, params)
	if err != nil {
		return nil, err
	}
	resp := &MetaAPIResponse{Body: body}
	if h := headers.Get("X-Ad-Account-Usage"); h != "" {
		resp.Usage = parseAdAccountUsage(h)
	}
	return resp, nil
}

// GetMetadata lấy metadata (danh sách fields có sẵn) cho một node.
// nodeID: act_123 (ad account), hoặc campaign_id, adset_id, ad_id (số thuần).
// Meta API không hỗ trợ fields=*; metadata=1 là cách duy nhất để auto-discovery fields.
// Chỉ hoạt động với node đơn, không hoạt động với edge (vd: act_123/campaigns).
// Trả về danh sách field names để dùng trong tham số fields của request tiếp theo.
func (c *MetaGraphClient) GetMetadata(ctx context.Context, nodeID string) (*MetaMetadata, error) {
	params := map[string]string{"metadata": "1"}
	body, _, err := c.getInternal(ctx, nodeID, params)
	if err != nil {
		return nil, err
	}
	var meta MetaMetadata
	if err := json.Unmarshal(body, &meta); err != nil {
		return nil, fmt.Errorf("parse metadata response: %w", err)
	}
	return &meta, nil
}

// FieldsFromMetadata trích danh sách field names từ metadata, nối thành chuỗi dùng cho params fields.
func FieldsFromMetadata(meta *MetaMetadata) string {
	if meta == nil || len(meta.Metadata.Fields) == 0 {
		return ""
	}
	names := make([]string, 0, len(meta.Metadata.Fields))
	seen := make(map[string]bool)
	for _, f := range meta.Metadata.Fields {
		if f.Name != "" && !seen[f.Name] {
			names = append(names, f.Name)
			seen[f.Name] = true
		}
	}
	if len(names) == 0 {
		return ""
	}
	return strings.Join(names, ",")
}

// GetAdAccount lấy chi tiết một ad account (act_xxx). Dùng để sync metaData.
func (c *MetaGraphClient) GetAdAccount(ctx context.Context, adAccountID string, fields string) ([]byte, error) {
	if fields == "" {
		fields = MetaFieldsAdAccount
	}
	params := map[string]string{"fields": fields}
	return c.Get(ctx, adAccountID, params)
}

// GetAdAccountWithResponse giống GetAdAccount nhưng trả về usage headers (cho throttle).
func (c *MetaGraphClient) GetAdAccountWithResponse(ctx context.Context, adAccountID string, fields string) (*MetaAPIResponse, error) {
	if fields == "" {
		fields = MetaFieldsAdAccount
	}
	params := map[string]string{"fields": fields}
	return c.GetWithResponse(ctx, adAccountID, params)
}

// GetAdAccounts lấy danh sách ad accounts của user.
// Trả về response raw (có data, paging).
func (c *MetaGraphClient) GetAdAccounts(ctx context.Context, fields string, limit int) ([]byte, error) {
	params := map[string]string{}
	if fields != "" {
		params["fields"] = fields
	} else {
		params["fields"] = MetaFieldsAdAccount
	}
	if limit > 0 {
		params["limit"] = fmt.Sprintf("%d", limit)
	}
	return c.Get(ctx, "me/adaccounts", params)
}

// GetAdAccountsWithResponse giống GetAdAccounts nhưng trả về usage headers.
func (c *MetaGraphClient) GetAdAccountsWithResponse(ctx context.Context, fields string, limit int) (*MetaAPIResponse, error) {
	params := map[string]string{}
	if fields != "" {
		params["fields"] = fields
	} else {
		params["fields"] = MetaFieldsAdAccount
	}
	if limit > 0 {
		params["limit"] = fmt.Sprintf("%d", limit)
	}
	return c.GetWithResponse(ctx, "me/adaccounts", params)
}

// GetCampaigns lấy campaigns của ad account. after: cursor trang tiếp (rỗng = trang đầu).
func (c *MetaGraphClient) GetCampaigns(ctx context.Context, adAccountID string, fields string, limit int, after string) ([]byte, error) {
	params := map[string]string{}
	if fields != "" {
		params["fields"] = fields
	} else {
		params["fields"] = MetaFieldsCampaign
	}
	if limit > 0 {
		params["limit"] = fmt.Sprintf("%d", limit)
	}
	if after != "" {
		params["after"] = after
	}
	return c.Get(ctx, adAccountID+"/campaigns", params)
}

// GetCampaignsWithResponse giống GetCampaigns nhưng trả về usage headers.
func (c *MetaGraphClient) GetCampaignsWithResponse(ctx context.Context, adAccountID string, fields string, limit int, after string) (*MetaAPIResponse, error) {
	params := map[string]string{}
	if fields != "" {
		params["fields"] = fields
	} else {
		params["fields"] = MetaFieldsCampaign
	}
	if limit > 0 {
		params["limit"] = fmt.Sprintf("%d", limit)
	}
	if after != "" {
		params["after"] = after
	}
	return c.GetWithResponse(ctx, adAccountID+"/campaigns", params)
}

// GetAdSets lấy ad sets của ad account hoặc campaign. after: cursor trang tiếp.
func (c *MetaGraphClient) GetAdSets(ctx context.Context, objectID string, fields string, limit int, after string) ([]byte, error) {
	params := map[string]string{}
	if fields != "" {
		params["fields"] = fields
	} else {
		params["fields"] = MetaFieldsAdSet
	}
	if limit > 0 {
		params["limit"] = fmt.Sprintf("%d", limit)
	}
	if after != "" {
		params["after"] = after
	}
	return c.Get(ctx, objectID+"/adsets", params)
}

// GetAdSetsWithResponse giống GetAdSets nhưng trả về usage headers.
func (c *MetaGraphClient) GetAdSetsWithResponse(ctx context.Context, objectID string, fields string, limit int, after string) (*MetaAPIResponse, error) {
	params := map[string]string{}
	if fields != "" {
		params["fields"] = fields
	} else {
		params["fields"] = MetaFieldsAdSet
	}
	if limit > 0 {
		params["limit"] = fmt.Sprintf("%d", limit)
	}
	if after != "" {
		params["after"] = after
	}
	return c.GetWithResponse(ctx, objectID+"/adsets", params)
}

// GetAds lấy ads của ad account, ad set, hoặc campaign. after: cursor trang tiếp.
func (c *MetaGraphClient) GetAds(ctx context.Context, objectID string, fields string, limit int, after string) ([]byte, error) {
	params := map[string]string{}
	if fields != "" {
		params["fields"] = fields
	} else {
		params["fields"] = MetaFieldsAd
	}
	if limit > 0 {
		params["limit"] = fmt.Sprintf("%d", limit)
	}
	if after != "" {
		params["after"] = after
	}
	return c.Get(ctx, objectID+"/ads", params)
}

// GetAdsWithResponse giống GetAds nhưng trả về usage headers.
func (c *MetaGraphClient) GetAdsWithResponse(ctx context.Context, objectID string, fields string, limit int, after string) (*MetaAPIResponse, error) {
	params := map[string]string{}
	if fields != "" {
		params["fields"] = fields
	} else {
		params["fields"] = MetaFieldsAd
	}
	if limit > 0 {
		params["limit"] = fmt.Sprintf("%d", limit)
	}
	if after != "" {
		params["after"] = after
	}
	return c.GetWithResponse(ctx, objectID+"/ads", params)
}

// GetInsights lấy insights (hiệu suất) cho object. after: cursor trang tiếp.
// datePreset: "last_7d", "last_30d", "today", "yesterday", ...
// level: "account", "campaign", "adset", "ad"
// timeIncrement: 1 = breakdown theo ngày (date_start, date_stop theo từng ngày). 0 = tổng hợp.
func (c *MetaGraphClient) GetInsights(ctx context.Context, objectID string, datePreset string, level string, fields string, after string, timeIncrement int) ([]byte, error) {
	params := map[string]string{
		"date_preset": datePreset,
	}
	if timeIncrement > 0 {
		params["time_increment"] = fmt.Sprintf("%d", timeIncrement)
	}
	if level != "" {
		params["level"] = level
	}
	if fields != "" {
		params["fields"] = fields
	} else {
		params["fields"] = MetaFieldsInsights
	}
	if after != "" {
		params["after"] = after
	}
	return c.Get(ctx, objectID+"/insights", params)
}

// GetInsightsWithResponse giống GetInsights nhưng trả về usage headers.
func (c *MetaGraphClient) GetInsightsWithResponse(ctx context.Context, objectID string, datePreset string, level string, fields string, after string, timeIncrement int) (*MetaAPIResponse, error) {
	params := map[string]string{"date_preset": datePreset}
	if timeIncrement > 0 {
		params["time_increment"] = fmt.Sprintf("%d", timeIncrement)
	}
	if level != "" {
		params["level"] = level
	}
	if fields != "" {
		params["fields"] = fields
	} else {
		params["fields"] = MetaFieldsInsights
	}
	if after != "" {
		params["after"] = after
	}
	return c.GetWithResponse(ctx, objectID+"/insights", params)
}

// ExchangeShortForLongLived đổi short-lived token (~2h) sang long-lived token (~60 ngày).
// Gọi server-side với app_id, app_secret. Không dùng access_token của client.
// Trả về: accessToken, expiresIn (giây), error.
func ExchangeShortForLongLived(ctx context.Context, appID, appSecret, shortLivedToken string) (accessToken string, expiresIn int, err error) {
	if appID == "" || appSecret == "" || shortLivedToken == "" {
		return "", 0, fmt.Errorf("cần app_id, app_secret và short_lived_token")
	}
	u := GraphAPIBase + "/" + GraphAPIVersion + "/oauth/access_token"
	vals := url.Values{}
	vals.Set("grant_type", "fb_exchange_token")
	vals.Set("client_id", appID)
	vals.Set("client_secret", appSecret)
	vals.Set("fb_exchange_token", shortLivedToken)
	fullURL := u + "?" + vals.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fullURL, nil)
	if err != nil {
		return "", 0, fmt.Errorf("tạo request: %w", err)
	}
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", 0, fmt.Errorf("gọi Meta oauth: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", 0, fmt.Errorf("đọc response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		var errResp MetaErrorResponse
		if json.Unmarshal(body, &errResp) == nil && errResp.Error.Message != "" {
			return "", 0, fmt.Errorf("Meta API lỗi %d: %s", resp.StatusCode, errResp.Error.Message)
		}
		return "", 0, fmt.Errorf("Meta API lỗi %d: %s", resp.StatusCode, string(body))
	}
	var result struct {
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", 0, fmt.Errorf("parse response: %w", err)
	}
	if result.AccessToken == "" {
		return "", 0, fmt.Errorf("Meta không trả access_token")
	}
	return result.AccessToken, result.ExpiresIn, nil
}

// IsRateLimitError kiểm tra lỗi có phải rate limit (429 hoặc 17/613) không.
func IsRateLimitError(err error) bool {
	var rle *MetaRateLimitError
	return errors.As(err, &rle)
}

// parseRetryAfter parse header Retry-After (số giây hoặc HTTP-date).
// Trả về 0 nếu không parse được.
func parseRetryAfter(s string) time.Duration {
	if s == "" {
		return 0
	}
	if sec, err := strconv.Atoi(s); err == nil && sec > 0 {
		return time.Duration(sec) * time.Second
	}
	// HTTP-date format - đơn giản hóa: bỏ qua, trả về 0
	return 0
}
