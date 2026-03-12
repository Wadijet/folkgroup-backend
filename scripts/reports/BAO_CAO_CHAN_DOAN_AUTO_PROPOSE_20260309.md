# BÁO CÁO CHẨN ĐOÁN AUTO-PROPOSE

**Ngày tạo:** 2026-03-09 23:44  
**Database:** folkform_auth

---

## 1. ADS_META_CONFIG

Tổng documents: **2**

Configs có autoProposeEnabled (true hoặc không set): **2**

| adAccountId | ownerOrganizationId | autoProposeEnabled |
|-------------|---------------------|--------------------|
| act_1155651358104839 | 69a655f0088600c32e62f955 | true |
| act_377880728187652 | 69a655f0088600c32e62f955 | true |

## 2. META_CAMPAIGNS

Tổng campaigns: **16**

Campaigns ACTIVE (effectiveStatus hoặc status): **12**

Campaigns có currentMetrics.alertFlags (ít nhất 1 flag): **16**

### Mẫu campaigns có alertFlags (tối đa 10)

| campaignId | adAccountId | name | ownerOrgId | status | alertFlags |
|------------|-------------|------|------------|--------|------------|
| 120238743299300705 | 377880728187652 | FolkForm__PAGE-02__C... | 69a655f0... | ACTIVE | [cpm_low chs_warning] |
| 120238591027920705 | 377880728187652 | FolkForm__PAGE-01__C... | 69a655f0... | ACTIVE | [cpa_mess_high msg_rate_low chs_warning ... |
| 120238529856460705 | 377880728187652 | FolkForm__PAGE-01__0... | 69a655f0... | ACTIVE | [cpa_mess_high msg_rate_low chs_warning ... |
| 120238356155560705 | 377880728187652 | (Value) FolkForm__PA... | 69a655f0... | PAUSED | [cpa_mess_high cpa_purchase_high chs_cri... |
| 120238218350970705 | 377880728187652 | FolkForm__PAGE-07__C... | 69a655f0... | ACTIVE | [cpa_purchase_high cpm_high chs_warning ... |
| 120238217484910705 | 377880728187652 | FolkForm__PAGE-01__C... | 69a655f0... | PAUSED | [cpa_purchase_high chs_warning] |
| 120238210602000705 | 377880728187652 | FolkForm__PAGE-01__C... | 69a655f0... | ACTIVE | [cpa_purchase_high chs_warning] |
| 120238184613370705 | 377880728187652 | FolkForm__PAGE-02__C... | 69a655f0... | ACTIVE | [chs_warning] |
| 120238184460180705 | 377880728187652 | FolkForm__PAGE-02__C... | 69a655f0... | PAUSED | [cpa_purchase_high chs_warning portfolio... |
| 120237644845720705 | 377880728187652 | FolkForm__PAGE-01__0... | 69a655f0... | ACTIVE | [cpa_mess_high msg_rate_low cpm_low chs_... |

## 3. CHUỖI ĐIỀU KIỆN GetCampaignsForAutoPropose

**Bước 1:** Configs có autoProposeEnabled: **2**

**Bước 2:** Campaigns thỏa TẤT CẢ điều kiện (adAccountId in configs, ownerOrgId in configs, ACTIVE, có alertFlags): **12**

✅ Có campaigns đủ điều kiện để auto-propose. Nếu vẫn không tạo action, kiểm tra:
- Rule evaluation: alertFlags có trigger rule nào không (Kill/Decrease/Increase)
- ShouldAutoPropose: rule có autoPropose = true trong ActionRuleConfig
- HasPendingProposalForCampaign: đã có pending cho campaign chưa

## 4. ACTION_PENDING_APPROVAL

| Loại | Số lượng |
|------|----------|
| Tổng | 0 |
| domain=ads | 0 |
| domain=ads, status=pending | 0 |

## 5. KẾT LUẬN VÀ GỢI Ý

### Checklist

- [ ] ads_meta_config có config với autoProposeEnabled: **2** configs
- [ ] meta_campaigns có alertFlags: **16** campaigns
- [ ] Campaigns đủ điều kiện (config + ACTIVE + alertFlags): **12**

### 🔧 Nếu vẫn không tạo action

1. **Rule không trigger:** alertFlags có thể không match bất kỳ rule Kill/Decrease/Increase nào. Kiểm tra definitions và threshold.
2. **autoPropose = false:** ActionRuleConfig có thể tắt autoPropose cho rule tương ứng.
3. **Đã có pending:** action_pending_approval có thể đã có bản ghi pending cho campaign — hệ thống tránh duplicate.

