package logger

import (
	"fmt"
	"io"
	"os"
	"runtime/debug"
	"sync"

	"github.com/sirupsen/logrus"
)

// AsyncHook là một hook để ghi log bất đồng bộ, tránh blocking request handling
// Hook này sẽ buffer log entries và ghi chúng vào các writers trong một goroutine riêng
// Hỗ trợ nhiều writers (file, stdout, etc.) để tránh blocking
type AsyncHook struct {
	writers    []io.Writer // Danh sách các writers (file, stdout, etc.)
	entries    chan *logrus.Entry
	wg         sync.WaitGroup
	mu         sync.Mutex
	closed     bool
	bufferSize int
}

// NewAsyncHook tạo một async hook mới với một writer
// bufferSize: kích thước buffer cho log entries (mặc định 1000)
func NewAsyncHook(writer io.Writer, bufferSize int) *AsyncHook {
	return NewAsyncHookWithWriters([]io.Writer{writer}, bufferSize)
}

// NewAsyncHookWithWriters tạo một async hook mới với nhiều writers
// bufferSize: kích thước buffer cho log entries (mặc định 1000)
func NewAsyncHookWithWriters(writers []io.Writer, bufferSize int) *AsyncHook {
	if bufferSize <= 0 {
		bufferSize = 1000 // Mặc định 1000 entries
	}

	hook := &AsyncHook{
		writers:    writers,
		entries:    make(chan *logrus.Entry, bufferSize),
		bufferSize: bufferSize,
	}

	// Khởi động goroutine để xử lý log entries
	hook.wg.Add(1)
	go hook.processEntries()

	return hook
}

// Levels trả về các log levels mà hook này xử lý
func (h *AsyncHook) Levels() []logrus.Level {
	return logrus.AllLevels
}

// Fire được gọi mỗi khi có log entry mới
// Hàm này sẽ không block, chỉ đưa entry vào channel
func (h *AsyncHook) Fire(entry *logrus.Entry) error {
	h.mu.Lock()
	closed := h.closed
	h.mu.Unlock()

	if closed {
		// Nếu hook đã đóng, ghi trực tiếp vào tất cả writers (fallback)
		var data []byte
		var err error

		if entry.Logger.Formatter != nil {
			data, err = entry.Logger.Formatter.Format(entry)
		} else {
			line, strErr := entry.String()
			if strErr != nil {
				return strErr
			}
			data = []byte(line)
		}

		if err != nil {
			return err
		}

		for _, writer := range h.writers {
			_, _ = writer.Write(data) // Ignore errors khi đã đóng
		}
		return nil
	}

	// Non-blocking send: nếu channel đầy, bỏ qua log entry này
	// Điều này đảm bảo không block request handling
	select {
	case h.entries <- entry:
		// Entry đã được đưa vào channel thành công
	default:
		// Channel đầy, bỏ qua log entry này để không block
		// Có thể log warning nếu cần, nhưng không nên log ở đây vì sẽ tạo vòng lặp
	}

	return nil
}

// processEntries xử lý log entries trong một goroutine riêng
// ⚠️ QUAN TRỌNG: Hàm này có recover để đảm bảo logger goroutine không crash server
func (h *AsyncHook) processEntries() {
	defer h.wg.Done()

	for entry := range h.entries {
		// ✅ THÊM RECOVER để bắt panic và không làm crash server
		func() {
			defer func() {
				if r := recover(); r != nil {
					// Không thể dùng logger ở đây vì sẽ tạo vòng lặp
					// Ghi trực tiếp vào stderr để báo lỗi
					fmt.Fprintf(os.Stderr, "[LOGGER PANIC] Logger goroutine panic recovered: %v\n", r)
					debug.PrintStack()
					// Tiếp tục xử lý entry tiếp theo, không crash server
				}
			}()

			// Kiểm tra xem entry có bị filter không
			// FilterHook đã đánh dấu entry bị filter bằng field "_filtered"
			if filtered, ok := entry.Data["_filtered"].(bool); ok && filtered {
				// Entry bị filter, bỏ qua không ghi log
				return
			}

			// Loại bỏ field "_filtered" khỏi entry trước khi format
			// (field này chỉ dùng để đánh dấu, không cần ghi vào log)
			filteredEntry := entry
			if _, ok := entry.Data["_filtered"]; ok {
				// Tạo entry mới không có field "_filtered"
				filteredEntry = entry.Dup()
				delete(filteredEntry.Data, "_filtered")
			}

			// Format entry thành bytes sử dụng formatter của logger
			// entry.Logger.Formatter sẽ format entry với formatter đã được set
			var data []byte
			var err error

			if filteredEntry.Logger.Formatter != nil {
				// Dùng formatter của logger để format entry
				data, err = filteredEntry.Logger.Formatter.Format(filteredEntry)
			} else {
				// Fallback: dùng String() nếu không có formatter
				line, strErr := filteredEntry.String()
				if strErr != nil {
					return // Bỏ qua entry này nếu không format được
				}
				data = []byte(line)
			}

			if err != nil {
				return // Bỏ qua nếu không format được
			}

			// Ghi vào tất cả writers (có thể block ở đây, nhưng không ảnh hưởng request handling)
			// Nếu một writer chậm, nó sẽ không block các writers khác
			for _, writer := range h.writers {
				_, err = writer.Write(data)
				if err != nil {
					// Không thể log lỗi ở đây vì sẽ tạo vòng lặp
					// Tiếp tục với writer tiếp theo
					continue
				}
			}
		}()
	}
}

// Close đóng hook và đợi tất cả entries được xử lý xong
func (h *AsyncHook) Close() error {
	h.mu.Lock()
	if h.closed {
		h.mu.Unlock()
		return nil
	}
	h.closed = true
	h.mu.Unlock()

	close(h.entries)
	h.wg.Wait()
	return nil
}
