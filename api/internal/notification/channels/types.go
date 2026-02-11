package channels

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
