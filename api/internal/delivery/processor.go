package delivery

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sync"
	"time"

	deliverysvc "meta_commerce/internal/api/delivery/service"
	deliverymodels "meta_commerce/internal/api/delivery/models"
	notifmodels "meta_commerce/internal/api/notification/models"
	notifsvc "meta_commerce/internal/api/notification/service"
	basesvc "meta_commerce/internal/api/base/service"
	"meta_commerce/internal/delivery/channels"
	"meta_commerce/internal/logger"
	"meta_commerce/internal/notification"
	"meta_commerce/internal/worker"
	"meta_commerce/internal/worker/metrics"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Processor xử lý queue items - chỉ xử lý delivery (như "bưu điện")
// Nhận: sender, recipient, content đã render
// Gửi đi
type Processor struct {
	queueService   *deliverysvc.DeliveryQueueService
	historyService *deliverysvc.DeliveryHistoryService
	senderService  *notifsvc.NotificationSenderService
	baseURL        string
}

// NewProcessor tạo mới Processor
func NewProcessor(baseURL string) (*Processor, error) {
	queueService, err := deliverysvc.NewDeliveryQueueService()
	if err != nil {
		return nil, fmt.Errorf("failed to create queue service: %w", err)
	}

	historyService, err := deliverysvc.NewDeliveryHistoryService()
	if err != nil {
		return nil, fmt.Errorf("failed to create history service: %w", err)
	}

	senderService, err := notifsvc.NewNotificationSenderService()
	if err != nil {
		return nil, fmt.Errorf("failed to create sender service: %w", err)
	}

	return &Processor{
		queueService:   queueService,
		historyService: historyService,
		senderService:  senderService,
		baseURL:        baseURL,
	}, nil
}

// processDeliveryItem xử lý một queue item: reset stale, update status, gọi ProcessQueueItem.
// Dùng bởi worker pool và chạy tuần tự.
func (p *Processor) processDeliveryItem(ctx context.Context, item *deliverymodels.DeliveryQueueItem) {
	appLog := logger.GetAppLogger()
	// Nếu item đang processing (stale), reset về pending trước
	if item.Status == "processing" {
		ids := []interface{}{item.ID}
		if err := p.queueService.UpdateStatus(ctx, ids, "pending"); err != nil {
			appLog.WithError(err).WithField("queueItemId", item.ID.Hex()).Error("📦 [DELIVERY] Failed to reset stale item to pending")
			return
		}
		item.Status = "pending"
	}

	ids := []interface{}{item.ID}
	if err := p.queueService.UpdateStatus(ctx, ids, "processing"); err != nil {
		appLog.WithError(err).WithField("queueItemId", item.ID.Hex()).Error("📦 [DELIVERY] Failed to update queue item status")
		return
	}

	start := time.Now()
	defer func() {
		channelType := item.ChannelType
		if channelType == "" {
			channelType = "unknown"
		}
		metrics.RecordDuration("delivery:"+channelType, time.Since(start))
	}()
	defer func() {
		if r := recover(); r != nil {
			appLog.WithFields(map[string]interface{}{
				"panic":       r,
				"queueItemId": item.ID.Hex(),
			}).Error("📦 [DELIVERY] Panic khi xử lý queue item")
			ids := []interface{}{item.ID}
			p.queueService.UpdateStatus(ctx, ids, "pending")
			updateData := basesvc.UpdateData{
				Set: map[string]interface{}{
					"status":    "pending",
					"updatedAt": time.Now().Unix(),
				},
			}
			p.queueService.UpdateOne(ctx, bson.M{"_id": item.ID}, updateData, nil)
		}
	}()

	err := p.ProcessQueueItem(ctx, item)
	if err != nil {
		appLog.WithError(err).WithFields(map[string]interface{}{
			"queueItemId": item.ID.Hex(),
			"retryCount":  item.RetryCount,
		}).Error("📦 [DELIVERY] Failed to process queue item")
		existingItem, findErr := p.queueService.FindOneById(ctx, item.ID)
		if findErr == nil && existingItem.Status == "processing" {
			updateData := basesvc.UpdateData{
				Set: map[string]interface{}{
					"status":    "pending",
					"updatedAt": time.Now().Unix(),
				},
			}
			_, updateErr := p.queueService.UpdateOne(ctx, bson.M{"_id": item.ID}, updateData, nil)
			if updateErr != nil {
				appLog.WithError(updateErr).WithField("queueItemId", item.ID.Hex()).Error("📦 [DELIVERY] Failed to reset item to pending after error")
			}
		}
	}
}

// handleRetryOrFail xử lý retry logic cho mọi error case
// Nếu chưa hết retry: tăng retryCount, set nextRetryAt, reset về pending
// Nếu đã hết retry: đánh dấu failed và xóa khỏi queue
func (p *Processor) handleRetryOrFail(ctx context.Context, item *deliverymodels.DeliveryQueueItem, err error) error {
	log := logger.GetAppLogger()
	
	// Tăng retryCount
	item.RetryCount++
	
	if item.RetryCount < item.MaxRetries {
		// Chưa hết retry, schedule retry
		item.Status = "pending"
		backoffSeconds := int64(math.Pow(2, float64(item.RetryCount)))
		nextRetryAt := time.Now().Unix() + backoffSeconds
		item.NextRetryAt = &nextRetryAt
		item.UpdatedAt = time.Now().Unix()
		
		updateData := basesvc.UpdateData{
			Set: map[string]interface{}{
				"status":      item.Status,
				"retryCount":  item.RetryCount,
				"nextRetryAt": item.NextRetryAt,
				"updatedAt":   item.UpdatedAt,
				"error":       err.Error(), // Lưu error message
			},
		}
		_, updateErr := p.queueService.UpdateOne(ctx, bson.M{"_id": item.ID}, updateData, nil)
		if updateErr != nil {
			log.WithError(updateErr).WithField("queueItemId", item.ID.Hex()).Error("📦 [DELIVERY] Failed to update queue item for retry")
			return fmt.Errorf("failed to update queue item for retry: %w", updateErr)
		}
		
		// Đã tắt log Info để giảm log (chỉ log Error/Warn)
		return err // Return error để caller biết cần retry
	} else {
		// Đã hết số lần retry, đánh dấu failed và xóa khỏi queue
		updateData := basesvc.UpdateData{
			Set: map[string]interface{}{
				"status":    "failed",
				"error":     err.Error(),
				"updatedAt": time.Now().Unix(),
			},
		}
		_, updateErr := p.queueService.UpdateOne(ctx, bson.M{"_id": item.ID}, updateData, nil)
		if updateErr != nil {
			log.WithError(updateErr).WithField("queueItemId", item.ID.Hex()).Error("📦 [DELIVERY] Failed to mark queue item as failed")
			return fmt.Errorf("failed to mark queue item as failed: %w", updateErr)
		}
		
		// Xóa queue item (cleanup)
		deleteErr := p.queueService.DeleteOne(ctx, bson.M{"_id": item.ID})
		if deleteErr != nil {
			log.WithError(deleteErr).WithField("queueItemId", item.ID.Hex()).Warn("📦 [DELIVERY] Failed to delete failed queue item (đã đánh dấu failed, sẽ không được filter ra nữa)")
		} else {
			// Đã tắt log Info để giảm log (chỉ log Error/Warn)
		}
		
		return fmt.Errorf("max retries exceeded: %w", err)
	}
}

// ProcessQueueItem xử lý một queue item - chỉ xử lý delivery (nhận content đã render)
func (p *Processor) ProcessQueueItem(ctx context.Context, item *deliverymodels.DeliveryQueueItem) error {
	var err error
	log := logger.GetAppLogger()
	// Đã tắt log Info để giảm log (background job chạy thường xuyên)

	// 1. Validate senderID trước
	if item.SenderID.IsZero() {
		err := fmt.Errorf("senderID is empty or invalid")
		log.WithFields(map[string]interface{}{
			"queueItemId": item.ID.Hex(),
		}).Error("📦 [DELIVERY] Queue item có senderID rỗng")
		return p.handleRetryOrFail(ctx, item, err)
	}

	// 2. Lấy sender (Option C: Hybrid - ưu tiên SenderConfig, fallback query từ SenderID)
	var sender *notifmodels.NotificationChannelSender
	if item.SenderConfig != "" {
		// Fast path: Decrypt và dùng sender config từ queue item
		decryptedConfig, err := DecryptSenderConfig(item.SenderConfig)
		if err != nil {
			log.WithError(err).WithFields(map[string]interface{}{
				"senderId": item.SenderID.Hex(),
			}).Warn("📦 [DELIVERY] Không thể decrypt sender config, fallback về query từ SenderID")
			// Fallback về query từ SenderID
			s, err := p.senderService.FindOneById(ctx, item.SenderID)
			if err != nil {
				log.WithError(err).WithFields(map[string]interface{}{
					"senderId": item.SenderID.Hex(),
				}).Error("📦 [DELIVERY] Sender not found (fallback)")
				return p.handleRetryOrFail(ctx, item, fmt.Errorf("sender not found: %w", err))
			}
			sender = &s
		} else {
			// Parse decrypted config thành sender
			var decryptedSender notifmodels.NotificationChannelSender
			if err := json.Unmarshal(decryptedConfig, &decryptedSender); err != nil {
				log.WithError(err).WithFields(map[string]interface{}{
					"senderId": item.SenderID.Hex(),
				}).Warn("📦 [DELIVERY] Không thể parse decrypted sender config, fallback về query từ SenderID")
				// Fallback về query từ SenderID
				s, err := p.senderService.FindOneById(ctx, item.SenderID)
				if err != nil {
					log.WithError(err).WithFields(map[string]interface{}{
						"senderId": item.SenderID.Hex(),
					}).Error("📦 [DELIVERY] Sender not found (fallback)")
					return p.handleRetryOrFail(ctx, item, fmt.Errorf("sender not found: %w", err))
				}
				sender = &s
			} else {
				sender = &decryptedSender
				// Đã tắt log Debug để giảm log
			}
		}
	} else {
		// Fallback path: Query sender từ database
		s, err := p.senderService.FindOneById(ctx, item.SenderID)
		if err != nil {
			log.WithError(err).WithFields(map[string]interface{}{
				"senderId": item.SenderID.Hex(),
			}).Error("📦 [DELIVERY] Sender not found")
			return fmt.Errorf("sender not found: %w", err)
		}
		sender = &s
		// Đã tắt log Debug để giảm log
	}

	if !sender.IsActive {
		err := fmt.Errorf("sender is not active")
		log.WithFields(map[string]interface{}{
			"senderId": item.SenderID.Hex(),
		}).Warn("📦 [DELIVERY] Sender không active")
		return p.handleRetryOrFail(ctx, item, err)
	}

	// 4. Parse CTAs từ JSON string (nếu có)
	var renderedCTAs []channels.RenderedCTA
	if len(item.CTAs) > 0 {
		for _, ctaJSON := range item.CTAs {
			var cta channels.RenderedCTA
			if err := json.Unmarshal([]byte(ctaJSON), &cta); err != nil {
				log.WithError(err).Warn("📦 [DELIVERY] Failed to parse CTA JSON, skipping")
				continue
			}
			renderedCTAs = append(renderedCTAs, cta)
		}
	}

	// 5. Tạo RenderedTemplate từ queue item
	rendered := &channels.RenderedTemplate{
		Subject: item.Subject,
		Content: item.Content,
		CTAs:    renderedCTAs,
	}

	// 6. Tạo history record (trước khi gửi)
	// Infer Domain và Severity từ EventType để lưu vào history (cho reporting)
	domain := notification.GetDomainFromEventType(item.EventType)
	severity := notification.GetSeverityFromEventType(item.EventType)

	historyID := primitive.NewObjectID()
	history := &deliverymodels.DeliveryHistory{
		ID:                  historyID,
		QueueItemID:         item.ID,
		EventType:           item.EventType,
		OwnerOrganizationID: item.OwnerOrganizationID,
		Domain:              domain,   // Lưu để reporting
		Severity:            severity, // Lưu để reporting
		ChannelType:         item.ChannelType,
		Recipient:           item.Recipient,
		Status:              "pending",
		Content:             rendered.Content,
		RetryCount:          item.RetryCount,
		CreatedAt:           time.Now().Unix(),
	}

	// Initialize CTAClicks array
	history.CTAClicks = make([]deliverymodels.CTAClick, len(renderedCTAs))
	for i, cta := range renderedCTAs {
		history.CTAClicks[i] = deliverymodels.CTAClick{
			CTAIndex:   i,
			Label:      cta.Label,
			ClickCount: 0,
		}
	}

	// 7. Gửi notification (CTAs đã có tracking URLs từ Notification System)
	sendErr := p.sendNotification(ctx, sender, item.ChannelType, item.Recipient, rendered, historyID.Hex())
	if sendErr != nil {
		history.Status = "failed"
		history.Error = sendErr.Error()
		history.SentAt = nil
	} else {
		history.Status = "sent"
		now := time.Now().Unix()
		history.SentAt = &now
	}

	// 8. Lưu history
	_, err = p.historyService.InsertOne(ctx, *history)
	if err != nil {
		log.WithError(err).WithField("historyId", historyID.Hex()).Error("📦 [DELIVERY] Failed to save history")
		return p.handleRetryOrFail(ctx, item, fmt.Errorf("failed to save history: %w", err))
	}

	// 9. Xử lý kết quả gửi notification
	if sendErr != nil {
		// Gửi thất bại, xử lý retry hoặc fail
		return p.handleRetryOrFail(ctx, item, sendErr)
	} else {
		// Gửi thành công, xóa queue item
		err = p.queueService.DeleteOne(ctx, bson.M{"_id": item.ID})
		if err != nil {
			log.WithError(err).WithField("queueItemId", item.ID.Hex()).Warn("📦 [DELIVERY] Failed to delete completed queue item")
		} else {
			// Đã tắt log Info để giảm log (chỉ log Error/Warn)
		}
		return nil
	}
}



// sendNotification gửi notification qua channel tương ứng
func (p *Processor) sendNotification(ctx context.Context, sender *notifmodels.NotificationChannelSender, channelType string, recipient string, rendered *channels.RenderedTemplate, historyID string) error {
	switch channelType {
	case "email":
		return channels.SendEmail(ctx, sender, recipient, rendered, historyID, p.baseURL)
	case "telegram":
		return channels.SendTelegram(ctx, sender, recipient, rendered, historyID, p.baseURL)
	case "webhook":
		return channels.SendWebhook(ctx, recipient, rendered, historyID, p.baseURL)
	default:
		return fmt.Errorf("unsupported channel type: %s", channelType)
	}
}

// StartCleanupJob bắt đầu background job để dọn dẹp items bị kẹt
func (p *Processor) StartCleanupJob(ctx context.Context) {
	cleanupInterval := 1 * time.Minute // Chạy mỗi 1 phút
	staleMinutes := 5                  // Items processing quá 5 phút được coi là stuck
	batchSize := 50                     // Xử lý tối đa 50 items mỗi lần

	go func() {
		// ✅ THÊM RECOVER để đảm bảo cleanup job goroutine không crash server
		defer func() {
			if r := recover(); r != nil {
				log := logger.GetAppLogger()
				log.WithFields(map[string]interface{}{
					"panic": r,
				}).Error("📦 [CLEANUP] Cleanup job goroutine panic recovered, job sẽ tiếp tục chạy")
			}
		}()

		ticker := time.NewTicker(cleanupInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if !worker.IsWorkerActive(worker.WorkerDeliveryCleanup) {
					time.Sleep(1 * time.Minute)
					continue
				}
				prio := worker.GetPriority(worker.WorkerDeliveryCleanup, worker.PriorityLow)
				if worker.ShouldThrottle(prio) {
					continue
				}
				log := logger.GetAppLogger()

				// Tìm items bị kẹt
				effBatch := worker.GetEffectiveBatchSize(batchSize, prio)
				stuckItems, err := p.queueService.FindStuckItems(ctx, staleMinutes, effBatch)
				if err != nil {
					log.WithError(err).Error("📦 [CLEANUP] Failed to find stuck queue items")
					continue
				}

				if len(stuckItems) == 0 {
					// Đã tắt log Debug để giảm log
					continue
				}

				// Đã tắt log Info để giảm log (chỉ log Error/Warn khi có vấn đề nghiêm trọng)

				for _, item := range stuckItems {
					func() {
						defer func() {
							if r := recover(); r != nil {
								log := logger.GetAppLogger()
								log.WithFields(map[string]interface{}{
									"panic":       r,
									"queueItemId": item.ID.Hex(),
								}).Error("📦 [CLEANUP] Panic khi cleanup item")
							}
						}()

						// Xử lý từng item bị kẹt
						if item.SenderID.IsZero() {
							// Item có senderID rỗng, đánh dấu failed và xóa
							log.WithField("queueItemId", item.ID.Hex()).Warn("📦 [CLEANUP] Item có senderID rỗng, đánh dấu failed và xóa")
							updateData := basesvc.UpdateData{
								Set: map[string]interface{}{
									"status":    "failed",
									"error":     "senderID is empty or invalid (cleaned up by cleanup job)",
									"updatedAt": time.Now().Unix(),
								},
							}
							_, err := p.queueService.UpdateOne(ctx, bson.M{"_id": item.ID}, updateData, nil)
							if err != nil {
								log.WithError(err).WithField("queueItemId", item.ID.Hex()).Error("📦 [CLEANUP] Failed to mark item as failed")
							} else {
								p.queueService.DeleteOne(ctx, bson.M{"_id": item.ID})
								// Đã tắt log Info để giảm log
							}
						} else if item.Status == "processing" {
							// Item đang processing quá lâu, reset về pending để retry
							log.WithField("queueItemId", item.ID.Hex()).Warn("📦 [CLEANUP] Item processing quá lâu, reset về pending")
							updateData := basesvc.UpdateData{
								Set: map[string]interface{}{
									"status":    "pending",
									"nextRetryAt": nil, // Reset nextRetryAt để có thể xử lý ngay
									"updatedAt": time.Now().Unix(),
								},
							}
							_, err := p.queueService.UpdateOne(ctx, bson.M{"_id": item.ID}, updateData, nil)
							if err != nil {
								log.WithError(err).WithField("queueItemId", item.ID.Hex()).Error("📦 [CLEANUP] Failed to reset stale item to pending")
							} else {
								// Đã tắt log Info để giảm log
							}
						}
					}()
				}

				// Cleanup items failed cũ (quá 7 ngày)
				deletedCount, err := p.queueService.CleanupFailedItems(ctx, 7)
				if err != nil {
					log.WithError(err).Error("📦 [CLEANUP] Failed to cleanup old failed items")
				} else if deletedCount > 0 {
					// Đã tắt log Info để giảm log
				}
			}
		}
	}()
}

// Start bắt đầu background worker để xử lý queue
func (p *Processor) Start(ctx context.Context) {
	interval := 5 * time.Second
	batchSize := 10
	maxRetryDelay := 60 * time.Second
	retryDelay := 5 * time.Second

	// Khởi động cleanup job
	p.StartCleanupJob(ctx)

	for {
		func() {
			defer func() {
				if r := recover(); r != nil {
					log := logger.GetAppLogger()
					log.WithFields(map[string]interface{}{
						"panic": r,
					}).Error("📦 [DELIVERY] Processor panic, sẽ tự khởi động lại sau khi delay")
					time.Sleep(retryDelay)
					retryDelay *= 2
					if retryDelay > maxRetryDelay {
						retryDelay = maxRetryDelay
					}
				} else {
					retryDelay = 5 * time.Second
				}
			}()

			ticker := time.NewTicker(interval)
			defer ticker.Stop()

			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					if !worker.IsWorkerActive(worker.WorkerDelivery) {
						time.Sleep(1 * time.Minute)
						continue
					}
					prio := worker.GetPriority(worker.WorkerDelivery, worker.PriorityHigh)
					effBatch := worker.GetEffectiveBatchSize(batchSize, prio)
					items, err := p.queueService.FindPending(ctx, effBatch)
					if err != nil {
						log := logger.GetAppLogger()
						log.WithError(err).Error("📦 [DELIVERY] Failed to find pending queue items")
						continue
					}

					if len(items) == 0 {
						// Log mỗi 30 giây một lần để biết processor đang chạy
						// Đã tắt log Debug để giảm log
						continue
					}

					// Item ưu tiên (Priority 1=critical, 2=high): bắt buộc chạy, không bị throttle
					hasPriority := false
					for i := range items {
						if items[i].Priority >= 1 && items[i].Priority <= 2 {
							hasPriority = true
							break
						}
					}
					if !hasPriority {
						if worker.ShouldThrottle(prio) {
							continue
						}
						effInterval := worker.GetEffectiveInterval(interval, prio)
						if effInterval > interval {
							time.Sleep(effInterval - interval)
						}
					}

					// Worker pool: pool size từ env, điều chỉnh theo CPU/RAM qua Controller
					basePool := worker.GetPoolSize(worker.WorkerDelivery, 6)
					poolSize := worker.GetEffectivePoolSize(basePool, prio)
					if poolSize <= 1 {
						// Chạy tuần tự
						for i := range items {
							p.processDeliveryItem(ctx, &items[i])
						}
					} else {
						// Chạy song song với worker pool
						jobs := make(chan *deliverymodels.DeliveryQueueItem, len(items))
						var wg sync.WaitGroup
						for i := 0; i < poolSize; i++ {
							wg.Add(1)
							go func() {
								defer wg.Done()
								for item := range jobs {
									p.processDeliveryItem(ctx, item)
								}
							}()
						}
						for i := range items {
							jobs <- &items[i]
						}
						close(jobs)
						wg.Wait()
					}
				}
			}
		}()

		select {
		case <-ctx.Done():
			return
		default:
		}
	}
}
