package channels

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	notifmodels "meta_commerce/internal/api/notification/models"
	"meta_commerce/internal/logger"
)

// parseTelegramRecipient tách recipient thành chatID và message_thread_id (topic).
// Format: "chatID" (gửi vào chat chính) hoặc "chatID:topicID" (gửi vào topic cụ thể trong forum supergroup).
// Ví dụ: "-123456789" hoặc "-123456789:12345"
func parseTelegramRecipient(recipient string) (chatID string, messageThreadID *int64) {
	parts := strings.SplitN(recipient, ":", 2)
	chatID = strings.TrimSpace(parts[0])
	if len(parts) == 2 && parts[1] != "" {
		if id, err := strconv.ParseInt(strings.TrimSpace(parts[1]), 10, 64); err == nil {
			messageThreadID = &id
		}
	}
	return chatID, messageThreadID
}

// SendTelegram gửi telegram message.
// recipient: chatID (ví dụ "-123456789") hoặc "chatID:topicID" (ví dụ "-123456789:12345") để gửi vào topic cụ thể.
func SendTelegram(ctx context.Context, sender *notifmodels.NotificationChannelSender, recipient string, template *RenderedTemplate, historyID string, baseURL string) error {
	chatID, messageThreadID := parseTelegramRecipient(recipient)
	log := logger.GetAppLogger()
	logFields := map[string]interface{}{
		"historyId":  historyID,
		"chatID":     chatID,
		"senderId":   sender.ID.Hex(),
		"senderName": sender.Name,
		"botUsername": sender.BotUsername,
	}
	if messageThreadID != nil {
		logFields["messageThreadId"] = *messageThreadID
	}
	log.WithFields(logFields).Info("📱 [TELEGRAM] Bắt đầu gửi Telegram message")
	
	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", sender.BotToken)

	// Format CTAs thành inline keyboard
	inlineKeyboard := [][]map[string]interface{}{}
	row := []map[string]interface{}{}
	for _, cta := range template.CTAs {
		// Log URL trước khi gửi
		log.WithFields(map[string]interface{}{
			"historyId":   historyID,
			"ctaLabel":    cta.Label,
			"ctaAction":   cta.Action,
			"originalURL": cta.OriginalURL,
		}).Debug("📱 [TELEGRAM] CTA URL trước khi gửi")
		
		// Telegram không chấp nhận localhost trong URL
		// Nếu URL chứa localhost, bỏ qua CTA này hoặc dùng original URL nếu có
		ctaURL := cta.Action
		if strings.Contains(ctaURL, "localhost") || strings.Contains(ctaURL, "127.0.0.1") {
			log.WithFields(map[string]interface{}{
				"historyId": historyID,
				"ctaLabel":  cta.Label,
				"ctaURL":    ctaURL,
			}).Warn("📱 [TELEGRAM] Bỏ qua CTA vì URL chứa localhost (Telegram không chấp nhận)")
			// Bỏ qua CTA này vì Telegram không chấp nhận localhost
			continue
		}
		
		button := map[string]interface{}{
			"text": cta.Label,
			"url":  ctaURL, // Đã có tracking URL
		}
		row = append(row, button)
		if len(row) >= 3 { // Tối đa 3 buttons/row
			inlineKeyboard = append(inlineKeyboard, row)
			row = []map[string]interface{}{}
		}
	}
	if len(row) > 0 {
		inlineKeyboard = append(inlineKeyboard, row)
	}

	payload := map[string]interface{}{
		"chat_id": chatID,
		"text":    template.Content,
	}
	if messageThreadID != nil {
		payload["message_thread_id"] = *messageThreadID
	}

	if len(inlineKeyboard) > 0 {
		keyboard := map[string]interface{}{
			"inline_keyboard": inlineKeyboard,
		}
		payload["reply_markup"] = keyboard
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		log.WithError(err).WithFields(map[string]interface{}{
			"historyId": historyID,
			"chatID":    chatID,
			"url":       url,
		}).Error("📱 [TELEGRAM] Lỗi khi gọi Telegram API")
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// Đọc response body để xem lỗi chi tiết
		bodyBytes, _ := io.ReadAll(resp.Body)
		errorMsg := fmt.Sprintf("telegram API returned status %d: %s", resp.StatusCode, string(bodyBytes))
		log.WithFields(map[string]interface{}{
			"historyId":   historyID,
			"chatID":      chatID,
			"statusCode":  resp.StatusCode,
			"response":    string(bodyBytes),
		}).Error("📱 [TELEGRAM] Telegram API trả về lỗi")
		return fmt.Errorf("%s", errorMsg)
	}

	log.WithFields(map[string]interface{}{
		"historyId": historyID,
		"chatID":    chatID,
	}).Info("📱 [TELEGRAM] Gửi Telegram message thành công")
	return nil
}
