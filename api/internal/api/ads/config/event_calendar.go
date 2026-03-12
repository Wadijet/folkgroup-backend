// Package config — Event Calendar 12 tháng Việt Nam theo FolkForm v4.1.
// Dùng cho Mode Detection (+3 BLITZ trong Prep Days), Mess Trap Override, Reset Budget bonus.
package config

import "time"

// EventItem mô tả một sự kiện trong lịch.
type EventItem struct {
	Month      int    // Tháng (1-12)
	Name       string // Tên sự kiện
	EventDay   int    // Ngày sự kiện (vd: 8 cho 8/3)
	PrepDays   int    // Số ngày Prep trước event
	BlitzBonus int    // Điểm cộng BLITZ khi trong Prep/Event
}

// EventCalendarVN 12 sự kiện Việt Nam theo FolkForm v4.1.
var EventCalendarVN = []EventItem{
	{1, "Tết Nguyên Đán", 28, 15, 3},
	{2, "Valentine / Rằm Tháng Giêng", 14, 5, 3},
	{3, "Ngày Quốc Tế Phụ Nữ 8/3", 8, 7, 3},
	{4, "Giỗ Tổ + 30/4", 10, 3, 3},
	{5, "Quốc Tế Lao Động 1/5", 1, 2, 3},
	{6, "Tết Đoan Ngọ", 25, 5, 3},
	{7, "Vu Lan Báo Hiếu", 22, 7, 3},
	{8, "Ngày Phụ Nữ VN 20/8", 20, 7, 3},
	{9, "Tết Trung Thu", 25, 10, 3},
	{10, "Ngày Phụ Nữ VN 20/10", 20, 10, 3},
	{11, "Ngày Nhà Giáo 20/11", 20, 10, 3},
	{12, "Giáng Sinh", 25, 10, 3},
}

// IsEventWindow kiểm tra ngày có trong Prep Days hoặc Event Day không.
// Trả về (isInWindow, blitzBonus, eventName).
func IsEventWindow(t time.Time) (bool, int, string) {
	day := t.Day()
	month := int(t.Month())
	for _, ev := range EventCalendarVN {
		if ev.Month != month {
			continue
		}
		// Prep: [eventDay - prepDays, eventDay - 1] hoặc [eventDay, eventDay]
		startPrep := ev.EventDay - ev.PrepDays
		if startPrep < 1 {
			startPrep = 1
		}
		if day >= startPrep && day <= ev.EventDay {
			return true, ev.BlitzBonus, ev.Name
		}
	}
	return false, 0, ""
}

// IsWeekend trả về true nếu là T7 hoặc CN (penalty -2).
func IsWeekend(t time.Time) bool {
	wd := t.Weekday()
	return wd == time.Saturday || wd == time.Sunday
}
