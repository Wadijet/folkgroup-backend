package channels

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// SendWebhook gửi webhook
func SendWebhook(ctx context.Context, webhookURL string, template *RenderedTemplate, historyID string, baseURL string) error {
	// Format CTAs thành JSON
	actions := []map[string]interface{}{}
	for _, cta := range template.CTAs {
		actions = append(actions, map[string]interface{}{
			"label": cta.Label,
			"url":   cta.Action,
			"style": cta.Style,
		})
	}

	payload := map[string]interface{}{
		"content":   template.Content,
		"timestamp": time.Now().Unix(),
		"actions":   actions,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", webhookURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("webhook returned status %d", resp.StatusCode)
	}

	return nil
}
