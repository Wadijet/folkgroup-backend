package main

import (
	"context"
	"reflect"
	"sort"

	"meta_commerce/config"
	"meta_commerce/internal/api/aidecision/eventpipeline"
	aidecisionhooks "meta_commerce/internal/api/aidecision/hooks"
	aidecisionsvc "meta_commerce/internal/api/aidecision/service"
	crmvc "meta_commerce/internal/api/crm/service"
	learningsvc "meta_commerce/internal/api/learning/service"
	"meta_commerce/internal/global"
	"meta_commerce/internal/utility/identity"
	pkgapproval "meta_commerce/pkg/approval"

	"github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/mongo"
)

func InitRegistry() {
	// Luồng: thay đổi collection nguồn → OnDataChanged → AI Decision → proposals → Executor → Learning
	logrus.Info("Initialized registry")

	// Khởi tạo registry và đăng ký các collections
	err := InitCollections(global.MongoDB_Session, global.MongoDB_ServerConfig)
	if err != nil {
		logrus.Fatalf("Failed to initialize collections: %v", err)
	}
	logrus.Info("Initialized collection registry")

	// Đăng ký identity resolver (external id → uid) cho enrich links khi InsertOne
	if crmSvc, err := crmvc.NewCrmCustomerService(); err == nil {
		identity.SetDefaultResolver(&crmvc.CrmResolver{CrmCustomerService: crmSvc})
		logrus.Info("Identity resolver (CRM) registered")
	} else {
		logrus.Warnf("Identity resolver chưa đăng ký (CRM service: %v)", err)
	}

	// Rule Intelligence (seed system Ads/CRM/CIX/AI Decision dispatch) — InitDefaultData Step 1b, sau System Organization.

	// Một nơi nạp + tài liệu luồng datachanged (chi tiết: internal/api/aidecision/eventpipeline).
	eventpipeline.EnsureSideEffectModulesLoaded()
	for _, line := range eventpipeline.LogLines() {
		logrus.Info(line)
	}

	// Đồng bộ: L1 DoSyncUpsert giảm ghi DB. CRUD → OnDataChanged (L2 cổng enqueue) → decision_events_queue → consumer (CRM/Report/Ads).
	decSvc := aidecisionsvc.NewAIDecisionService()
	aidecisionhooks.RegisterAIDecisionOnDataChanged(decSvc)
	logrus.Info("AI Decision: OnDataChanged (L2 cổng queue) → decision_events_queue → consumer CRM/Report/Ads")

	// Đăng ký OnActionClosed: khi action đóng (executed/rejected/failed) — tham số closureType hiện là status cuối (engine), không dùng trực tiếp ở Learning.
	// Learning đọc decisionCaseId + trace từ ActionPending / payload; bỏ qua ghi khi closure decision case không đủ (vision_policy).
	pkgapproval.OnActionClosed = func(ctx context.Context, domain string, doc *pkgapproval.ActionPending, closureType string) {
		_ = closureType
		if doc == nil {
			return
		}
		_, _ = learningsvc.CreateLearningCaseFromAction(ctx, doc)
	}
	logrus.Info("Approval OnActionClosed (Learning per action) registered")
}

// InitCollections khởi tạo và đăng ký các collections MongoDB
func InitCollections(client *mongo.Client, cfg *config.Configuration) error {
	db := client.Database(cfg.MongoDB_DBName_Auth)
	colNames := []string{"auth_rel_organization_shares"}
	seen := map[string]struct{}{"auth_rel_organization_shares": {}}
	val := reflect.ValueOf(global.MongoDB_ColNames)
	for i := 0; i < val.NumField(); i++ {
		name := val.Field(i).String()
		if name == "" {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		colNames = append(colNames, name)
	}
	sort.Strings(colNames)

	for _, name := range colNames {
		registered, err := global.RegistryCollections.Register(name, db.Collection(name))
		if err != nil {
			logrus.Errorf("Failed to register collection %s: %v", name, err)
			return err
		}

		if registered {
			logrus.Infof("Collection %s registered successfully", name)
		} else {
			logrus.Errorf("Collection %s already registered", name)
		}

	}

	return nil
}
