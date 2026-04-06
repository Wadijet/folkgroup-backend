// Package datachanged — Đăng ký side-effect hồ sơ Ads (debounce meta hooks) sau datachanged.
package datachanged

import (
	"meta_commerce/internal/api/aidecision/datachangedsidefx"
)

func init() {
	datachangedsidefx.Register(30, "meta_ads_profile", func(ac *datachangedsidefx.ApplyContext) error {
		if !ac.Route.AdsProfilePipeline {
			return nil
		}
		if !ac.Dec.AllowAds {
			return nil
		}
		ProcessAdsProfileFromDataChange(ac.Ctx, ac.E)
		return nil
	})
}
