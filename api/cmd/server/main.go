package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/sirupsen/logrus"

	adsworker "meta_commerce/internal/api/ads/worker"
	crmqueue "meta_commerce/internal/api/aidecision/crmqueue"
	"meta_commerce/internal/api/aidecision/decisionlive"
	aidecisionworker "meta_commerce/internal/api/aidecision/worker"
	learningworker "meta_commerce/internal/api/learning/worker"
	cixworker "meta_commerce/internal/api/cix/worker"
	orderintelworker "meta_commerce/internal/api/orderintel/worker"
	crmworker "meta_commerce/internal/api/crm/worker"
	crmvc "meta_commerce/internal/api/crm/service"
	ruleintelmigration "meta_commerce/internal/api/ruleintel/migration"
	basesvc "meta_commerce/internal/api/base/service"
	_ "meta_commerce/internal/api/ads"         // Đăng ký executor domain ads + deferred + event types (init executor.go)
	_ "meta_commerce/internal/executors/cix" // Đăng ký cix executor với approval (init)
	approval "meta_commerce/internal/approval"
	"meta_commerce/internal/delivery"
	"meta_commerce/internal/global"
	"meta_commerce/internal/logger"
	"meta_commerce/internal/systemalert"
	"meta_commerce/internal/worker"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// initLogger khởi tạo và cấu hình logger cho toàn bộ ứng dụng
func initLogger() {
	// Khởi tạo logger với cấu hình mặc định
	// Logger sẽ tự động đọc environment variables để cấu hình
	if err := logger.Init(nil); err != nil {
		panic(fmt.Sprintf("Failed to initialize logger: %v", err))
	}

	// Log thông tin khởi tạo bằng logger mới
	log := logger.GetAppLogger()
	log.Info("Logger system initialized successfully")
}

// runBackfillConvAndExit chạy BackfillActivity(types=conversation) rồi thoát. Dùng khi token hết hạn.
func runBackfillConvAndExit(argIdx int) {
	orgIDStr := "698c341c977ebc6295312ad8"
	if argIdx+1 < len(os.Args) && os.Args[argIdx+1] != "" && os.Args[argIdx+1][0] != '-' {
		orgIDStr = os.Args[argIdx+1]
	}
	orgID, err := primitive.ObjectIDFromHex(orgIDStr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ownerOrganizationId không hợp lệ: %v\n", err)
		os.Exit(1)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	svc, err := crmvc.NewCrmCustomerService()
	if err != nil {
		fmt.Fprintf(os.Stderr, "NewCrmCustomerService: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Chạy BackfillActivity(types=conversation) cho org %s...\n", orgIDStr)
	result, err := svc.BackfillActivity(ctx, orgID, 0, []string{"conversation"}, nil, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "BackfillActivity: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("✓ Xong. Conversations processed: %d, logged: %d, skipped: %d\n",
		result.ConversationsProcessed, result.ConversationsLogged, result.ConversationsSkippedNoResolve)
	os.Exit(0)
}

// runSeedRuleIntelAndExit chạy seed Rule Intelligence (Ads + CRM) rồi thoát.
func runSeedRuleIntelAndExit() {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	initCtx := basesvc.WithSystemDataInsertAllowed(ctx)
	if err := ruleintelmigration.SeedRuleAdsSystem(initCtx); err != nil {
		fmt.Fprintf(os.Stderr, "SeedRuleAdsSystem: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("✓ SeedRuleAdsSystem xong")
	if err := ruleintelmigration.SeedRuleCrmSystem(initCtx); err != nil {
		fmt.Fprintf(os.Stderr, "SeedRuleCrmSystem: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("✓ SeedRuleCrmSystem xong")
	if err := ruleintelmigration.SeedRuleAidecisionDispatch(initCtx); err != nil {
		fmt.Fprintf(os.Stderr, "SeedRuleAidecisionDispatch: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("✓ SeedRuleAidecisionDispatch xong")
	if err := ruleintelmigration.SeedRuleAidecisionContextPolicy(initCtx); err != nil {
		fmt.Fprintf(os.Stderr, "SeedRuleAidecisionContextPolicy: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("✓ SeedRuleAidecisionContextPolicy xong")
	if err := ruleintelmigration.SeedRuleAidecisionSideEffectPolicy(initCtx); err != nil {
		fmt.Fprintf(os.Stderr, "SeedRuleAidecisionSideEffectPolicy: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("✓ SeedRuleAidecisionSideEffectPolicy xong")
	fmt.Println("Chạy go run scripts/check_ruleintel_db.go để kiểm tra.")
	os.Exit(0)
}

// runSeedAidecisionDispatchAndExit chỉ seed RULE_DECISION_CONSUMER_DISPATCH (+ logic/param/output) rồi thoát.
func runSeedAidecisionDispatchAndExit() {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	initCtx := basesvc.WithSystemDataInsertAllowed(ctx)
	if err := ruleintelmigration.SeedRuleAidecisionDispatch(initCtx); err != nil {
		fmt.Fprintf(os.Stderr, "SeedRuleAidecisionDispatch: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("✓ SeedRuleAidecisionDispatch xong")
	if err := ruleintelmigration.SeedRuleAidecisionContextPolicy(initCtx); err != nil {
		fmt.Fprintf(os.Stderr, "SeedRuleAidecisionContextPolicy: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("✓ SeedRuleAidecisionContextPolicy xong")
	if err := ruleintelmigration.SeedRuleAidecisionSideEffectPolicy(initCtx); err != nil {
		fmt.Fprintf(os.Stderr, "SeedRuleAidecisionSideEffectPolicy: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("✓ SeedRuleAidecisionSideEffectPolicy xong")
	os.Exit(0)
}

// runSyncZaloCustomersOnStart sync fb_customers nguồn Zalo (pageId pzl_) vào crm_customers khi server khởi động.
func runSyncZaloCustomersOnStart(ctx context.Context, log *logrus.Logger) {
	defer func() {
		if r := recover(); r != nil {
			log.WithField("panic", r).Error("[CRM] SyncZaloCustomers: Panic recovered")
		}
	}()
	log.Info("[CRM] SyncZaloCustomers: Bắt đầu sync khách Zalo vào crm_customers")
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	svc, err := crmvc.NewCrmCustomerService()
	if err != nil {
		log.WithError(err).Warn("[CRM] SyncZaloCustomers: NewCrmCustomerService thất bại, bỏ qua")
		return
	}
	count, err := svc.SyncZaloCustomersOnly(ctx)
	if err != nil {
		log.WithError(err).Warn("[CRM] SyncZaloCustomers: sync thất bại")
		return
	}
	log.WithField("count", count).Info("[CRM] SyncZaloCustomers: Hoàn thành — đã merge khách Zalo vào crm_customers")
}

// runRecalcAllCustomersOnStart recalc toàn bộ khách hàng của tất cả org khi server khởi động.
// Hàm tạm — dùng worker pool để tăng tốc.
func runRecalcAllCustomersOnStart(ctx context.Context, log *logrus.Logger) {
	defer func() {
		if r := recover(); r != nil {
			log.WithField("panic", r).Error("[CRM] RecalcAll: Panic recovered")
		}
	}()
	log.Info("[CRM] RecalcAll: enqueue recalc toàn bộ khách (AI Decision queue)")
	ctx, cancel := context.WithTimeout(ctx, 60*time.Minute)
	defer cancel()
	poolSize := worker.GetEffectivePoolSize(12, worker.PriorityLow)
	eventID, err := crmqueue.EmitCrmIntelligenceRecalculateAllOrgsRequested(ctx, poolSize)
	if err != nil {
		log.WithError(err).Warn("[CRM] RecalcAll: ghi event thất bại")
		return
	}
	log.WithFields(map[string]interface{}{
		"eventId": eventID,
	}).Info("[CRM] RecalcAll: Đã ghi event — consumer AI Decision sẽ chạy RecalculateAllCustomersForAllOrgs")
}

// runRecalcMismatchOnStart chạy RecalculateMismatchCustomers và RecalculateOrderCountMismatchCustomers trong goroutine khi khởi động.
// 1. Engaged crm nhưng visitor trong activity snapshot
// 2. First/repeat/vip/inactive — recalc để đảm bảo metrics khớp DB
func runRecalcMismatchOnStart(ctx context.Context, orgID primitive.ObjectID, limit int, log *logrus.Logger) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Fprintf(os.Stderr, "[CRM] RecalcMismatch: Panic recovered, tiến trình không bị dừng: %v\n", r)
		}
	}()
	ctx, cancel := context.WithTimeout(ctx, 60*time.Minute)
	defer cancel()
	// 1. Engaged vs visitor mismatch
	log.WithFields(map[string]interface{}{"orgId": orgID.Hex(), "limit": limit}).
		Info("[CRM] RecalcMismatch: enqueue engaged/visitor mismatch (AI Decision)")
	poolSize1 := worker.GetEffectivePoolSize(10, worker.PriorityLow)
	ev1, err := crmqueue.EmitCrmIntelligenceRecalculateMismatchRequested(ctx, orgID, limit, poolSize1)
	if err != nil {
		log.WithError(err).Error("[CRM] RecalcMismatch: ghi event engaged/visitor thất bại")
		return
	}
	log.WithField("eventId", ev1).Info("[CRM] RecalcMismatch: Đã ghi event engaged/visitor")

	// 2. Order count mismatch (first/repeat/vip/inactive)
	log.Info("[CRM] RecalcMismatch: enqueue order count mismatch (AI Decision)")
	poolSize2 := worker.GetEffectivePoolSize(12, worker.PriorityLow)
	ev2, err := crmqueue.EmitCrmIntelligenceRecalculateOrderCountMismatchRequested(ctx, orgID, limit, poolSize2)
	if err != nil {
		log.WithError(err).Error("[CRM] RecalcMismatch: ghi event order count thất bại")
		return
	}
	log.WithField("eventId", ev2).Info("[CRM] RecalcMismatch: Đã ghi event order count")
}

// main_thread khởi tạo và chạy Fiber server
// normalizeListenAddress chuẩn hóa địa chỉ listen (hỗ trợ cả "8080" và ":8080")
func normalizeListenAddress(addr string) string {
	if addr == "" {
		return ":8080"
	}
	if addr[0] == ':' {
		return addr
	}
	return ":" + addr
}

func main_thread() {
	// Khởi tạo app với cấu hình
	app := InitFiberApp()

	// Khởi động server với cấu hình listen
	cfg := global.MongoDB_ServerConfig
	address := normalizeListenAddress(cfg.Address)

	log := logger.GetAppLogger()
	log.Info("Starting Fiber server...")

	// Kiểm tra port có sẵn sàng trước khi Listen (tránh crash im lặng khi port đã bị chiếm)
	if ln, err := net.Listen("tcp", address); err != nil {
		errMsg := fmt.Sprintf("❌ [SERVER] Không thể bind port %s: %v. Có thể port đã được sử dụng bởi process khác. Thử đổi ADDRESS trong env hoặc tắt process đang dùng port.", address, err)
		log.Error(errMsg)
		fmt.Fprintln(os.Stderr, errMsg)
		os.Exit(1)
	} else {
		ln.Close()
	}
	
	// Helper function để resolve đường dẫn từ thư mục api
	resolvePath := func(path string) string {
		if filepath.IsAbs(path) {
			return path
		}
		// Tìm thư mục api
		currentDir, err := os.Getwd()
		if err != nil {
			return path
		}
		for {
			envDir := filepath.Join(currentDir, "config", "env")
			if _, err := os.Stat(envDir); err == nil {
				return filepath.Join(currentDir, path)
			}
			parentDir := filepath.Dir(currentDir)
			if parentDir == currentDir {
				return path
			}
			currentDir = parentDir
		}
	}

	// Kiểm tra xem có bật TLS không
	if cfg.EnableTLS && cfg.TLSCertFile != "" && cfg.TLSKeyFile != "" {
		// Resolve đường dẫn certificate và key từ thư mục api
		certPath := resolvePath(cfg.TLSCertFile)
		keyPath := resolvePath(cfg.TLSKeyFile)
		
		// Kiểm tra file certificate và key tồn tại
		if _, err := os.Stat(certPath); os.IsNotExist(err) {
			errMsg := fmt.Sprintf("TLS certificate file not found: %s (resolved from: %s)", certPath, cfg.TLSCertFile)
			log.Error(errMsg)
			fmt.Fprintln(os.Stderr, errMsg)
			os.Exit(1)
		}
		if _, err := os.Stat(keyPath); os.IsNotExist(err) {
			errMsg := fmt.Sprintf("TLS key file not found: %s (resolved from: %s)", keyPath, cfg.TLSKeyFile)
			log.Error(errMsg)
			fmt.Fprintln(os.Stderr, errMsg)
			os.Exit(1)
		}

		// Load certificate và key
		cert, err := tls.LoadX509KeyPair(certPath, keyPath)
		if err != nil {
			errMsg := fmt.Sprintf("Error loading TLS certificate: %v", err)
			log.Error(errMsg)
			fmt.Fprintln(os.Stderr, errMsg)
			os.Exit(1)
		}

		// Tạo listener với TLS
		ln, err := net.Listen("tcp", address)
		if err != nil {
			errMsg := fmt.Sprintf("Error creating listener: %v", err)
			log.Error(errMsg)
			fmt.Fprintln(os.Stderr, errMsg)
			os.Exit(1)
		}
		
		// Cấu hình TLS
		tlsConfig := &tls.Config{
			Certificates: []tls.Certificate{cert},
			MinVersion:   tls.VersionTLS12,
		}
		
		// Wrap listener với TLS
		tlsListener := tls.NewListener(ln, tlsConfig)
		
		log.WithFields(map[string]interface{}{
			"address": address,
			"cert":    certPath,
			"key":     keyPath,
		}).Info("Starting server with HTTPS/TLS")
		
		// Khởi động server với TLS listener
		if err := app.Listener(tlsListener); err != nil {
			errMsg := fmt.Sprintf("❌ [SERVER] Fiber Listener với TLS thất bại: %v", err)
			log.Error(errMsg)
			fmt.Fprintln(os.Stderr, errMsg)
			os.Exit(1)
		}
	} else {
		// Khởi động server HTTP thông thường
		log.WithFields(map[string]interface{}{
			"address":  address,
			"protocol": "HTTP",
		}).Info("Starting server with HTTP")

		listenConfig := fiber.ListenConfig{}
		if err := app.Listen(address, listenConfig); err != nil {
			errMsg := fmt.Sprintf("❌ [SERVER] Fiber Listen thất bại trên %s: %v", address, err)
			log.Error(errMsg)
			fmt.Fprintln(os.Stderr, errMsg)
			os.Exit(1)
		}
	}
}

// Hàm main
func main() {
	// Khởi tạo logger
	initLogger()

	// Khởi tạo các biến toàn cục
	InitGlobal()

	// Khởi tạo registry
	InitRegistry()

	// Trung tâm chỉ huy AI Decision: queue depth trong RAM; đồng bộ Mongo lần đầu rồi mỗi ~5 phút (AI_DECISION_METRICS_RECONCILE_SEC)
	decisionlive.StartCommandCenterReconciler(context.Background())

	// Khởi tạo cơ chế duyệt (pkg/approval engine + bridge)
	approval.Init()

	// Subcommand: --backfill-conv [ownerOrganizationId] — chạy backfill conversation rồi thoát
	for i, arg := range os.Args {
		if arg == "--backfill-conv" {
			runBackfillConvAndExit(i)
		}
	}

	// Khởi tạo dữ liệu mặc định
	InitDefaultData()

	// Subcommand: --seed-ruleintel — chạy seed Rule Intelligence rồi thoát (dùng khi DB thiếu rules)
	for _, arg := range os.Args {
		if arg == "--seed-ruleintel" {
			runSeedRuleIntelAndExit()
		}
	}

	// Subcommand: --seed-aidecision-dispatch — chỉ seed rule routing consumer AI Decision (nhẹ hơn --seed-ruleintel)
	for _, arg := range os.Args {
		if arg == "--seed-aidecision-dispatch" {
			runSeedAidecisionDispatchAndExit()
		}
	}

	// Lấy base URL từ environment variable hoặc dùng default
	baseURL := os.Getenv("BASE_URL")
	if baseURL == "" {
		cfg := global.MongoDB_ServerConfig
		protocol := "http"
		if cfg.EnableTLS {
			protocol = "https"
		}
		baseURL = fmt.Sprintf("%s://localhost:%s", protocol, cfg.Address)
	}

	log := logger.GetAppLogger()

	// Đăng ký callback cảnh báo CPU/RAM/disk quá tải (phải gọi trước khi khởi động workers)
	systemalert.Register()

	// Đăng ký tất cả workers vào Registry thống nhất
	reg := worker.DefaultRegistry()

	// Worker Controller: lấy mẫu CPU định kỳ, throttle workers khi quá tải — đăng ký đầu tiên
	reg.Register("system_worker_controller", worker.DefaultController())

	// Delivery Processor (Hệ thống 1)
	if processor, err := delivery.NewProcessor(baseURL); err != nil {
		log.WithError(err).Error("Failed to create delivery processor, continuing without delivery worker")
		reg.Register(worker.WorkerDelivery, nil)
	} else {
		reg.Register(worker.WorkerDelivery, processor)
	}

	// Command Cleanup Worker (Module 2)
	if w, err := worker.NewCommandCleanupWorker(1*time.Minute, 300); err != nil {
		log.WithError(err).Error("Failed to create command cleanup worker")
		reg.Register(worker.WorkerCommandCleanup, nil)
	} else {
		reg.Register(worker.WorkerCommandCleanup, w)
	}

	// Agent Command Cleanup Worker
	if w, err := worker.NewAgentCommandCleanupWorker(1*time.Minute, 300); err != nil {
		log.WithError(err).Error("Failed to create agent command cleanup worker")
		reg.Register(worker.WorkerAgentCommandCleanup, nil)
	} else {
		reg.Register(worker.WorkerAgentCommandCleanup, w)
	}

	// Agent Activity Cleanup Worker
	if w, err := worker.NewAgentActivityCleanupWorker(1*time.Hour, 1); err != nil {
		log.WithError(err).Error("Failed to create agent activity cleanup worker")
		reg.Register(worker.WorkerAgentActivityCleanup, nil)
	} else {
		reg.Register(worker.WorkerAgentActivityCleanup, w)
	}

	// Report Dirty Workers — 3 worker độc lập (ads, order, customer), config riêng qua API/env
	reportDirtyDomains := []struct {
		domain string
		name  string
	}{
		{"ads", worker.WorkerReportDirtyAds},
		{"order", worker.WorkerReportDirtyOrder},
		{"customer", worker.WorkerReportDirtyCustomer},
	}
	for _, d := range reportDirtyDomains {
		if w, err := worker.NewReportDirtyWorker(d.domain); err != nil {
			log.WithError(err).WithField("domain", d.domain).Warn("Failed to create report dirty worker")
			reg.Register(d.name, nil)
		} else {
			reg.Register(d.name, w)
		}
	}

	// Report: flush touch trong RAM → MarkDirty (multi-rate ads/order/customer; poll REPORT_REDIS_TOUCH_POLL_TICK_SEC)
	reg.Register(worker.WorkerReportRedisTouchFlush, worker.NewReportRedisTouchFlushWorker())

	// CRM Ingest Worker
	reg.Register(worker.WorkerCrmPendingMerge, worker.NewCrmPendingMergeWorker(30*time.Second, 50))

	// CRM Intel Compute — poll crm_intel_compute (domain CRM; không tính trong consumer AI Decision)
	reg.Register(worker.WorkerCrmIntelCompute, crmworker.NewCrmIntelComputeWorker(3*time.Second))

	// CRM Bulk Worker
	if w, err := worker.NewCrmBulkWorker(2*time.Minute, 2); err != nil {
		log.WithError(err).Warn("Failed to create CRM Bulk Worker")
		reg.Register(worker.WorkerCrmBulk, nil)
	} else {
		reg.Register(worker.WorkerCrmBulk, w)
	}

	// Ads Workers
	reg.Register(worker.WorkerAdsExecution, adsworker.NewAdsExecutionWorker(30*time.Second, 10))
	reg.Register(worker.WorkerAdsAutoPropose, aidecisionworker.NewAdsAutoProposeWorker(30*time.Minute, baseURL))
	reg.Register(worker.WorkerAdsCircuitBreaker, adsworker.NewAdsCircuitBreakerWorker(10*time.Minute))
	reg.Register(worker.WorkerAdsDailyScheduler, adsworker.NewAdsDailySchedulerWorker(1*time.Minute, baseURL))
	reg.Register(worker.WorkerAdsPancakeHeartbeat, adsworker.NewAdsPancakeHeartbeatWorker(15*time.Minute))
	reg.Register(worker.WorkerAdsCounterfactual, adsworker.NewAdsCounterfactualWorker(30*time.Minute))

	// Classification Refresh Workers
	if w, err := worker.NewClassificationRefreshWorker(24*time.Hour, 200, worker.ClassificationRefreshModeFull); err != nil {
		log.WithError(err).Warn("Failed to create classification refresh full worker")
		reg.Register(worker.WorkerClassificationFull, nil)
	} else {
		reg.Register(worker.WorkerClassificationFull, w)
	}
	if w, err := worker.NewClassificationRefreshWorker(6*time.Hour, 200, worker.ClassificationRefreshModeSmart); err != nil {
		log.WithError(err).Warn("Failed to create classification refresh smart worker")
		reg.Register(worker.WorkerClassificationSmart, nil)
	} else {
		reg.Register(worker.WorkerClassificationSmart, w)
	}

	// CIX Intel Compute — poll cix_intel_compute, phân tích hội thoại qua Rule Engine
	reg.Register(worker.WorkerCixIntelCompute, cixworker.NewCixIntelComputeWorker(30*time.Second, 50))

	// AI Decision Consumer — decision_events_queue (gồm cix.analysis_requested → enqueue cix_intel_compute, giống CRM/Ads/Order)
	reg.Register(worker.WorkerAIDecisionConsumer, aidecisionworker.NewAIDecisionConsumerWorker(2*time.Second))

	// AI Decision Debounce Worker — flush debounce state hết window → message.batch_ready (chạy khi AI_DECISION_DEBOUNCE_ENABLED=true)
	reg.Register(worker.WorkerAIDecisionDebounce, aidecisionworker.NewAIDecisionDebounceWorker(5*time.Second))

	// AI Decision Closure Worker — đóng case quá hạn với closed_timeout (AI_DECISION_CLOSURE_MAX_AGE_HOURS=24)
	reg.Register(worker.WorkerAIDecisionClosure, aidecisionworker.NewAIDecisionClosureWorker(10*time.Minute))

	// Order Intel Compute — poll order_intel_compute, tính Raw→L3→Flags tại domain
	reg.Register(worker.WorkerOrderIntelCompute, orderintelworker.NewOrderIntelComputeWorker(3*time.Second))

	// Ads Intel Compute — poll ads_intel_compute (domain ads; không tính trong consumer AI Decision)
	reg.Register(worker.WorkerAdsIntelCompute, adsworker.NewAdsIntelComputeWorker(3*time.Second))

	// CRM Context Worker — consume customer.context_requested → load customer → emit customer.context_ready
	reg.Register(worker.WorkerCrmContext, crmworker.NewCrmContextWorker(5*time.Second))

	// ads.context_requested → enqueue ads_intel_compute (context_ready) → worker domain đọc meta_campaigns → emit ads.context_ready.

	// Learning Rule Suggestion Worker — Phase 3: phân tích learning_cases → rule suggestions (LEARNING_RULE_SUGGESTION_ENABLED=true)
	reg.Register(worker.WorkerLearningRuleSuggestion, learningworker.NewLearningRuleSuggestionWorker(1*time.Hour))

	// Learning Evaluation Worker — batch tính evaluation cho learning_cases
	reg.Register(worker.WorkerLearningEvaluation, learningworker.NewLearningEvaluationWorker(5*time.Minute, 50))

	// Learning Insight Aggregate Worker — Phase 3: aggregate anonymized cross-merchant (stub)
	reg.Register(worker.WorkerLearningInsightAggregate, nil)

	// Identity Backfill Worker — backfill uid, sourceIds, links cho doc cũ (4 lớp identity)
	reg.Register(worker.WorkerIdentityBackfill, worker.NewIdentityBackfillWorker(10*time.Minute, 500))

	// Context chung cho tất cả workers — cancel khi shutdown
	ctxWorkers, cancelWorkers := context.WithCancel(context.Background())
	defer cancelWorkers()

	// Khởi động tất cả workers qua Registry (panic recovery tích hợp sẵn)
	reg.StartAll(ctxWorkers)
	log.WithFields(map[string]interface{}{"count": reg.Count()}).Info("Đã khởi động workers qua Registry")

	// Recalc toàn bộ khách hàng khi khởi động — đã tắt. Bật lại bằng CRM_RECALC_ALL_ON_START=1 nếu cần.
	// if os.Getenv("CRM_RECALC_ALL_ON_START") == "1" {
	// 	go runRecalcAllCustomersOnStart(ctxWorkers, log)
	// }

	// Recalc mismatch khi khởi động — dành cho đồng bộ thủ công. Mặc định tắt.
	// Bật bằng CRM_RECALC_MISMATCH_ON_START=1, CRM_RECALC_MISMATCH_ORG, CRM_RECALC_MISMATCH_LIMIT.
	// if os.Getenv("CRM_RECALC_MISMATCH_ON_START") == "1" {
	// 	orgStr := os.Getenv("CRM_RECALC_MISMATCH_ORG")
	// 	if orgStr == "" {
	// 		orgStr = "69a655f0088600c32e62f955"
	// 	}
	// 	limit := 0
	// 	if s := os.Getenv("CRM_RECALC_MISMATCH_LIMIT"); s != "" {
	// 		if n, err := strconv.Atoi(s); err == nil && n >= 0 {
	// 			limit = n
	// 		}
	// 	}
	// 	orgID, errOrg := primitive.ObjectIDFromHex(orgStr)
	// 	if errOrg == nil {
	// 		go runRecalcMismatchOnStart(ctxWorkers, orgID, limit, log)
	// 	} else {
	// 		log.WithError(errOrg).Warn("CRM_RECALC_MISMATCH_ON_START: ownerOrganizationId không hợp lệ, bỏ qua")
	// 	}
	// }

	// Chạy Fiber server trên main thread (blocking)
	main_thread()
}
