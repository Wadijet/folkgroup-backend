# Đề xuất tối ưu Agent Activity Log

## 1. Vấn đề hiện tại

### 1.1 Cấu trúc AgentActivityLog

```go
type AgentActivityLog struct {
    ID           primitive.ObjectID
    AgentID      primitive.ObjectID
    ActivityType string   // "check_in", "command_executed", "config_applied", "job_run", "error"
    Timestamp    int64
    Data         map[string]interface{}  // ← VẤN ĐỀ: Không giới hạn kích thước
    Message      string
    Severity     string
}
```

### 1.2 Nguồn dữ liệu phình to

- **Check-in** (mỗi ~60 giây): Lưu **toàn bộ** `checkInData` vào `Data.checkInData`:
  - `configData` – full config (có thể rất lớn: jobs, schedules, params…)
  - `systemInfo` – OS, Arch, CPU, Memory, Disk…
  - `metrics` – bot-level metrics
  - `jobStatus` – array chi tiết từng job
  - `configVersion`, `configHash`, `status`, `healthStatus`, `tags`, metadata…

- **Trùng lặp**: Phần lớn dữ liệu đã được lưu trong `agent_registry.Status` (systemInfo, metrics, jobStatus, configVersion, configHash).

- **Tần suất**: Bot check-in mỗi 60 giây → ~1.440 bản ghi/ngày/agent với payload lớn.

---

## 2. Đề xuất phương án

### Phương án A: Giảm dữ liệu lưu theo activityType (Khuyến nghị)

**Ý tưởng**: Chỉ lưu dữ liệu cần thiết cho từng loại activity.

| ActivityType      | Dữ liệu nên lưu                                      | Không lưu                          |
|-------------------|------------------------------------------------------|------------------------------------|
| `check_in`        | `status`, `healthStatus`, `configVersion` (nếu đổi)   | configData, systemInfo, metrics, jobStatus |
| `command_executed`| `commandType`, `target`, `status`, `summary`         | result chi tiết (trừ khi error)     |
| `config_applied`  | `version`, `hash`, `status`                          | configData đầy đủ                   |
| `job_run`         | `jobId`, `status`, `duration`                        | output/result lớn                   |
| `error`           | `message`, `code`, `context`                         | stack trace đầy đủ (có thể truncate)|

**Ưu điểm**: Giảm mạnh dung lượng, dễ triển khai, vẫn đủ cho audit/debug.  
**Nhược điểm**: Mất chi tiết đầy đủ cho check-in (nhưng đã có trong `agent_registry`).

---

### Phương án B: Không log check_in định kỳ

**Ý tưởng**: Check-in chỉ cập nhật `agent_registry`; không tạo activity log.

- Activity log chỉ dùng cho: `command_executed`, `config_applied`, `job_run`, `error`.
- "Last seen" lấy từ `agent_registry.LastCheckInAt`.

**Ưu điểm**: Giảm rất mạnh số lượng bản ghi.  
**Nhược điểm**: Không có lịch sử check-in (heartbeat) trong activity log.

---

### Phương án C: Check-in nhẹ (lightweight)

**Ý tưởng**: Vẫn log check_in nhưng chỉ lưu tối thiểu:

```json
{
  "activityType": "check_in",
  "data": {
    "status": "online",
    "healthStatus": "healthy",
    "configVersion": 1709568000
  }
}
```

- Bỏ: `configData`, `systemInfo`, `metrics`, `jobStatus`, metadata dài.
- Có thể thêm `summary`: `"jobsRunning": 2`, `"hasError": false` nếu cần.

**Ưu điểm**: Cân bằng giữa audit và dung lượng.  
**Nhược điểm**: Cần sửa logic gọi `LogActivity` trong `HandleEnhancedCheckIn`.

---

### Phương án D: TTL (Time To Live) / Retention

**Ý tưởng**: Giữ nguyên cách log hiện tại, nhưng xóa bản ghi cũ theo thời gian.

- MongoDB TTL index: `db.agent_activity_logs.createIndex({ "timestamp": 1 }, { expireAfterSeconds: 604800 })` (7 ngày).
- Hoặc job định kỳ xóa theo `timestamp`.

**Ưu điểm**: Dễ triển khai, không đụng logic log.  
**Nhược điểm**: Vẫn tốn dung lượng và I/O cho dữ liệu phình to trong thời gian retention.

---

### Phương án E: Sampling check-in

**Ý tưởng**: Chỉ log một phần check-in:

- Chỉ log mỗi check-in thứ N (ví dụ: mỗi 10 phút).
- Hoặc chỉ log khi có thay đổi: `status`, `healthStatus`, `configVersion`.

**Ưu điểm**: Giảm số lượng bản ghi.  
**Nhược điểm**: Logic phức tạp hơn, có thể bỏ sót sự kiện quan trọng.

---

## 3. Khuyến nghị triển khai

### Bước 1: Áp dụng Phương án A + C (ngắn hạn)

1. **Sửa `HandleEnhancedCheckIn`** – không truyền toàn bộ `checkInData`:

   ```go
   // Thay vì:
   activityData := map[string]interface{}{"checkInData": checkInData}

   // Dùng:
   activityData := buildLightweightCheckInData(checkInData)
   ```

2. **Hàm `buildLightweightCheckInData`**:

   ```go
   func buildLightweightCheckInData(checkInData map[string]interface{}) map[string]interface{} {
       data := map[string]interface{}{
           "status":       checkInData["status"],
           "healthStatus": checkInData["healthStatus"],
       }
       if v, ok := checkInData["configVersion"]; ok {
           data["configVersion"] = v
       }
       if v, ok := checkInData["configHash"]; ok && v != "" {
           data["configHash"] = v
       }
       // Tùy chọn: summary metrics nếu cần
       if jobs, ok := checkInData["jobStatus"].([]interface{}); ok && len(jobs) > 0 {
           data["jobCount"] = len(jobs)
       }
       return data
   }
   ```

3. **Giới hạn kích thước `Data`** trong `LogActivity`:
   - Thêm helper `truncateData(data map[string]interface{}, maxBytes int)`.
   - Hoặc whitelist fields theo `activityType` (như bảng trên).

### Bước 2: Thêm TTL (Phương án D)

- Tạo TTL index cho `agent_activity_logs` (ví dụ: 7–30 ngày).
- Giúp collection không tăng trưởng vô hạn.

### Bước 3: (Tùy chọn) Xem xét Phương án B

- Nếu sau khi áp dụng A+C vẫn còn quá nhiều check-in:
  - Có thể tắt log check_in hoàn toàn.
  - Chỉ dùng `agent_registry` cho trạng thái realtime.

---

## 4. Tóm tắt

| Phương án | Độ phức tạp | Giảm dung lượng | Giảm số bản ghi | Khuyến nghị |
|-----------|-------------|-----------------|-----------------|-------------|
| A: Giảm data theo type | Trung bình | Cao | Không | ✅ Áp dụng |
| B: Không log check_in | Thấp | Cao | Rất cao | Cân nhắc |
| C: Check-in nhẹ | Thấp | Cao | Không | ✅ Áp dụng |
| D: TTL/Retention | Thấp | Trung bình | Không | ✅ Áp dụng |
| E: Sampling | Cao | Trung bình | Cao | Tùy chọn |

**Thứ tự ưu tiên**: C → A → D → (B nếu cần) → E.
