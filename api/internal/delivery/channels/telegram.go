package channels

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"meta_commerce/internal/logger"
	notifmodels "meta_commerce/internal/api/notification/models"
)

// SendTelegram gửi telegram message
func SendTelegram(ctx context.Context, sender *notifmodels.NotificationChannelSender, chatID string, template *RenderedTemplate, historyID string, baseURL string) error {
	log := logger.GetAppLogger()
	log.WithFields(map[string]interface{}{
		"historyId":  historyID,
		"chatID":     chatID,
		"senderId":   sender.ID.Hex(),
		"senderName": sender.Name,
		"botUsername": sender.BotUsername,
	}).Info("📱 [TELEGRAM] Bắt đầu gửi Telegram message")
	
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
