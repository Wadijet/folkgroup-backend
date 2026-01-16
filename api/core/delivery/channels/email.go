package channels

import (
	"context"
	"fmt"

	models "meta_commerce/core/api/models/mongodb"

	"gopkg.in/gomail.v2"
)

// RenderedTemplate là template đã được render
type RenderedTemplate struct {
	Subject string
	Content string
	CTAs    []RenderedCTA
}

// RenderedCTA là CTA đã được render
type RenderedCTA struct {
	Label       string
	Action      string // Tracking URL (đã được thay thế)
	OriginalURL string // Original URL (để redirect sau khi track)
	Style       string // Chỉ để styling
}

// SendEmail gửi email
func SendEmail(ctx context.Context, sender *models.NotificationChannelSender, recipient string, template *RenderedTemplate, historyID string, baseURL string) error {
	// Format CTAs thành HTML buttons
	ctaHTML := ""
	for _, cta := range template.CTAs {
		styleClass := "btn-primary"
		if cta.Style != "" {
			styleClass = "btn-" + cta.Style
		}
		ctaHTML += fmt.Sprintf(`<a href="%s" class="btn %s" style="display:inline-block;padding:10px 20px;margin:5px;text-decoration:none;border-radius:5px;background-color:#007bff;color:#fff;">%s</a>`,
			cta.Action, styleClass, cta.Label)
	}

	// Combine content + CTAs
	htmlContent := template.Content
	if ctaHTML != "" {
		htmlContent += "<div style='margin-top:20px;'>" + ctaHTML + "</div>"
	}

	// Thêm tracking pixel (để track open) - dùng unified tracking endpoint
	trackingPixel := fmt.Sprintf(`<img src="%s/api/v1/track/open/%s" width="1" height="1" style="display:none">`,
		baseURL, historyID)
	htmlContent += trackingPixel

	msg := gomail.NewMessage()
	msg.SetHeader("From", fmt.Sprintf("%s <%s>", sender.FromName, sender.FromEmail))
	msg.SetHeader("To", recipient)
	msg.SetHeader("Subject", template.Subject)
	msg.SetBody("text/html", htmlContent)

	dialer := gomail.NewDialer(sender.SMTPHost, sender.SMTPPort, sender.SMTPUsername, sender.SMTPPassword)
	return dialer.DialAndSend(msg)
}
