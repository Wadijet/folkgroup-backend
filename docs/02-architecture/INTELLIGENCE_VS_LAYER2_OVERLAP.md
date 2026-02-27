# So sánh Intelligence Layer (Lớp 3) vs Lớp 2 (Customer Segmentation)

> Lớp 2 gồm 5 trục: **CHANNEL | VALUE | LIFECYCLE | LOYALTY | MOMENTUM**
> Lớp 3 (Intelligence) bổ sung tiêu chí mới theo stage. **Tiêu chí trùng Lớp 2 đã được bỏ** — dùng trực tiếp từ Lớp 2.

---

## Bảng trùng lặp / liên quan

| Nhóm | Tiêu chí Intelligence | Trùng / Liên quan Lớp 2? | Chi tiết |
|------|------------------------|---------------------------|----------|
| **First** | purchaseQuality | ❌ Không trùng | L2 Value dùng totalSpent; First dùng AvgOrderValue (AOV đơn đầu) |
| **First** | experienceQuality | ❌ Không trùng | L2 không có CancelledOrderCount |
| **First** | engagementAfterPurchase | ❌ Không trùng | L2 không có engagement/conversation |
| **First** | reorderTiming | ⚠️ **Liên quan** | Cùng dùng days_since_last như L2 Lifecycle, nhưng ngưỡng khác (7, 60 vs 30, 90, 180) |
| **First** | repeatProbability | ❌ Không trùng | Composite từ 4 chiều trên |
| **Repeat** | repeatDepth (R1-R4) | ⚠️ **Liên quan** | Dùng orderCount — L2 Loyalty cũng dùng (core≥5, repeat≥2) nhưng bucket khác |
| **Repeat** | repeatFrequency | ⚠️ **Liên quan** | Cùng recency/lifecycle — ngưỡng động (avg_days giữa đơn) vs L2 cố định |
| **Repeat** | spendMomentum | ⚠️ **Liên quan** | L2 Momentum dùng revenue_last_30d/90d; Repeat dùng AOV gần nhất vs AOV TB |
| **Repeat** | productExpansion | ❌ Không trùng | L2 không có OwnedSkuCount/category |
| **Repeat** | emotionalEngagement | ❌ Không trùng | L2 không có engagement |
| **Repeat** | upgradePotential | ❌ Không trùng | Composite |
| **VIP** | statusHealth | ✅ **TRÙNG** | Map trực tiếp từ `LifecycleStage` (active→vip_active, cooling→vip_cooling, inactive→vip_at_risk, dead→vip_inactive) |
| **VIP** | vipDepth | ⚠️ **Liên quan** | Dùng orderCount giống L2 Loyalty; bucket khác (8-12, 13-25, 26-40, 40+) |
| **VIP** | spendTrend | ⚠️ **Liên quan** | Tương tự L2 Momentum (AOV ratio vs revenue ratio) |
| **VIP** | productDiversity | ❌ Không trùng | L2 không có |
| **VIP** | engagementLevel | ❌ Không trùng | L2 không có |
| **VIP** | riskScore | ❌ Không trùng | Composite (dùng statusHealth, spendTrend, engagement) |
| **Inactive** | valueTier | ✅ **TRÙNG** | Map trực tiếp từ `ValueTier` (vip→vip_inactive, high→high_inactive, …) |
| **Inactive** | inactiveDuration | ✅ **TRÙNG** | Map từ `LifecycleStage` + DaysSinceLast (cooling 30-90, inactive 90-180, dead 180+) — chi tiết hơn L2 |
| **Inactive** | previousBehavior | ✅ **TRÙNG** | Map từ `LoyaltyStage` + OrderCount (core→was_vip, repeat→was_repeat, one_time→was_first_only) |
| **Inactive** | spendMomentumBefore | ✅ **TRÙNG** | Map trực tiếp từ `MomentumStage` (declining→downscaling, lost→sudden_drop, stable→stable) |
| **Inactive** | engagementDrop | ❌ Không trùng | L2 không có |
| **Inactive** | reactivationPotential | ❌ Không trùng | Composite |

---

## Tóm tắt

### ✅ Đã bỏ — dùng Lớp 2 (cập nhật 2025)

| Nhóm | Tiêu chí đã bỏ | Dùng thay thế |
|------|----------------|---------------|
| VIP | statusHealth | `lifecycleStage` (Lớp 2) |
| Inactive | valueTier | `valueTier` (Lớp 2) |
| Inactive | inactiveDuration | `lifecycleStage` (Lớp 2) |
| Inactive | previousBehavior | `loyaltyStage` + `orderCount` (Lớp 2) |
| Inactive | spendMomentumBefore | `momentumStage` (Lớp 2) |

→ **Đã triển khai**: Code và DTO đã cập nhật — các tiêu chí trên không còn trong Lớp 3. Dùng trực tiếp field Lớp 2 từ `CustomerItem`.

### ⚠️ Liên quan (cùng nguồn dữ liệu, khác ngưỡng/logic)

- **reorderTiming** (First) vs Lifecycle
- **repeatDepth** (Repeat) vs Loyalty
- **repeatFrequency** (Repeat) vs Lifecycle
- **spendMomentum** (Repeat), **spendTrend** (VIP) vs Momentum

→ Khác mục đích: L2 dùng để segment; Intelligence dùng để phân tích hành vi chi tiết theo stage. **Giữ cả hai**.

### ❌ Không trùng (mới, không có trong L2)

- engagement (First, Repeat, VIP, Inactive)
- productExpansion / productDiversity
- experienceQuality (cancelled)
- purchaseQuality (AOV đơn đầu)
- Các composite (repeatProbability, upgradePotential, riskScore, reactivationPotential)

---

## Trạng thái triển khai (đã cập nhật)

1. **VIP**: Đã bỏ statusHealth — dùng lifecycleStage (Lớp 2).
2. **Inactive**: Đã bỏ valueTier, inactiveDuration, previousBehavior, spendMomentumBefore — dùng Lớp 2.
3. Các tiêu chí liên quan “liên quan” — chúng bổ sung góc nhìn chi tiết cho từng stage.
