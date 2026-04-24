// Package datachanged — Đăng ký side-effect CIX (cix_intel_compute) sau datachanged fb_message_items.
package datachanged

import (
	"strings"

	"meta_commerce/internal/api/aidecision/crmqueue"
	"meta_commerce/internal/api/aidecision/datachangedsidefx"
	"meta_commerce/internal/api/aidecision/eventintake"
	"meta_commerce/internal/global"
	"meta_commerce/internal/logger"
)

func init() {
	datachangedsidefx.Register(40, "cix_job_intel", func(ac *datachangedsidefx.ApplyContext) error {
		if ac.Src != global.MongoDB_ColNames.FbMessageItems {
			return nil
		}
		if !ac.Route.CixIntelPipeline {
			return nil
		}
		if ac.CixIntelDefer > 0 {
			var tid, cid string
			if ac.Evt != nil {
				tid = strings.TrimSpace(ac.Evt.TraceID)
				cid = strings.TrimSpace(ac.Evt.CorrelationID)
			}
			return eventintake.ScheduleDeferredSideEffect(ac.Ctx, eventintake.DeferredKindCixIntelCompute, ac.OrgHex, ac.Src, ac.IDHex, ac.CixIntelDefer, tid, cid)
		}
		var tid, cid string
		if ac.Evt != nil {
			tid = strings.TrimSpace(ac.Evt.TraceID)
			cid = strings.TrimSpace(ac.Evt.CorrelationID)
		}
		cixBus := crmqueue.CompleteDomainJobBus(crmqueue.DomainQueueBusFieldsPtrFromDecisionEvent(ac.Evt), crmqueue.ProcessorDomainCIX, crmqueue.EnqueueSourceConversationIntel)
		if err := EnqueueCixComputeFromDataChange(ac.Ctx, ac.E, ac.IDHex, tid, cid, cixBus); err != nil {
			logger.GetAppLogger().WithError(err).WithFields(map[string]interface{}{
				"eventId": ac.Evt.EventID, "orgHex": ac.OrgHex, "sourceCollection": ac.Src,
			}).Warn("📋 [CIX_INTEL] Không xếp job cix_intel_compute từ fb_message_items datachanged")
		}
		return nil
	})
}
