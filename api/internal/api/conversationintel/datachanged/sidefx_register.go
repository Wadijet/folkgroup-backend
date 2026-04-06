// Package datachanged — Đăng ký side-effect CIX (cix_intel_compute) sau datachanged fb_message_items.
package datachanged

import (
	"meta_commerce/internal/api/aidecision/datachangedsidefx"
	"meta_commerce/internal/api/aidecision/eventintake"
	"meta_commerce/internal/global"
	"meta_commerce/internal/logger"
)

func init() {
	datachangedsidefx.Register(40, "cix_intel_compute", func(ac *datachangedsidefx.ApplyContext) error {
		if ac.Src != global.MongoDB_ColNames.FbMessageItems {
			return nil
		}
		if !ac.Route.CixIntelPipeline {
			return nil
		}
		if ac.CixIntelDefer > 0 {
			eventintake.ScheduleDeferredSideEffect(eventintake.DeferredKindCixIntelCompute, ac.OrgHex, ac.Src, ac.IDHex, ac.CixIntelDefer)
			return nil
		}
		if err := EnqueueCixComputeFromDataChange(ac.Ctx, ac.E, ac.IDHex); err != nil {
			logger.GetAppLogger().WithError(err).WithFields(map[string]interface{}{
				"eventId": ac.Evt.EventID, "orgHex": ac.OrgHex, "sourceCollection": ac.Src,
			}).Warn("📋 [CIX_INTEL] Không xếp job cix_intel_compute từ fb_message_items datachanged")
		}
		return nil
	})
}
