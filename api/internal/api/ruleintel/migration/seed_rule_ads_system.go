// Package migration — Seed toàn bộ Rule Intelligence cho Ads domain (FolkForm v4.1).
// Tất cả rules thuộc System Organization (OwnerOrganizationID + IsSystem=true).
// Dùng CRUD services Upsert.
package migration

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"meta_commerce/internal/api/ruleintel/models"
	"meta_commerce/internal/api/ruleintel/service"
)

// scriptSlA — Logic SL-A theo FolkForm v4.1 Rule 01 (full logic từ layer1).
var scriptSlA = `function evaluate(ctx) {
  var layer1 = ctx.layers.layer1 || {};
  var raw = ctx.layers.raw || {};
  var params = ctx.params || {};
  var report = { log: '' };
  if (layer1.lifecycle === 'NEW') {
    report.result = 'filtered';
    report.log = '1. Lifecycle: filtered — Campaign NEW (< 7 ngày)';
    return { output: null, report: report };
  }
  report.log = '1. Lifecycle: passed (' + (layer1.lifecycle || '') + ')';
  var convRate = layer1.convRate_7d || 0;
  var thConvException = params.th_convRateException || 0.2;
  if (convRate > thConvException) {
    report.result = 'no_match';
    report.log += '\n2. Exception: Conv_Rate ' + (convRate * 100).toFixed(1) + '% > ' + (thConvException * 100) + '% — không kill';
    return { output: null, report: report };
  }
  var spendPct = layer1.spendPct_7d || 0;
  var runtimeMin = layer1.runtimeMinutes || 0;
  var thSpend = params.th_spendPctBase || 0.2;
  var thRuntime = params.th_runtimeMin || 90;
  if (spendPct <= thSpend || runtimeMin <= thRuntime) {
    report.result = 'no_match';
    report.log += '\n2. Điều kiện nền: spendPct=' + (spendPct * 100).toFixed(1) + '% (cần >' + (thSpend * 100) + '%), runtime=' + runtimeMin + 'p (cần >' + thRuntime + 'p)';
    return { output: null, report: report };
  }
  var cpaMess = layer1.cpaMess_7d || 0;
  var mqs = layer1.mqs_7d || 999;
  var thCpa = params.th_cpaMessKill || 180000;
  var thMessMax = params.th_messMax || 3;
  var thMqsMin = params.th_mqsMin || 1;
  var mess = 999;
  if (raw && raw.meta && raw.meta.mess != null) { mess = Number(raw.meta.mess); }
  if (mqs >= (params.th_mqsDecreaseMin || 2)) {
    report.result = 'no_match';
    report.log += '\n2. MQS >= 2 → sl_a_decrease (rule khác), không kill';
    return { output: null, report: report };
  }
  if (cpaMess <= thCpa || mess >= thMessMax || mqs >= thMqsMin) {
    report.result = 'no_match';
    report.log += '\n2. sl_a: cpaMess=' + cpaMess + ' (cần >' + thCpa + '), mess=' + mess + ' (<' + thMessMax + '), mqs=' + mqs + ' (<' + thMqsMin + ')';
    return { output: null, report: report };
  }
  report.result = 'match';
  report.log += '\n2. sl_a match\n3. Kết quả: PAUSE sl_a';
  var action = { action_code: 'PAUSE', ruleCode: 'sl_a', reason: 'Hệ thống đề xuất [SL-A]: CPA mess cao, mess thấp, MQS thấp — Stop Loss', value: null };
  if (params.resultCheckConfig) { action.result_check = params.resultCheckConfig; }
  return { output: action, report: report };
}`

// scriptFlagMoEligible — Morning On Eligible: cpaMess < th, convRate >= 8%, chs >= 60, orders >= 1, mess >= 3, freq < 3.
// Input: layer1, layer3, raw.meta, pancake.pos, params (th_cpaMessMoMax).
var scriptFlagMoEligible = `function evaluate(ctx) {
  var layer1 = ctx.layers.layer1 || {}; var layer3 = ctx.layers.layer3 || {}; var raw = ctx.layers.raw || {};
  var meta = raw.meta || {}; var pos = (raw.pancake && raw.pancake.pos) || {}; var params = ctx.params || {};
  var report = { log: '' };
  var toF = function(m, k) { var v = m[k]; if (v == null) return 0; if (typeof v === 'number') return v; if (typeof v === 'string') return parseFloat(v) || 0; return 0; };
  var toI = function(m, k) { var v = m[k]; if (v == null) return 0; return parseInt(v, 10) || 0; };
  var cpaMess = toF(layer1, 'cpaMess_7d'); var convRate = toF(layer1, 'convRate_7d'); var chs = toF(layer3, 'chs');
  var orders = toI(pos, 'orders'); var mess = toI(meta, 'mess'); var freq = toF(meta, 'frequency');
  var thCpa = params.th_cpaMessMoMax || 216000;
  report.log = '1. cpaMess=' + cpaMess + ', convRate=' + (convRate*100).toFixed(1) + '%, chs=' + chs + ', orders=' + orders + ', mess=' + mess + ', freq=' + freq;
  if (cpaMess >= thCpa) { report.result = 'no_match'; report.log += '\n2. cpaMess>=' + thCpa + ' → no_match'; return { output: null, report: report }; }
  if (convRate < 0.08) { report.result = 'no_match'; report.log += '\n2. convRate<8% → no_match'; return { output: null, report: report }; }
  if (chs < 60) { report.result = 'no_match'; report.log += '\n2. chs<60 → no_match'; return { output: null, report: report }; }
  if (orders < 1) { report.result = 'no_match'; report.log += '\n2. orders<1 → no_match'; return { output: null, report: report }; }
  if (mess < 3) { report.result = 'no_match'; report.log += '\n2. mess<3 → no_match'; return { output: null, report: report }; }
  if (freq >= 3) { report.result = 'no_match'; report.log += '\n2. freq>=3 → no_match'; return { output: null, report: report }; }
  report.result = 'match'; report.log += '\n2. mo_eligible match — camp đủ điều kiện bật lại sáng';
  return { output: { flag: 'mo_eligible', value: true }, report: report };
}`

// scriptFlagSlB — SL-B: Spend > 30% (NORMAL) / 20% (BLITZ/PROTECT), runtime > 90, mess = 0.
// Input: layer1, layer2 (currentMode), raw.meta, params.
var scriptFlagSlB = `function evaluate(ctx) {
  var layer1 = ctx.layers.layer1 || {}; var layer2 = ctx.layers.layer2 || {}; var raw = ctx.layers.raw || {};
  var meta = raw.meta || {}; var pos = (raw.pancake && raw.pancake.pos) || {}; var params = ctx.params || {};
  var report = { log: '' }; var toF = function(m,k){var v=m[k];if(v==null)return 0;if(typeof v==='number')return v;return parseFloat(v)||0;};
  var toI = function(m,k){var v=m[k];if(v==null)return 0;return parseInt(v,10)||0;};
  var spendPct = toF(layer1,'spendPct_7d'); var runtime = toF(layer1,'runtimeMinutes'); var mess = toI(meta,'mess');
  var mode = (layer2.currentMode||'').toUpperCase(); var thSpend = (mode==='BLITZ'||mode==='PROTECT') ? (params.th_spendPctSlBBlitz||0.2) : (params.th_spendPctSlB||0.3);
  var thRuntime = params.th_runtimeMinutesBase || 90;
  report.log = '1. spendPct=' + (spendPct*100).toFixed(1) + '%, runtime=' + runtime + ', mess=' + mess + ', mode=' + mode + ', thSpend=' + (thSpend*100) + '%';
  if (spendPct <= thSpend && !(spendPct===0 && toF(meta,'spend')>0)) { report.result='no_match'; report.log += '\n2. spendPct<=' + thSpend + ' → no_match'; return {output:null,report:report}; }
  if (runtime <= thRuntime) { report.result='no_match'; report.log += '\n2. runtime<=' + thRuntime + ' → no_match'; return {output:null,report:report}; }
  if (mess !== 0) { report.result='no_match'; report.log += '\n2. mess!=0 → no_match'; return {output:null,report:report}; }
  report.result='match'; report.log += '\n2. sl_b match — Có spend, 0 mess'; return {output:{flag:'sl_b',value:true},report:report};
}`

// scriptFlagNoonCutEligible — Noon Cut: cpaMess > 144k, spendPct trong (20%, 55%), healthState IN (warning,critical).
// Input: layer1, layer3 (healthState), raw.meta, params.
var scriptFlagNoonCutEligible = `function evaluate(ctx) {
  var layer1 = ctx.layers.layer1 || {}; var layer3 = ctx.layers.layer3 || {}; var raw = ctx.layers.raw || {};
  var params = ctx.params || {}; var report = { log: '' };
  var toF = function(m,k){var v=m[k];if(v==null)return 0;if(typeof v==='number')return v;return parseFloat(v)||0;};
  var getS = function(m,k){var v=m[k];return (v!=null&&typeof v==='string')?v:'';};
  var cpaMess = toF(layer1,'cpaMess_7d'); var spendPct = toF(layer1,'spendPct_7d'); var healthState = getS(layer3,'healthState');
  var thCpa = params.th_cpaMessNoonCutMin || 144000; var thSpendMax = params.th_spendPctNoonCutMax || 0.55; var thSpendMin = params.th_spendPctBase || 0.2;
  report.log = '1. cpaMess=' + cpaMess + ', spendPct=' + (spendPct*100).toFixed(1) + '%, healthState=' + healthState;
  if (cpaMess <= thCpa) { report.result='no_match'; report.log += '\n2. cpaMess<=' + thCpa + ' → no_match'; return {output:null,report:report}; }
  if (spendPct >= thSpendMax) { report.result='no_match'; report.log += '\n2. spendPct>=' + (thSpendMax*100) + '% → no_match'; return {output:null,report:report}; }
  if (spendPct <= thSpendMin && !(spendPct===0 && toF((raw.meta||{}),'spend')>0)) { report.result='no_match'; report.log += '\n2. spendPct<=' + (thSpendMin*100) + '% → no_match'; return {output:null,report:report}; }
  if (healthState !== 'warning' && healthState !== 'critical') { report.result='no_match'; report.log += '\n2. healthState không warning/critical → no_match'; return {output:null,report:report}; }
  report.result='match'; report.log += '\n2. noon_cut_eligible match'; return {output:{flag:'noon_cut_eligible',value:true},report:report};
}`

// scriptFlagSafetyNet — Safety Net: orders >= 3, convRate >= 10%, chs >= 60.
// Input: layer1, layer3, pancake.pos, params.
var scriptFlagSafetyNet = `function evaluate(ctx) {
  var layer1 = ctx.layers.layer1 || {}; var layer3 = ctx.layers.layer3 || {}; var raw = ctx.layers.raw || {};
  var pos = (raw.pancake && raw.pancake.pos) || {}; var params = ctx.params || {}; var report = { log: '' };
  var toF = function(m,k){var v=m[k];if(v==null)return 0;if(typeof v==='number')return v;return parseFloat(v)||0;};
  var toI = function(m,k){var v=m[k];if(v==null)return 0;return parseInt(v,10)||0;};
  var orders = toI(pos,'orders'); var convRate = toF(layer1,'convRate_7d'); var chs = toF(layer3,'chs');
  var thOrders = params.th_safetyNetOrdersMin || 3; var thCr = params.th_safetyNetCrMin || 0.1; var thChs = params.th_chsMin || 60;
  report.log = '1. orders=' + orders + ', convRate=' + (convRate*100).toFixed(1) + '%, chs=' + chs.toFixed(1);
  if (orders < thOrders) { report.result='no_match'; report.log += '\n2. orders<' + thOrders + ' → no_match'; return {output:null,report:report}; }
  if (convRate < thCr) { report.result='no_match'; report.log += '\n2. convRate<' + (thCr*100) + '% → no_match'; return {output:null,report:report}; }
  if (chs < thChs) { report.result='no_match'; report.log += '\n2. chs<' + thChs + ' → no_match'; return {output:null,report:report}; }
  report.result='match'; report.log += '\n2. safety_net match'; return {output:{flag:'safety_net',value:true},report:report};
}`

// scriptFlagIncreaseEligible — Increase Eligible: convRate > 12%, freq < 2, spendPct > 45%, chs >= 60.
// Input: layer1, layer3, raw.meta, params.
var scriptFlagIncreaseEligible = `function evaluate(ctx) {
  var layer1 = ctx.layers.layer1 || {}; var layer3 = ctx.layers.layer3 || {}; var raw = ctx.layers.raw || {};
  var meta = raw.meta || {}; var params = ctx.params || {}; var report = { log: '' };
  var toF = function(m,k){var v=m[k];if(v==null)return 0;if(typeof v==='number')return v;return parseFloat(v)||0;};
  var convRate = toF(layer1,'convRate_7d'); var freq = toF(meta,'frequency'); var spendPct = toF(layer1,'spendPct_7d'); var chs = toF(layer3,'chs');
  report.log = '1. convRate=' + (convRate*100).toFixed(1) + '%, freq=' + freq + ', spendPct=' + (spendPct*100).toFixed(1) + '%, chs=' + chs.toFixed(1);
  if (convRate <= 0.12) { report.result='no_match'; report.log += '\n2. convRate<=12% → no_match'; return {output:null,report:report}; }
  if (freq >= 2) { report.result='no_match'; report.log += '\n2. freq>=2 → no_match'; return {output:null,report:report}; }
  if (spendPct <= 0.45) { report.result='no_match'; report.log += '\n2. spendPct<=45% → no_match'; return {output:null,report:report}; }
  if (chs < 60) { report.result='no_match'; report.log += '\n2. chs<60 → no_match'; return {output:null,report:report}; }
  report.result='match'; report.log += '\n2. increase_eligible match'; return {output:{flag:'increase_eligible',value:true},report:report};
}`

// scriptFlagCpaMessHigh — CPA Mess cao: cpaMess_7d > th_cpaMessKill VÀ mess > 0.
// Input: layer1 (cpaMess_7d), raw.meta (mess), params (th_cpaMessKill).
var scriptFlagCpaMessHigh = `function evaluate(ctx) {
  var l1 = ctx.layers.layer1 || {}; var r = ctx.layers.raw || {}; var m = r.meta || {}; var p = ctx.params || {};
  var toF = function(x,k){var v=x[k];if(v==null)return 0;if(typeof v==='number')return v;return parseFloat(v)||0;};
  var toI = function(x,k){var v=x[k];if(v==null)return 0;return parseInt(v,10)||0;};
  var report = { log: '' };
  var cpa = toF(l1,'cpaMess_7d'); var mess = toI(m,'mess'); var th = p.th_cpaMessKill || 180000;
  report.log = '1. cpaMess_7d=' + cpa + ', mess=' + mess + ', th=' + th;
  if (cpa <= th) { report.result = 'no_match'; report.log += '\n2. cpaMess <= th → no_match'; return { output: null, report: report }; }
  if (mess <= 0) { report.result = 'no_match'; report.log += '\n2. mess <= 0 → no_match'; return { output: null, report: report }; }
  report.result = 'match'; report.log += '\n2. cpa_mess_high match';
  return { output: { flag: 'cpa_mess_high', value: true }, report: report };
}`

// scriptFlagCpaPurchaseHigh — CPA Purchase cao: cpaPurchase_7d > th VÀ orders > 0.
// Input: layer1 (cpaPurchase_7d), pancake.pos (orders), params (th_cpaPurchaseHardStop).
var scriptFlagCpaPurchaseHigh = `function evaluate(ctx) {
  var l1 = ctx.layers.layer1 || {}; var r = ctx.layers.raw || {}; var pos = (r.pancake && r.pancake.pos) || {}; var p = ctx.params || {};
  var toF = function(x,k){var v=x[k];if(v==null)return 0;if(typeof v==='number')return v;return parseFloat(v)||0;};
  var toI = function(x,k){var v=x[k];if(v==null)return 0;return parseInt(v,10)||0;};
  var report = { log: '' };
  var cpa = toF(l1,'cpaPurchase_7d'); var orders = toI(pos,'orders'); var th = p.th_cpaPurchaseHardStop || 1050000;
  report.log = '1. cpaPurchase_7d=' + cpa + ', orders=' + orders + ', th=' + th;
  if (cpa <= th) { report.result = 'no_match'; report.log += '\n2. cpaPurchase <= th → no_match'; return { output: null, report: report }; }
  if (orders <= 0) { report.result = 'no_match'; report.log += '\n2. orders <= 0 → no_match'; return { output: null, report: report }; }
  report.result = 'match'; report.log += '\n2. cpa_purchase_high match';
  return { output: { flag: 'cpa_purchase_high', value: true }, report: report };
}`

// scriptFlagConvRateLow — Conv rate thấp + mess trap: convRate_7d < th VÀ mess >= thMess.
// Input: layer1 (convRate_7d), raw.meta (mess), params (th_convRateMessTrap, th_messTrapSlDMin).
var scriptFlagConvRateLow = `function evaluate(ctx) {
  var l1 = ctx.layers.layer1 || {}; var r = ctx.layers.raw || {}; var m = r.meta || {}; var p = ctx.params || {};
  var toF = function(x,k){var v=x[k];if(v==null)return 0;if(typeof v==='number')return v;return parseFloat(v)||0;};
  var toI = function(x,k){var v=x[k];if(v==null)return 0;return parseInt(v,10)||0;};
  var report = { log: '' };
  var cr = toF(l1,'convRate_7d'); var mess = toI(m,'mess'); var th = p.th_convRateMessTrap || 0.05; var thMess = p.th_messTrapSlDMin || 15;
  report.log = '1. convRate_7d=' + (cr*100).toFixed(2) + '%, mess=' + mess + ', th=' + th + ', thMess=' + thMess;
  if (cr >= th) { report.result = 'no_match'; report.log += '\n2. convRate >= th → no_match'; return { output: null, report: report }; }
  if (mess < thMess) { report.result = 'no_match'; report.log += '\n2. mess < thMess → no_match'; return { output: null, report: report }; }
  report.result = 'match'; report.log += '\n2. conv_rate_low match';
  return { output: { flag: 'conv_rate_low', value: true }, report: report };
}`

// scriptFlagCtrCritical — CTR thảm họa: ctr < th_ctrKill.
// Input: raw.meta (ctr), params (th_ctrKill).
var scriptFlagCtrCritical = `function evaluate(ctx) {
  var r = ctx.layers.raw || {}; var m = r.meta || {}; var p = ctx.params || {};
  var toF = function(x,k){var v=x[k];if(v==null)return 0;if(typeof v==='number')return v;return parseFloat(v)||0;};
  var report = { log: '' };
  var ctr = toF(m,'ctr'); var th = p.th_ctrKill || 0.0035;
  report.log = '1. ctr=' + (ctr*100).toFixed(2) + '%, th=' + (th*100).toFixed(2) + '%';
  if (ctr >= th) { report.result = 'no_match'; report.log += '\n2. ctr >= th → no_match'; return { output: null, report: report }; }
  report.result = 'match'; report.log += '\n2. ctr_critical match';
  return { output: { flag: 'ctr_critical', value: true }, report: report };
}`

// scriptFlagMsgRateLow — Msg rate thấp: msgRate_7d < th_msgRateLow.
// Input: layer1 (msgRate_7d), params (th_msgRateLow).
var scriptFlagMsgRateLow = `function evaluate(ctx) {
  var l1 = ctx.layers.layer1 || {}; var p = ctx.params || {};
  var toF = function(x,k){var v=x[k];if(v==null)return 0;if(typeof v==='number')return v;return parseFloat(v)||0;};
  var report = { log: '' };
  var mr = toF(l1,'msgRate_7d'); var th = p.th_msgRateLow || 0.02;
  report.log = '1. msgRate_7d=' + mr.toFixed(4) + ', th=' + th;
  if (mr >= th) { report.result = 'no_match'; report.log += '\n2. msgRate >= th → no_match'; return { output: null, report: report }; }
  report.result = 'match'; report.log += '\n2. msg_rate_low match';
  return { output: { flag: 'msg_rate_low', value: true }, report: report };
}`

// scriptFlagCpmLow — CPM thấp: cpm < th_cpmMessTrapLow.
// Input: raw.meta (cpm), params (th_cpmMessTrapLow).
var scriptFlagCpmLow = `function evaluate(ctx) {
  var r = ctx.layers.raw || {}; var m = r.meta || {}; var p = ctx.params || {};
  var toF = function(x,k){var v=x[k];if(v==null)return 0;if(typeof v==='number')return v;return parseFloat(v)||0;};
  var report = { log: '' };
  var cpm = toF(m,'cpm'); var th = p.th_cpmMessTrapLow || 60000;
  report.log = '1. cpm=' + cpm + ', th=' + th;
  if (cpm >= th) { report.result = 'no_match'; report.log += '\n2. cpm >= th → no_match'; return { output: null, report: report }; }
  report.result = 'match'; report.log += '\n2. cpm_low match';
  return { output: { flag: 'cpm_low', value: true }, report: report };
}`

// scriptFlagCpmHigh — CPM cao: cpm > th_cpmHigh.
// Input: raw.meta (cpm), params (th_cpmHigh).
var scriptFlagCpmHigh = `function evaluate(ctx) {
  var r = ctx.layers.raw || {}; var m = r.meta || {}; var p = ctx.params || {};
  var toF = function(x,k){var v=x[k];if(v==null)return 0;if(typeof v==='number')return v;return parseFloat(v)||0;};
  var report = { log: '' };
  var cpm = toF(m,'cpm'); var th = p.th_cpmHigh || 180000;
  report.log = '1. cpm=' + cpm + ', th=' + th;
  if (cpm <= th) { report.result = 'no_match'; report.log += '\n2. cpm <= th → no_match'; return { output: null, report: report }; }
  report.result = 'match'; report.log += '\n2. cpm_high match';
  return { output: { flag: 'cpm_high', value: true }, report: report };
}`

// scriptFlagFrequencyHigh — Frequency cao: frequency > th_frequencyHigh.
// Input: raw.meta (frequency), params (th_frequencyHigh).
var scriptFlagFrequencyHigh = `function evaluate(ctx) {
  var r = ctx.layers.raw || {}; var m = r.meta || {}; var p = ctx.params || {};
  var toF = function(x,k){var v=x[k];if(v==null)return 0;if(typeof v==='number')return v;return parseFloat(v)||0;};
  var report = { log: '' };
  var freq = toF(m,'frequency'); var th = p.th_frequencyHigh || 3;
  report.log = '1. frequency=' + freq + ', th=' + th;
  if (freq <= th) { report.result = 'no_match'; report.log += '\n2. freq <= th → no_match'; return { output: null, report: report }; }
  report.result = 'match'; report.log += '\n2. frequency_high match';
  return { output: { flag: 'frequency_high', value: true }, report: report };
}`

// scriptFlagChsCritical — CHS Critical: healthState=critical HOẶC chs < th_chsWarningThreshold.
// Input: layer3 (healthState, chs), params (th_chsWarningThreshold).
var scriptFlagChsCritical = `function evaluate(ctx) {
  var l3 = ctx.layers.layer3 || {}; var p = ctx.params || {};
  var toF = function(x,k){var v=x[k];if(v==null)return 0;if(typeof v==='number')return v;return parseFloat(v)||0;};
  var getS = function(x,k){var v=x[k];return(v!=null&&typeof v==='string')?v:'';};
  var report = { log: '' };
  var hs = getS(l3,'healthState'); var chs = toF(l3,'chs'); var th = p.th_chsWarningThreshold || 40;
  report.log = '1. healthState=' + hs + ', chs=' + chs.toFixed(1) + ', th=' + th;
  if (hs === 'critical') { report.result = 'match'; report.log += '\n2. healthState=critical → chs_critical'; return { output: { flag: 'chs_critical', value: true }, report: report }; }
  if (chs < th) { report.result = 'match'; report.log += '\n2. chs < th → chs_critical'; return { output: { flag: 'chs_critical', value: true }, report: report }; }
  report.result = 'no_match'; report.log += '\n2. chs >= th → no_match'; return { output: null, report: report };
}`

// scriptFlagChsWarning — CHS Warning: healthState=warning HOẶC (chs >= th VÀ chs < 60).
// Input: layer3 (healthState, chs), params (th_chsWarningThreshold).
var scriptFlagChsWarning = `function evaluate(ctx) {
  var l3 = ctx.layers.layer3 || {}; var p = ctx.params || {};
  var toF = function(x,k){var v=x[k];if(v==null)return 0;if(typeof v==='number')return v;return parseFloat(v)||0;};
  var getS = function(x,k){var v=x[k];return(v!=null&&typeof v==='string')?v:'';};
  var report = { log: '' };
  var hs = getS(l3,'healthState'); var chs = toF(l3,'chs'); var th = p.th_chsWarningThreshold || 40;
  report.log = '1. healthState=' + hs + ', chs=' + chs.toFixed(1) + ', th=' + th;
  if (hs === 'warning') { report.result = 'match'; report.log += '\n2. healthState=warning → chs_warning'; return { output: { flag: 'chs_warning', value: true }, report: report }; }
  if (chs >= th && chs < 60) { report.result = 'match'; report.log += '\n2. chs trong khoảng [' + th + ',60) → chs_warning'; return { output: { flag: 'chs_warning', value: true }, report: report }; }
  report.result = 'no_match'; report.log += '\n2. chs_warning no_match'; return { output: null, report: report };
}`

// scriptFlagSlA — SL-A: spendPct>20%, runtime>90, cpaMess>th, mess<3, mqs<1.
// Input: layer1, raw.meta, params (th_spendPctBase, th_runtimeMinutesBase, th_cpaMessKill, th_mqsSlAMax).
var scriptFlagSlA = `function evaluate(ctx) {
  var l1=ctx.layers.layer1||{}; var r=ctx.layers.raw||{}; var m=r.meta||{}; var p=ctx.params||{};
  var toF=function(x,k){var v=x[k];if(v==null)return 0;if(typeof v==='number')return v;return parseFloat(v)||0;};
  var toI=function(x,k){var v=x[k];if(v==null)return 0;return parseInt(v,10)||0;};
  var report = { log: '' };
  var spend=toF(l1,'spendPct_7d'); var rt=toF(l1,'runtimeMinutes'); var cpa=toF(l1,'cpaMess_7d'); var mess=toI(m,'mess'); var mqs=toF(l1,'mqs_7d');
  var thSpend=p.th_spendPctBase||0.2; var thRt=p.th_runtimeMinutesBase||90; var thCpa=p.th_cpaMessKill||180000; var thMqs=p.th_mqsSlAMax||1;
  report.log = '1. spendPct=' + (spend*100).toFixed(1) + '%, runtime=' + rt + 'p, cpaMess=' + cpa + ', mess=' + mess + ', mqs=' + mqs.toFixed(1);
  if (spend<=thSpend && !(spend===0 && toF(m,'spend')>0)) { report.result='no_match'; report.log += '\n2. spendPct<=' + thSpend + ' → no_match'; return { output: null, report: report }; }
  if (rt<=thRt) { report.result='no_match'; report.log += '\n2. runtime<=' + thRt + ' → no_match'; return { output: null, report: report }; }
  if (cpa<=thCpa || mess>=3 || mqs>=thMqs) { report.result='no_match'; report.log += '\n2. cpa/mess/mqs không đạt (cpa cần >' + thCpa + ', mess<3, mqs<' + thMqs + ')'; return { output: null, report: report }; }
  report.result='match'; report.log += '\n2. sl_a match'; return { output: { flag: 'sl_a', value: true }, report: report };
}`

// scriptFlagSlADecrease — SL-A Decrease: như sl_a nhưng mqs >= 2 (đề xuất giảm thay vì kill).
// Input: layer1, raw.meta, params.
var scriptFlagSlADecrease = `function evaluate(ctx) {
  var l1=ctx.layers.layer1||{}; var r=ctx.layers.raw||{}; var m=r.meta||{}; var p=ctx.params||{};
  var toF=function(x,k){var v=x[k];if(v==null)return 0;if(typeof v==='number')return v;return parseFloat(v)||0;};
  var toI=function(x,k){var v=x[k];if(v==null)return 0;return parseInt(v,10)||0;};
  var report = { log: '' };
  var spend=toF(l1,'spendPct_7d'); var rt=toF(l1,'runtimeMinutes'); var cpa=toF(l1,'cpaMess_7d'); var mess=toI(m,'mess'); var mqs=toF(l1,'mqs_7d');
  var thSpend=p.th_spendPctBase||0.2; var thRt=p.th_runtimeMinutesBase||90; var thCpa=p.th_cpaMessKill||180000; var thMqs=p.th_mqsSlADecreaseMin||2;
  report.log = '1. spendPct=' + (spend*100).toFixed(1) + '%, runtime=' + rt + 'p, cpaMess=' + cpa + ', mess=' + mess + ', mqs=' + mqs.toFixed(1) + ' (cần>=' + thMqs + ')';
  if (spend<=thSpend && !(spend===0 && toF(m,'spend')>0)) { report.result='no_match'; report.log += '\n2. spendPct → no_match'; return { output: null, report: report }; }
  if (rt<=thRt) { report.result='no_match'; report.log += '\n2. runtime → no_match'; return { output: null, report: report }; }
  if (cpa<=thCpa || mess>=3 || mqs<thMqs) { report.result='no_match'; report.log += '\n2. cpa/mess/mqs (mqs cần>=' + thMqs + ') → no_match'; return { output: null, report: report }; }
  report.result='match'; report.log += '\n2. sl_a_decrease match'; return { output: { flag: 'sl_a_decrease', value: true }, report: report };
}`

// scriptFlagSlC — SL-C: spendPct>20%, runtime>90, ctr<th, spendPct>15%, cpm>th, msgRate<th.
// Input: layer1, raw.meta, params.
var scriptFlagSlC = `function evaluate(ctx) {
  var l1=ctx.layers.layer1||{}; var r=ctx.layers.raw||{}; var m=r.meta||{}; var p=ctx.params||{};
  var toF=function(x,k){var v=x[k];if(v==null)return 0;if(typeof v==='number')return v;return parseFloat(v)||0;};
  var report = { log: '' };
  var spend=toF(l1,'spendPct_7d'); var rt=toF(l1,'runtimeMinutes'); var ctr=toF(m,'ctr'); var cpm=toF(m,'cpm'); var mr=toF(l1,'msgRate_7d');
  var thSpend=p.th_spendPctBase||0.2; var thRt=p.th_runtimeMinutesBase||90; var thCtr=p.th_ctrKill||0.0035; var thSlC=p.th_spendPctSlC||0.15; var thCpm=p.th_cpmHigh||180000; var thMr=p.th_msgRateLow||0.02;
  report.log = '1. spendPct=' + (spend*100).toFixed(1) + '%, runtime=' + rt + ', ctr=' + (ctr*100).toFixed(2) + '%, cpm=' + cpm + ', msgRate=' + mr.toFixed(4);
  if (spend<=thSpend && !(spend===0 && toF(m,'spend')>0)) { report.result='no_match'; report.log += '\n2. spendPct → no_match'; return { output: null, report: report }; }
  if (rt<=thRt) { report.result='no_match'; report.log += '\n2. runtime → no_match'; return { output: null, report: report }; }
  if (ctr>=thCtr) { report.result='no_match'; report.log += '\n2. ctr>=' + thCtr + ' → no_match'; return { output: null, report: report }; }
  if (spend<=thSlC && !(spend===0 && toF(m,'spend')>0)) { report.result='no_match'; report.log += '\n2. spendPctSlC → no_match'; return { output: null, report: report }; }
  if (cpm<=thCpm) { report.result='no_match'; report.log += '\n2. cpm → no_match'; return { output: null, report: report }; }
  if (mr>=thMr) { report.result='no_match'; report.log += '\n2. msgRate → no_match'; return { output: null, report: report }; }
  report.result='match'; report.log += '\n2. sl_c match'; return { output: { flag: 'sl_c', value: true }, report: report };
}`

// scriptFlagSlD — SL-D: spendPct>20%, runtime>90, mess>=15, convRate<5%.
// Input: layer1, raw.meta, params.
var scriptFlagSlD = `function evaluate(ctx) {
  var l1=ctx.layers.layer1||{}; var r=ctx.layers.raw||{}; var m=r.meta||{}; var pos=(r.pancake&&r.pancake.pos)||{}; var p=ctx.params||{};
  var toF=function(x,k){var v=x[k];if(v==null)return 0;if(typeof v==='number')return v;return parseFloat(v)||0;};
  var toI=function(x,k){var v=x[k];if(v==null)return 0;return parseInt(v,10)||0;};
  var report = { log: '' };
  var spend=toF(l1,'spendPct_7d'); var rt=toF(l1,'runtimeMinutes'); var mess=toI(m,'mess'); var cr=toF(l1,'convRate_7d');
  var thSpend=p.th_spendPctBase||0.2; var thRt=p.th_runtimeMinutesBase||90; var thMess=p.th_messTrapSlDMin||15; var thCr=p.th_convRateMessTrap||0.05; var thSlD=p.th_spendPctSlD||0.2;
  report.log = '1. spendPct=' + (spend*100).toFixed(1) + '%, runtime=' + rt + ', mess=' + mess + ', convRate=' + (cr*100).toFixed(2) + '%';
  if (spend<=thSpend && !(spend===0 && toF(m,'spend')>0)) { report.result='no_match'; report.log += '\n2. spendPct → no_match'; return { output: null, report: report }; }
  if (rt<=thRt) { report.result='no_match'; report.log += '\n2. runtime → no_match'; return { output: null, report: report }; }
  if (mess<thMess) { report.result='no_match'; report.log += '\n2. mess<' + thMess + ' → no_match'; return { output: null, report: report }; }
  if (cr>=thCr) { report.result='no_match'; report.log += '\n2. convRate>=' + thCr + ' → no_match'; return { output: null, report: report }; }
  if (spend<=thSlD && !(spend===0 && toF(m,'spend')>0)) { report.result='no_match'; report.log += '\n2. spendPctSlD → no_match'; return { output: null, report: report }; }
  report.result='match'; report.log += '\n2. sl_d match'; return { output: { flag: 'sl_d', value: true }, report: report };
}`

// scriptFlagSlE — SL-E: spendPct>20%, runtime>90, cpaPurchase>th, orders>=3, convRate<10%, mqs<1.
// Input: layer1, pancake.pos, params.
var scriptFlagSlE = `function evaluate(ctx) {
  var l1=ctx.layers.layer1||{}; var r=ctx.layers.raw||{}; var pos=(r.pancake&&r.pancake.pos)||{}; var p=ctx.params||{};
  var toF=function(x,k){var v=x[k];if(v==null)return 0;if(typeof v==='number')return v;return parseFloat(v)||0;};
  var toI=function(x,k){var v=x[k];if(v==null)return 0;return parseInt(v,10)||0;};
  var report = { log: '' };
  var spend=toF(l1,'spendPct_7d'); var rt=toF(l1,'runtimeMinutes'); var cpa=toF(l1,'cpaPurchase_7d'); var orders=toI(pos,'orders'); var cr=toF(l1,'convRate_7d'); var mqs=toF(l1,'mqs_7d');
  var thSpend=p.th_spendPctBase||0.2; var thRt=p.th_runtimeMinutesBase||90; var thCpa=p.th_cpaPurchaseHardStop||1050000; var thOrd=p.th_slEOrdersMin||3; var thCr=p.th_slECrMax||0.1; var thMqs=p.th_mqsSlEMax||1;
  report.log = '1. spendPct=' + (spend*100).toFixed(1) + '%, runtime=' + rt + ', cpaPurchase=' + cpa + ', orders=' + orders + ', convRate=' + (cr*100).toFixed(1) + '%, mqs=' + mqs.toFixed(1);
  if (spend<=thSpend && !(spend===0 && toF(r.meta||{},'spend')>0)) { report.result='no_match'; report.log += '\n2. spendPct → no_match'; return { output: null, report: report }; }
  if (rt<=thRt) { report.result='no_match'; report.log += '\n2. runtime → no_match'; return { output: null, report: report }; }
  if (cpa<=thCpa || orders<thOrd || cr>=thCr || mqs>=thMqs) { report.result='no_match'; report.log += '\n2. cpa/orders/cr/mqs không đạt → no_match'; return { output: null, report: report }; }
  report.result='match'; report.log += '\n2. sl_e match'; return { output: { flag: 'sl_e', value: true }, report: report };
}`

// scriptFlagKoA — KO-A: deliveryStatus IN (LIMITED,NOT_DELIVERING), runtime>120, spendPct trong (0, 8%).
// Input: layer1, raw.meta (deliveryStatus), params.
var scriptFlagKoA = `function evaluate(ctx) {
  var l1=ctx.layers.layer1||{}; var r=ctx.layers.raw||{}; var m=r.meta||{}; var p=ctx.params||{};
  var toF=function(x,k){var v=x[k];if(v==null)return 0;if(typeof v==='number')return v;return parseFloat(v)||0;};
  var getS=function(x,k){var v=x[k];return(v!=null&&typeof v==='string')?v:'';};
  var report = { log: '' };
  var ds=getS(m,'deliveryStatus'); var rt=toF(l1,'runtimeMinutes'); var spend=toF(l1,'spendPct_7d'); var thRt=p.th_runtimeMinutesKoA||120; var thMax=p.th_spendPctKoAMax||0.08;
  report.log = '1. deliveryStatus=' + ds + ', runtime=' + rt + ', spendPct=' + (spend*100).toFixed(1) + '%';
  if (ds!=='LIMITED' && ds!=='NOT_DELIVERING') { report.result='no_match'; report.log += '\n2. deliveryStatus không LIMITED/NOT_DELIVERING → no_match'; return { output: null, report: report }; }
  if (rt<=thRt) { report.result='no_match'; report.log += '\n2. runtime<=' + thRt + ' → no_match'; return { output: null, report: report }; }
  if (spend<=0 || spend>=thMax) { report.result='no_match'; report.log += '\n2. spendPct không trong (0,' + (thMax*100) + '%) → no_match'; return { output: null, report: report }; }
  report.result='match'; report.log += '\n2. ko_a match'; return { output: { flag: 'ko_a', value: true }, report: report };
}`

// scriptFlagKoB — KO-B: ctr>1.8%, msgRate<2%, orders=0, spendPct>15%, mqs<0.5.
// Input: layer1, raw.meta, pancake.pos, params.
var scriptFlagKoB = `function evaluate(ctx) {
  var l1=ctx.layers.layer1||{}; var r=ctx.layers.raw||{}; var m=r.meta||{}; var pos=(r.pancake&&r.pancake.pos)||{}; var p=ctx.params||{};
  var toF=function(x,k){var v=x[k];if(v==null)return 0;if(typeof v==='number')return v;return parseFloat(v)||0;};
  var toI=function(x,k){var v=x[k];if(v==null)return 0;return parseInt(v,10)||0;};
  var report = { log: '' };
  var ctr=toF(m,'ctr'); var mr=toF(l1,'msgRate_7d'); var orders=toI(pos,'orders'); var spend=toF(l1,'spendPct_7d'); var mqs=toF(l1,'mqs_7d');
  var thCtr=p.th_ctrTrafficRac||0.018; var thMr=p.th_msgRateLow||0.02; var thSpend=p.th_spendPctKoB||0.15; var thMqs=p.th_mqsKoBMax||0.5;
  report.log = '1. ctr=' + (ctr*100).toFixed(2) + '%, msgRate=' + mr.toFixed(4) + ', orders=' + orders + ', spendPct=' + (spend*100).toFixed(1) + '%, mqs=' + mqs.toFixed(1);
  if (ctr<=thCtr) { report.result='no_match'; report.log += '\n2. ctr<=' + (thCtr*100) + '% → no_match'; return { output: null, report: report }; }
  if (mr>=thMr) { report.result='no_match'; report.log += '\n2. msgRate>=' + thMr + ' → no_match'; return { output: null, report: report }; }
  if (orders!==0) { report.result='no_match'; report.log += '\n2. orders!=0 → no_match'; return { output: null, report: report }; }
  if (spend<=thSpend && !(spend===0 && toF(m,'spend')>0)) { report.result='no_match'; report.log += '\n2. spendPct → no_match'; return { output: null, report: report }; }
  if (mqs>=thMqs) { report.result='no_match'; report.log += '\n2. mqs>=' + thMqs + ' → no_match'; return { output: null, report: report }; }
  report.result='match'; report.log += '\n2. ko_b match'; return { output: { flag: 'ko_b', value: true }, report: report };
}`

// scriptFlagKoC — KO-C: cpm > cpmHigh*multiplier, impressions<800, spendPct>10%.
// Input: layer1, raw.meta, params.
var scriptFlagKoC = `function evaluate(ctx) {
  var l1=ctx.layers.layer1||{}; var r=ctx.layers.raw||{}; var m=r.meta||{}; var p=ctx.params||{};
  var toF=function(x,k){var v=x[k];if(v==null)return 0;if(typeof v==='number')return v;return parseFloat(v)||0;};
  var toI=function(x,k){var v=x[k];if(v==null)return 0;return parseInt(v,10)||0;};
  var report = { log: '' };
  var cpm=toF(m,'cpm'); var imp=toI(m,'impressions'); var spend=toF(l1,'spendPct_7d'); var thCpm=p.th_cpmHigh||180000; var mult=p.th_cpmKoCMultiplier||2.5; var thSpend=p.th_spendPctKoC||0.1;
  report.log = '1. cpm=' + cpm + ', impressions=' + imp + ', spendPct=' + (spend*100).toFixed(1) + '%, thCpm*mult=' + (thCpm*mult);
  if (cpm<=thCpm*mult) { report.result='no_match'; report.log += '\n2. cpm không > thCpm*mult → no_match'; return { output: null, report: report }; }
  if (imp>=800) { report.result='no_match'; report.log += '\n2. impressions>=800 → no_match'; return { output: null, report: report }; }
  if (spend<=thSpend && !(spend===0 && toF(m,'spend')>0)) { report.result='no_match'; report.log += '\n2. spendPct → no_match'; return { output: null, report: report }; }
  report.result='match'; report.log += '\n2. ko_c match'; return { output: { flag: 'ko_c', value: true }, report: report };
}`

// scriptFlagMessTrapSuspect — Mess Trap nghi ngờ: cpaMess<60k, convRate<6%, mess>=20, orders=0, spendPct>15%.
// Input: layer1, raw.meta, pancake.pos, params.
var scriptFlagMessTrapSuspect = `function evaluate(ctx) {
  var l1=ctx.layers.layer1||{}; var r=ctx.layers.raw||{}; var m=r.meta||{}; var pos=(r.pancake&&r.pancake.pos)||{}; var p=ctx.params||{};
  var toF=function(x,k){var v=x[k];if(v==null)return 0;if(typeof v==='number')return v;return parseFloat(v)||0;};
  var toI=function(x,k){var v=x[k];if(v==null)return 0;return parseInt(v,10)||0;};
  var report = { log: '' };
  var cpa=toF(l1,'cpaMess_7d'); var cr=toF(l1,'convRate_7d'); var mess=toI(m,'mess'); var orders=toI(pos,'orders'); var spend=toF(l1,'spendPct_7d');
  var thCpa=p.th_cpaMessTrapLow||60000; var thCr=p.th_convRateMessTrap6||0.06; var thMess=p.th_messTrapSuspectMin||20; var thSpend=p.th_spendPctMessTrap||0.15;
  report.log = '1. cpaMess=' + cpa + ', convRate=' + (cr*100).toFixed(2) + '%, mess=' + mess + ', orders=' + orders + ', spendPct=' + (spend*100).toFixed(1) + '%';
  if (cpa>=thCpa) { report.result='no_match'; report.log += '\n2. cpaMess>=' + thCpa + ' → no_match'; return { output: null, report: report }; }
  if (cr>=thCr) { report.result='no_match'; report.log += '\n2. convRate>=' + (thCr*100) + '% → no_match'; return { output: null, report: report }; }
  if (mess<thMess) { report.result='no_match'; report.log += '\n2. mess<' + thMess + ' → no_match'; return { output: null, report: report }; }
  if (orders!==0) { report.result='no_match'; report.log += '\n2. orders!=0 → no_match'; return { output: null, report: report }; }
  if (spend<=thSpend && !(spend===0 && toF(m,'spend')>0)) { report.result='no_match'; report.log += '\n2. spendPct → no_match'; return { output: null, report: report }; }
  report.result='match'; report.log += '\n2. mess_trap_suspect match'; return { output: { flag: 'mess_trap_suspect', value: true }, report: report };
}`

// scriptFlagTrimEligible — Trim Kill: inTrimWindow=1, freq>2.2, chs<60, orders<3.
// Input: layer3, raw.meta, pancake.pos, params (inTrimWindow, th_frequencyTrim, th_trimOrdersMin).
var scriptFlagTrimEligible = `function evaluate(ctx) {
  var l1=ctx.layers.layer1||{}; var l3=ctx.layers.layer3||{}; var r=ctx.layers.raw||{}; var m=r.meta||{}; var pos=(r.pancake&&r.pancake.pos)||{}; var p=ctx.params||{};
  var toF=function(x,k){var v=x[k];if(v==null)return 0;if(typeof v==='number')return v;return parseFloat(v)||0;};
  var toI=function(x,k){var v=x[k];if(v==null)return 0;return parseInt(v,10)||0;};
  var report = { log: '' };
  var inTrim=p.inTrimWindow===true||p.inTrimWindow===1; var freq=toF(m,'frequency'); var chs=toF(l3,'chs'); var orders=toI(pos,'orders'); var thFreq=p.th_frequencyTrim||2.2; var thOrd=p.th_trimOrdersMin||3;
  report.log = '1. inTrimWindow=' + inTrim + ', freq=' + freq + ', chs=' + chs.toFixed(1) + ', orders=' + orders;
  if (!inTrim) { report.result='no_match'; report.log += '\n2. inTrimWindow=false → no_match'; return { output: null, report: report }; }
  if (freq<=thFreq) { report.result='no_match'; report.log += '\n2. freq<=' + thFreq + ' → no_match'; return { output: null, report: report }; }
  if (chs>=60) { report.result='no_match'; report.log += '\n2. chs>=60 → no_match'; return { output: null, report: report }; }
  if (orders>=thOrd) { report.result='no_match'; report.log += '\n2. orders>=' + thOrd + ' → no_match'; return { output: null, report: report }; }
  report.result='match'; report.log += '\n2. trim_eligible match'; return { output: { flag: 'trim_eligible', value: true }, report: report };
}`

// scriptFlagTrimEligibleDecrease — Trim Decrease: inTrimWindow=1, freq>2.2, chs<60, orders>=3.
// Input: layer3, raw.meta, pancake.pos, params.
var scriptFlagTrimEligibleDecrease = `function evaluate(ctx) {
  var l1=ctx.layers.layer1||{}; var l3=ctx.layers.layer3||{}; var r=ctx.layers.raw||{}; var m=r.meta||{}; var pos=(r.pancake&&r.pancake.pos)||{}; var p=ctx.params||{};
  var toF=function(x,k){var v=x[k];if(v==null)return 0;if(typeof v==='number')return v;return parseFloat(v)||0;};
  var toI=function(x,k){var v=x[k];if(v==null)return 0;return parseInt(v,10)||0;};
  var report = { log: '' };
  var inTrim=p.inTrimWindow===true||p.inTrimWindow===1; var freq=toF(m,'frequency'); var chs=toF(l3,'chs'); var orders=toI(pos,'orders'); var thFreq=p.th_frequencyTrim||2.2; var thOrd=p.th_trimOrdersMin||3;
  report.log = '1. inTrimWindow=' + inTrim + ', freq=' + freq + ', chs=' + chs.toFixed(1) + ', orders=' + orders + ' (cần>=' + thOrd + ')';
  if (!inTrim) { report.result='no_match'; report.log += '\n2. inTrimWindow=false → no_match'; return { output: null, report: report }; }
  if (freq<=thFreq) { report.result='no_match'; report.log += '\n2. freq<=' + thFreq + ' → no_match'; return { output: null, report: report }; }
  if (chs>=60) { report.result='no_match'; report.log += '\n2. chs>=60 → no_match'; return { output: null, report: report }; }
  if (orders<thOrd) { report.result='no_match'; report.log += '\n2. orders<' + thOrd + ' → no_match'; return { output: null, report: report }; }
  report.result='match'; report.log += '\n2. trim_eligible_decrease match'; return { output: { flag: 'trim_eligible_decrease', value: true }, report: report };
}`

// scriptFlagConvRateStrong — Conv rate mạnh: convRate_7d >= th_convRateStrong (mặc định 20%).
// Input: layer1 (convRate_7d), params (th_convRateStrong).
var scriptFlagConvRateStrong = `function evaluate(ctx) {
  var l1 = ctx.layers.layer1 || {}; var p = ctx.params || {};
  var toF = function(x,k){var v=x[k];if(v==null)return 0;if(typeof v==='number')return v;return parseFloat(v)||0;};
  var report = { log: '' };
  var cr = toF(l1,'convRate_7d'); var th = p.th_convRateStrong || 0.2;
  report.log = '1. convRate_7d=' + (cr*100).toFixed(2) + '%, th=' + (th*100) + '%';
  if (cr < th) { report.result = 'no_match'; report.log += '\n2. convRate < th → no_match'; return { output: null, report: report }; }
  report.result = 'match'; report.log += '\n2. conv_rate_strong match';
  return { output: { flag: 'conv_rate_strong', value: true }, report: report };
}`

// scriptFlagPortfolioAttention — Portfolio cần chú ý: portfolioCell = fix HOẶC recover.
// Input: layer3 (portfolioCell).
var scriptFlagPortfolioAttention = `function evaluate(ctx) {
  var l3 = ctx.layers.layer3 || {};
  var getS = function(x,k){var v=x[k];return(v!=null&&typeof v==='string')?v:'';};
  var report = { log: '' };
  var cell = getS(l3,'portfolioCell');
  report.log = '1. portfolioCell=' + cell;
  if (cell === 'fix' || cell === 'recover') { report.result = 'match'; report.log += '\n2. portfolio_attention match'; return { output: { flag: 'portfolio_attention', value: true }, report: report }; }
  report.result = 'no_match'; report.log += '\n2. portfolioCell khác fix/recover → no_match'; return { output: null, report: report };
}`

// scriptFlagWindowShopping — PATCH 04: Pattern Window Shopping (mess tăng sáng, CR sáng thấp, CR chiều hôm qua cao).
// Input: params (in_event_window, mess_07_12_today, mess_07_12_yesterday, orders_07_12_today, mess_12_22_yesterday, orders_12_22_yesterday, th_*).
var scriptFlagWindowShopping = `function evaluate(ctx) {
  var params = ctx.params || {};
  var report = { log: '' };
  report.log = '1. in_event_window=' + !!params.in_event_window;
  if (!params.in_event_window) {
    report.result = 'no_match'; report.log += '\n2. Không trong Event Window → no_match'; return { output: null, report: report };
  }
  var messToday = params.mess_07_12_today || 0;
  var messYesterday = params.mess_07_12_yesterday || 0;
  var ordersToday = params.orders_07_12_today || 0;
  var messY1222 = params.mess_12_22_yesterday || 0;
  var ordersY1222 = params.orders_12_22_yesterday || 0;
  report.log += ', mess07-12 today=' + messToday + ', yesterday=' + messYesterday + ', orders07-12=' + ordersToday + ', mess12-22 yesterday=' + messY1222 + ', orders=' + ordersY1222;
  if (messToday <= 0 || messYesterday <= 0 || messY1222 <= 0) {
    report.result = 'no_match'; report.log += '\n2. Thiếu dữ liệu mess/orders → no_match'; return { output: null, report: report };
  }
  var thMessIncrease = params.th_mess_increase_pct || 1.5;
  if (messToday <= messYesterday * thMessIncrease) {
    report.result = 'no_match'; report.log += '\n2. Mess 07-12h tăng <= ' + ((thMessIncrease-1)*100) + '% vs yesterday → no_match'; return { output: null, report: report };
  }
  var cr0712 = (ordersToday / messToday) * 100;
  var thCr0712Max = params.th_cr_07_12_max || 5;
  if (cr0712 >= thCr0712Max) {
    report.result = 'no_match'; report.log += '\n2. CR 07-12h>=' + thCr0712Max + '% → no_match'; return { output: null, report: report };
  }
  var crY1222 = (ordersY1222 / messY1222) * 100;
  var thCrY1222Min = params.th_cr_yesterday_1222_min || 10;
  if (crY1222 <= thCrY1222Min) {
    report.result = 'no_match'; report.log += '\n2. CR yesterday 12-22h<=' + thCrY1222Min + '% → no_match'; return { output: null, report: report };
  }
  report.result = 'match';
  report.log = 'PATCH 04: Window Shopping Pattern — Mess tăng, CR sáng thấp, CR chiều hôm qua cao';
  var output = { flag: 'window_shopping_pattern', value: true };
  return { output: output, report: report };
}`

// scriptNightOff — Logic time-based: params (account_mode, hour, minute, hour_*) → PAUSE khi đúng giờ tắt.
// Input: params (account_mode, hour, minute, hour_protect, hour_efficiency, hour_normal, minute_normal, hour_blitz).
// Output: action PAUSE khi mode+giờ khớp.
var scriptNightOff = `function evaluate(ctx) {
  var params = ctx.params || {};
  var report = { log: '' };
  var mode = params.account_mode || 'NORMAL';
  var h = params.hour || 0;
  var m = params.minute || 0;
  var hp = params.hour_protect || 21, he = params.hour_efficiency || 22;
  var hn = params.hour_normal || 22, mn = params.minute_normal || 30, hb = params.hour_blitz || 23;
  report.log = '1. mode=' + mode + ', giờ=' + h + ':' + m + ' (PROTECT:' + hp + ':00, EFF:' + he + ':00, NORMAL:' + hn + ':' + mn + ', BLITZ:' + hb + ':00)';
  var match = false;
  if (mode === 'PROTECT' && h === hp && m === 0) match = true;
  if (mode === 'EFFICIENCY' && h === he && m === 0) match = true;
  if (mode === 'NORMAL' && h === hn && m === mn) match = true;
  if (mode === 'BLITZ' && h === hb && m === 0) match = true;
  if (!match) {
    report.result = 'no_match'; report.log += '\n2. Không đúng giờ Night Off → no_match'; return { output: null, report: report };
  }
  report.result = 'match'; report.log += '\n2. Night Off match — mode ' + mode;
  var action = { action_code: 'PAUSE', ruleCode: 'night_off', reason: 'Night Off — mode ' + mode, value: null };
  return { output: action, report: report };
}`

// scriptLayer2 — Derivation Rule: raw + layer1 → layer2 (efficiency, demandQuality, auctionPressure, saturation, momentum).
// Input: raw (7d, meta), layer1 (roas_7d, msgRate_7d, convRate_7d).
// Output: layer2 (efficiency, demandQuality, auctionPressure, saturation, momentum).
var scriptLayer2 = `function evaluate(ctx) {
  var raw = ctx.layers.raw || {};
  var layer1 = ctx.layers.layer1 || {};
  var report = { log: '', result: 'match' };
  var toF = function(m,k){var v=m[k];if(v==null)return 0;if(typeof v==='number')return v;return parseFloat(v)||0;};
  var r7d = raw['7d'] || raw;
  var meta = r7d.meta || {};
  var cpm = toF(meta,'cpm'); var ctr = toF(meta,'ctr'); var freq = toF(meta,'frequency');
  var roas = toF(layer1,'roas_7d'); var msgRate = toF(layer1,'msgRate_7d'); var convRate = toF(layer1,'convRate_7d');
  report.log = '1. roas=' + roas.toFixed(2) + ', msgRate=' + msgRate.toFixed(4) + ', convRate=' + (convRate*100).toFixed(2) + '%, cpm=' + cpm + ', ctr=' + (ctr*100).toFixed(2) + '%, freq=' + freq;
  var scoreRoas = function(r){if(r>=3)return 100;if(r>=2)return 80;if(r>=1)return 60;if(r>=0.5)return 40;return 20;};
  var scoreRate = function(mr,cr){var s=(mr*50+cr*50)/2;return s>100?100:s;};
  var scoreCpmCtr = function(cp,ct){if(cp<50000&&ct>1)return 80;if(cp<100000&&ct>0.5)return 60;return 40;};
  var scoreFreq = function(f){if(f<=2)return 80;if(f<=4)return 60;return 40;};
  var eff = scoreRoas(roas); var demand = scoreRate(msgRate,convRate); var auction = scoreCpmCtr(cpm,ctr);
  var sat = scoreFreq(freq); var mom = 50;
  report.log += '\n2. efficiency=' + eff + ', demandQuality=' + demand + ', auctionPressure=' + auction + ', saturation=' + sat + ', momentum=' + mom;
  var output = { efficiency: eff, demandQuality: demand, auctionPressure: auction, saturation: sat, momentum: mom };
  return { output: output, report: report };
}`

// scriptLayer1 — Derivation Rule: raw → layer1 (msgRate, cpaMess, convRate, lifecycle, mqs, spendPct, runtimeMinutes, ...).
// Input: raw (7d, 2h, 1h, 30p, meta, pancake), params (nowMs, timeFactorForMQS).
// Output: layer1 (lifecycle, msgRate_7d, cpaMess_7d, convRate_7d, roas_7d, mqs_7d, spendPct_7d, runtimeMinutes, ...).
var scriptLayer1 = `function evaluate(ctx) {
  var raw = ctx.layers.raw || {};
  var params = ctx.params || {};
  var report = { log: '', result: 'match' };
  var toF = function(m,k){var v=m[k];if(v==null)return 0;if(typeof v==='number')return v;return parseFloat(v)||0;};
  var toI = function(m,k){var v=m[k];if(v==null)return 0;var x=parseInt(v,10);return isNaN(x)?0:x;};
  var r7d = raw['7d']||raw; var r2h = raw['2h']||null; var r1h = raw['1h']||null; var r30p = raw['30p']||null;
  var meta = r7d.meta||{}; var pancake = r7d.pancake||{}; var pos = pancake.pos||{};
  var spend = toF(meta,'spend'); var mess = toI(meta,'mess'); var inlineLinkClicks = toI(meta,'inlineLinkClicks');
  var orders = toI(pos,'orders'); var revenue = toF(pos,'revenue');
  var msgRate = (inlineLinkClicks>0&&mess>0) ? (mess/inlineLinkClicks) : 0;
  var cpaMess = (mess>0) ? (spend/mess) : 0;
  var cpaPurchase = (orders>0) ? (spend/orders) : 0;
  var convRate = (mess>0) ? (orders/mess) : 0;
  var convRate2h = 0; if(r2h){var o2=toI(r2h,'orders'),m2=toI(r2h,'mess');if(m2>0)convRate2h=o2/m2;}
  var convRate1h = 0; if(r1h){var o1=toI(r1h,'orders'),m1=toI(r1h,'mess');if(m1>0)convRate1h=o1/m1;}
  var roas = (spend>0) ? (revenue/spend) : 0;
  var metaCreatedAt = toI(r7d,'metaCreatedAt');
  var nowMs = params.nowMs || Date.now();
  var lifecycle = 'NEW';
  if(metaCreatedAt>0){
    var days = (nowMs - metaCreatedAt) / (24*60*60*1000);
    if(days<7) lifecycle='NEW'; else if(days<14) lifecycle='WARMING'; else if(days<30) lifecycle='CALIBRATED'; else lifecycle='MATURE';
  } else {
    if(inlineLinkClicks>=100) lifecycle='WARMING';
    if(inlineLinkClicks>=500) lifecycle='CALIBRATED';
    if(inlineLinkClicks>=2000) lifecycle='MATURE';
  }
  var timeFactor = params.timeFactorForMQS || 1;
  var messForMQS = mess; if(r30p){var m30=toI(r30p,'mess');if(m30>0)messForMQS=m30;}
  var mqs = messForMQS * convRate * timeFactor;
  var dailyBudget = toF(meta,'dailyBudget');
  var spendPct = (dailyBudget>0&&spend>0) ? (spend/dailyBudget) : 0;
  var runtimeMinutes = (metaCreatedAt>0) ? ((nowMs-metaCreatedAt)/60000) : 0;
  var msgRate30p = 0;
  if(r30p&&inlineLinkClicks>0){var m30=toI(r30p,'mess');var clicks30=inlineLinkClicks/336;if(clicks30>=1&&m30>0)msgRate30p=m30/clicks30;}
  var mess30p = r30p ? toI(r30p,'mess') : 0;
  var output = { lifecycle: lifecycle, msgRate_7d: msgRate, msgRate_30p: msgRate30p, mess_30p: mess30p,
    cpaMess_7d: cpaMess, cpaPurchase_7d: cpaPurchase, convRate_7d: convRate, convRate_2h: convRate2h, convRate_1h: convRate1h,
    roas_7d: roas, mqs_7d: mqs, spendPct_7d: spendPct, runtimeMinutes: runtimeMinutes };
  report.log = '1. lifecycle=' + lifecycle + ', convRate=' + convRate.toFixed(4) + ', mqs=' + mqs.toFixed(1) + ', spendPct=' + (spendPct*100).toFixed(1) + '%, runtime=' + runtimeMinutes.toFixed(0) + 'p';
  report.log += '\n2. layer1 computed';
  return { output: output, report: report };
}`

// scriptLayer3 — Derivation Rule: layer1 + layer2 → layer3 (chs, healthState, performanceTier, portfolioCell).
// Input: layer1 (roas_7d, lifecycle), layer2 (efficiency, demandQuality, auctionPressure, saturation, momentum).
// Output: layer3 (chs, healthState, performanceTier, portfolioCell, stage, diagnoses).
var scriptLayer3 = `function evaluate(ctx) {
  var layer1 = ctx.layers.layer1 || {};
  var layer2 = ctx.layers.layer2 || {};
  var report = { log: '', result: 'match' };
  var toF = function(m,k){var v=m[k];if(v==null)return 0;if(typeof v==='number')return v;return parseFloat(v)||0;};
  var getS = function(m,k){var v=m[k];return(v!=null&&typeof v==='string')?v:'';};
  var eff=toF(layer2,'efficiency'); var demand=toF(layer2,'demandQuality'); var auction=toF(layer2,'auctionPressure');
  var sat=toF(layer2,'saturation'); var mom=toF(layer2,'momentum');
  var chs = (eff+demand+auction+sat+mom)/5;
  report.log = '1. scores: eff=' + eff + ', demand=' + demand + ', auction=' + auction + ', sat=' + sat + ', mom=' + mom + ' → chs=' + chs.toFixed(1);
  var healthState = 'critical';
  if(chs>=80) healthState='strong'; else if(chs>=60) healthState='healthy'; else if(chs>=40) healthState='warning';
  var roas = toF(layer1,'roas_7d');
  var performanceTier = 'low';
  if(roas>=3) performanceTier='high'; else if(roas>=1.5) performanceTier='medium';
  var lifecycle = getS(layer1,'lifecycle');
  var portfolioCell = 'test';
  if(lifecycle==='NEW') portfolioCell='test';
  else if(lifecycle==='WARMING') portfolioCell = (performanceTier==='high')?'potential':'test';
  else if(lifecycle==='CALIBRATED') {
    if(performanceTier==='high') portfolioCell='scale'; else if(performanceTier==='medium') portfolioCell='maintain'; else portfolioCell='fix';
  } else if(lifecycle==='MATURE') {
    if(performanceTier==='high') portfolioCell='scale'; else if(performanceTier==='medium') portfolioCell='maintain'; else portfolioCell='recover';
  }
  report.log += '\n2. healthState=' + healthState + ', performanceTier=' + performanceTier + ', portfolioCell=' + portfolioCell;
  var output = { chs: chs, healthState: healthState, performanceTier: performanceTier, stage: 'stable', portfolioCell: portfolioCell, diagnoses: [] };
  return { output: output, report: report };
}`

// scriptFlagBased — Logic chung cho rules flag → action.
// Input: ctx.layers.flag (object flags đã set), ctx.params (triggerFlag/requireFlags, action, ruleCode, reason, value, exceptionFlags, killRulesEnabled, ...)
// Output: action { action_code, ruleCode, reason, value, result_check? } hoặc null khi no_match.
var scriptFlagBased = `function evaluate(ctx) {
  var flags = ctx.layers.flag || {};
  var params = ctx.params || {};
  var report = { log: '' };

  // Bước 1: Kiểm tra trigger — cần triggerFlag HOẶC requireFlags
  var matched = false;
  if (params.triggerFlag) {
    matched = !!flags[params.triggerFlag];
    if (!matched) {
      report.result = 'no_match';
      report.log = '1. Flag ' + params.triggerFlag + ' không có (flags: ' + Object.keys(flags).join(', ') + ')';
      return { output: null, report: report };
    }
    report.log = '1. Flag ' + params.triggerFlag + ' có → tiếp tục';
  } else if (params.requireFlags && params.requireFlags.length > 0) {
    for (var i = 0; i < params.requireFlags.length; i++) {
      if (!flags[params.requireFlags[i]]) {
        report.result = 'no_match';
        report.log = '1. Require flags thiếu ' + params.requireFlags[i] + ' (cần: ' + params.requireFlags.join(', ') + ')';
        return { output: null, report: report };
      }
    }
    matched = true;
    report.log = '1. Tất cả requireFlags có: ' + params.requireFlags.join(', ');
  } else {
    report.result = 'no_match';
    report.log = '1. Thiếu triggerFlag hoặc requireFlags trong params';
    return { output: null, report: report };
  }

  // Bước 2: Kiểm tra exception flags
  var ex = params.exceptionFlags || [];
  for (var j = 0; j < ex.length; j++) {
    if (flags[ex[j]]) {
      report.result = 'no_match';
      report.log += '\n2. Exception: flag ' + ex[j] + ' đang bật → không thực thi';
      return { output: null, report: report };
    }
  }
  report.log += '\n2. Không có exception flag';

  // Bước 3: Kiểm tra killRulesEnabled + freeze
  if (params.killRulesEnabled === false && params.freeze === true) {
    report.result = 'no_match';
    report.log += '\n3. KillRulesEnabled=false, rule freeze';
    return { output: null, report: report };
  }
  report.log += '\n3. KillRulesEnabled/freeze OK';

  // Bước 4: PATCH 04 — Window Shopping: suspend nếu trong cửa sổ WS và chưa có safety kill
  if (params.skipMessTrapWindowShopping && params.windowShoppingPattern && params.isBefore1400) {
    var safetyKill = (params.msgRateRatio > 0 && params.msgRateRatio < 0.01) || (params.cpmVnd > 0 && params.cpmVnd < 40000);
    if (!safetyKill) {
      report.result = 'no_match';
      report.log += '\n4. PATCH 04: Window shopping, suspend (msgRateRatio=' + (params.msgRateRatio || 0) + ', cpmVnd=' + (params.cpmVnd || 0) + ')';
      return { output: null, report: report };
    }
  }
  report.log += '\n4. PATCH 04 OK';

  // Bước 5: Match — tạo action
  report.result = 'match';
  var flag = params.triggerFlag || (params.requireFlags && params.requireFlags.length ? params.requireFlags.join(',') : 'flags');
  report.log += '\n5. Match: ' + flag + ' → ' + (params.action || params.ruleCode || 'action') + ': ' + (params.reason || '');
  var val = params.value !== undefined ? params.value : null;
  var action = { action_code: params.action, ruleCode: params.ruleCode, reason: params.reason, value: val };
  if (params.resultCheckConfig) { action.result_check = params.resultCheckConfig; }
  return { output: action, report: report };
}`

// SeedRuleAdsSystem seed toàn bộ rules Ads (Kill, Decrease, Increase) — OwnerOrganizationID + IsSystem.
func SeedRuleAdsSystem(ctx context.Context) error {
	systemOrgID := GetSystemOrgIDForSeed(ctx)
	if err := seedOutputContract(ctx, systemOrgID); err != nil {
		return err
	}
	if err := seedLogicScripts(ctx, systemOrgID); err != nil {
		return err
	}
	if err := seedParamSets(ctx, systemOrgID); err != nil {
		return err
	}
	if err := seedRuleDefinitions(ctx, systemOrgID); err != nil {
		return err
	}
	return nil
}

func seedOutputContract(ctx context.Context, systemOrgID primitive.ObjectID) error {
	svc, err := service.NewOutputContractService()
	if err != nil {
		return err
	}
	oc := models.OutputContract{
		OutputID:            "OUT_ACTION_CANDIDATE",
		OutputVersion:       1,
		OutputType:          "action",
		OwnerOrganizationID: systemOrgID,
		IsSystem:            true,
		SchemaDefinition: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"action_code":  map[string]interface{}{"type": "string", "enum": []string{"PAUSE", "DECREASE", "INCREASE", "RESUME", "ARCHIVE"}},
				"reason":       map[string]interface{}{"type": "string"},
				"result_check": map[string]interface{}{"type": "object", "description": "Cấu hình check kết quả sau khi thực thi: afterHours, source, fields"},
			},
		},
		RequiredFields: []string{"action_code", "reason"},
	}
	_, err = svc.Upsert(ctx, bson.M{"output_id": oc.OutputID, "output_version": oc.OutputVersion}, oc)
	if err != nil {
		return err
	}
	ocFlag := models.OutputContract{
		OutputID:            "OUT_FLAG_CANDIDATE",
		OutputVersion:       1,
		OutputType:          "flag",
		OwnerOrganizationID: systemOrgID,
		IsSystem:            true,
		SchemaDefinition: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"flag":  map[string]interface{}{"type": "string"},
				"value": map[string]interface{}{"type": "boolean"},
			},
		},
		RequiredFields: []string{"flag"},
	}
	_, err = svc.Upsert(ctx, bson.M{"output_id": ocFlag.OutputID, "output_version": ocFlag.OutputVersion}, ocFlag)
	if err != nil {
		return err
	}
	ocLayer3 := models.OutputContract{
		OutputID: "OUT_LAYER3", OutputVersion: 1, OutputType: "metric_layer",
		OwnerOrganizationID: systemOrgID, IsSystem: true,
		SchemaDefinition: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"chs": map[string]interface{}{"type": "number"}, "healthState": map[string]interface{}{"type": "string"},
				"performanceTier": map[string]interface{}{"type": "string"}, "portfolioCell": map[string]interface{}{"type": "string"},
			},
		},
		RequiredFields: []string{"chs", "healthState", "portfolioCell"},
	}
	_, err = svc.Upsert(ctx, bson.M{"output_id": ocLayer3.OutputID, "output_version": ocLayer3.OutputVersion}, ocLayer3)
	if err != nil {
		return err
	}
	ocLayer2 := models.OutputContract{
		OutputID: "OUT_LAYER2", OutputVersion: 1, OutputType: "metric_layer",
		OwnerOrganizationID: systemOrgID, IsSystem: true,
		SchemaDefinition: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"efficiency": map[string]interface{}{"type": "number"}, "demandQuality": map[string]interface{}{"type": "number"},
				"auctionPressure": map[string]interface{}{"type": "number"}, "saturation": map[string]interface{}{"type": "number"}, "momentum": map[string]interface{}{"type": "number"},
			},
		},
		RequiredFields: []string{"efficiency", "demandQuality", "auctionPressure", "saturation", "momentum"},
	}
	_, err = svc.Upsert(ctx, bson.M{"output_id": ocLayer2.OutputID, "output_version": ocLayer2.OutputVersion}, ocLayer2)
	if err != nil {
		return err
	}
	ocLayer1 := models.OutputContract{
		OutputID: "OUT_LAYER1", OutputVersion: 1, OutputType: "metric_layer",
		OwnerOrganizationID: systemOrgID, IsSystem: true,
		SchemaDefinition: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"lifecycle": map[string]interface{}{"type": "string"}, "msgRate_7d": map[string]interface{}{"type": "number"},
				"cpaMess_7d": map[string]interface{}{"type": "number"}, "convRate_7d": map[string]interface{}{"type": "number"},
				"roas_7d": map[string]interface{}{"type": "number"}, "mqs_7d": map[string]interface{}{"type": "number"},
				"spendPct_7d": map[string]interface{}{"type": "number"}, "runtimeMinutes": map[string]interface{}{"type": "number"},
			},
		},
		RequiredFields: []string{"lifecycle", "convRate_7d", "roas_7d"},
	}
	_, err = svc.Upsert(ctx, bson.M{"output_id": ocLayer1.OutputID, "output_version": ocLayer1.OutputVersion}, ocLayer1)
	return err
}

func seedLogicScripts(ctx context.Context, systemOrgID primitive.ObjectID) error {
	svc, err := service.NewLogicScriptService()
	if err != nil {
		return err
	}
	scripts := []models.LogicScript{
		// sl_a — full logic (đã có trong seed_rule_ads_kill.go, gọi từ đây)
		{LogicID: "LOGIC_ADS_KILL_SL_A", LogicVersion: 1, LogicType: "script", Runtime: "goja", EntryFunction: "evaluate", Status: "active", OwnerOrganizationID: systemOrgID, IsSystem: true, Script: scriptSlA},
		// Flag-based kill rules
		{LogicID: "LOGIC_ADS_ACTION_FLAG_BASED", LogicVersion: 1, LogicType: "script", Runtime: "goja", EntryFunction: "evaluate", Status: "active", OwnerOrganizationID: systemOrgID, IsSystem: true, Script: scriptFlagBased},
		{LogicID: "LOGIC_ADS_FLAG_WINDOW_SHOPPING", LogicVersion: 1, LogicType: "script", Runtime: "goja", EntryFunction: "evaluate", Status: "active", OwnerOrganizationID: systemOrgID, IsSystem: true, Script: scriptFlagWindowShopping},
		{LogicID: "LOGIC_ADS_FLAG_MO_ELIGIBLE", LogicVersion: 1, LogicType: "script", Runtime: "goja", EntryFunction: "evaluate", Status: "active", OwnerOrganizationID: systemOrgID, IsSystem: true, Script: scriptFlagMoEligible},
		{LogicID: "LOGIC_ADS_FLAG_SL_B", LogicVersion: 1, LogicType: "script", Runtime: "goja", EntryFunction: "evaluate", Status: "active", OwnerOrganizationID: systemOrgID, IsSystem: true, Script: scriptFlagSlB},
		{LogicID: "LOGIC_ADS_FLAG_NOON_CUT_ELIGIBLE", LogicVersion: 1, LogicType: "script", Runtime: "goja", EntryFunction: "evaluate", Status: "active", OwnerOrganizationID: systemOrgID, IsSystem: true, Script: scriptFlagNoonCutEligible},
		{LogicID: "LOGIC_ADS_FLAG_SAFETY_NET", LogicVersion: 1, LogicType: "script", Runtime: "goja", EntryFunction: "evaluate", Status: "active", OwnerOrganizationID: systemOrgID, IsSystem: true, Script: scriptFlagSafetyNet},
		{LogicID: "LOGIC_ADS_FLAG_INCREASE_ELIGIBLE", LogicVersion: 1, LogicType: "script", Runtime: "goja", EntryFunction: "evaluate", Status: "active", OwnerOrganizationID: systemOrgID, IsSystem: true, Script: scriptFlagIncreaseEligible},
		{LogicID: "LOGIC_ADS_KILL_NIGHT_OFF", LogicVersion: 1, LogicType: "script", Runtime: "goja", EntryFunction: "evaluate", Status: "active", OwnerOrganizationID: systemOrgID, IsSystem: true, Script: scriptNightOff},
		{LogicID: "LOGIC_ADS_LAYER3", LogicVersion: 1, LogicType: "script", Runtime: "goja", EntryFunction: "evaluate", Status: "active", OwnerOrganizationID: systemOrgID, IsSystem: true, Script: scriptLayer3},
		{LogicID: "LOGIC_ADS_LAYER2", LogicVersion: 1, LogicType: "script", Runtime: "goja", EntryFunction: "evaluate", Status: "active", OwnerOrganizationID: systemOrgID, IsSystem: true, Script: scriptLayer2},
		{LogicID: "LOGIC_ADS_LAYER1", LogicVersion: 1, LogicType: "script", Runtime: "goja", EntryFunction: "evaluate", Status: "active", OwnerOrganizationID: systemOrgID, IsSystem: true, Script: scriptLayer1},
		// 23 flags còn lại
		{LogicID: "LOGIC_ADS_FLAG_CPA_MESS_HIGH", LogicVersion: 1, LogicType: "script", Runtime: "goja", EntryFunction: "evaluate", Status: "active", OwnerOrganizationID: systemOrgID, IsSystem: true, Script: scriptFlagCpaMessHigh},
		{LogicID: "LOGIC_ADS_FLAG_CPA_PURCHASE_HIGH", LogicVersion: 1, LogicType: "script", Runtime: "goja", EntryFunction: "evaluate", Status: "active", OwnerOrganizationID: systemOrgID, IsSystem: true, Script: scriptFlagCpaPurchaseHigh},
		{LogicID: "LOGIC_ADS_FLAG_CONV_RATE_LOW", LogicVersion: 1, LogicType: "script", Runtime: "goja", EntryFunction: "evaluate", Status: "active", OwnerOrganizationID: systemOrgID, IsSystem: true, Script: scriptFlagConvRateLow},
		{LogicID: "LOGIC_ADS_FLAG_CTR_CRITICAL", LogicVersion: 1, LogicType: "script", Runtime: "goja", EntryFunction: "evaluate", Status: "active", OwnerOrganizationID: systemOrgID, IsSystem: true, Script: scriptFlagCtrCritical},
		{LogicID: "LOGIC_ADS_FLAG_MSG_RATE_LOW", LogicVersion: 1, LogicType: "script", Runtime: "goja", EntryFunction: "evaluate", Status: "active", OwnerOrganizationID: systemOrgID, IsSystem: true, Script: scriptFlagMsgRateLow},
		{LogicID: "LOGIC_ADS_FLAG_CPM_LOW", LogicVersion: 1, LogicType: "script", Runtime: "goja", EntryFunction: "evaluate", Status: "active", OwnerOrganizationID: systemOrgID, IsSystem: true, Script: scriptFlagCpmLow},
		{LogicID: "LOGIC_ADS_FLAG_CPM_HIGH", LogicVersion: 1, LogicType: "script", Runtime: "goja", EntryFunction: "evaluate", Status: "active", OwnerOrganizationID: systemOrgID, IsSystem: true, Script: scriptFlagCpmHigh},
		{LogicID: "LOGIC_ADS_FLAG_FREQUENCY_HIGH", LogicVersion: 1, LogicType: "script", Runtime: "goja", EntryFunction: "evaluate", Status: "active", OwnerOrganizationID: systemOrgID, IsSystem: true, Script: scriptFlagFrequencyHigh},
		{LogicID: "LOGIC_ADS_FLAG_CHS_CRITICAL", LogicVersion: 1, LogicType: "script", Runtime: "goja", EntryFunction: "evaluate", Status: "active", OwnerOrganizationID: systemOrgID, IsSystem: true, Script: scriptFlagChsCritical},
		{LogicID: "LOGIC_ADS_FLAG_CHS_WARNING", LogicVersion: 1, LogicType: "script", Runtime: "goja", EntryFunction: "evaluate", Status: "active", OwnerOrganizationID: systemOrgID, IsSystem: true, Script: scriptFlagChsWarning},
		{LogicID: "LOGIC_ADS_FLAG_SL_A", LogicVersion: 1, LogicType: "script", Runtime: "goja", EntryFunction: "evaluate", Status: "active", OwnerOrganizationID: systemOrgID, IsSystem: true, Script: scriptFlagSlA},
		{LogicID: "LOGIC_ADS_FLAG_SL_A_DECREASE", LogicVersion: 1, LogicType: "script", Runtime: "goja", EntryFunction: "evaluate", Status: "active", OwnerOrganizationID: systemOrgID, IsSystem: true, Script: scriptFlagSlADecrease},
		{LogicID: "LOGIC_ADS_FLAG_SL_C", LogicVersion: 1, LogicType: "script", Runtime: "goja", EntryFunction: "evaluate", Status: "active", OwnerOrganizationID: systemOrgID, IsSystem: true, Script: scriptFlagSlC},
		{LogicID: "LOGIC_ADS_FLAG_SL_D", LogicVersion: 1, LogicType: "script", Runtime: "goja", EntryFunction: "evaluate", Status: "active", OwnerOrganizationID: systemOrgID, IsSystem: true, Script: scriptFlagSlD},
		{LogicID: "LOGIC_ADS_FLAG_SL_E", LogicVersion: 1, LogicType: "script", Runtime: "goja", EntryFunction: "evaluate", Status: "active", OwnerOrganizationID: systemOrgID, IsSystem: true, Script: scriptFlagSlE},
		{LogicID: "LOGIC_ADS_FLAG_KO_A", LogicVersion: 1, LogicType: "script", Runtime: "goja", EntryFunction: "evaluate", Status: "active", OwnerOrganizationID: systemOrgID, IsSystem: true, Script: scriptFlagKoA},
		{LogicID: "LOGIC_ADS_FLAG_KO_B", LogicVersion: 1, LogicType: "script", Runtime: "goja", EntryFunction: "evaluate", Status: "active", OwnerOrganizationID: systemOrgID, IsSystem: true, Script: scriptFlagKoB},
		{LogicID: "LOGIC_ADS_FLAG_KO_C", LogicVersion: 1, LogicType: "script", Runtime: "goja", EntryFunction: "evaluate", Status: "active", OwnerOrganizationID: systemOrgID, IsSystem: true, Script: scriptFlagKoC},
		{LogicID: "LOGIC_ADS_FLAG_MESS_TRAP_SUSPECT", LogicVersion: 1, LogicType: "script", Runtime: "goja", EntryFunction: "evaluate", Status: "active", OwnerOrganizationID: systemOrgID, IsSystem: true, Script: scriptFlagMessTrapSuspect},
		{LogicID: "LOGIC_ADS_FLAG_TRIM_ELIGIBLE", LogicVersion: 1, LogicType: "script", Runtime: "goja", EntryFunction: "evaluate", Status: "active", OwnerOrganizationID: systemOrgID, IsSystem: true, Script: scriptFlagTrimEligible},
		{LogicID: "LOGIC_ADS_FLAG_TRIM_ELIGIBLE_DECREASE", LogicVersion: 1, LogicType: "script", Runtime: "goja", EntryFunction: "evaluate", Status: "active", OwnerOrganizationID: systemOrgID, IsSystem: true, Script: scriptFlagTrimEligibleDecrease},
		{LogicID: "LOGIC_ADS_FLAG_CONV_RATE_STRONG", LogicVersion: 1, LogicType: "script", Runtime: "goja", EntryFunction: "evaluate", Status: "active", OwnerOrganizationID: systemOrgID, IsSystem: true, Script: scriptFlagConvRateStrong},
		{LogicID: "LOGIC_ADS_FLAG_PORTFOLIO_ATTENTION", LogicVersion: 1, LogicType: "script", Runtime: "goja", EntryFunction: "evaluate", Status: "active", OwnerOrganizationID: systemOrgID, IsSystem: true, Script: scriptFlagPortfolioAttention},
	}
	for _, s := range scripts {
		if _, err := svc.Upsert(ctx, bson.M{"logic_id": s.LogicID, "logic_version": s.LogicVersion}, s); err != nil {
			return err
		}
	}
	return nil
}

func seedParamSets(ctx context.Context, systemOrgID primitive.ObjectID) error {
	svc, err := service.NewParamSetService()
	if err != nil {
		return err
	}
	sets := []models.ParamSet{
		{ParamSetID: "PARAM_ADS_KILL_SL_A", ParamVersion: 1, OwnerOrganizationID: systemOrgID, IsSystem: true, Domain: "ads", Segment: "default", Parameters: map[string]interface{}{
			"th_spendPctBase": 0.20, "th_runtimeMin": 90, "th_cpaMessKill": 180000, "th_messMax": 3, "th_mqsMin": 1, "th_mqsDecreaseMin": 2, "th_convRateException": 0.20,
			"resultCheckConfig": map[string]interface{}{"afterHours": 4, "source": "siblings", "fields": []string{"cr", "orders"}},
		}},
		{ParamSetID: "PARAM_ADS_KILL_SL_B", ParamVersion: 1, OwnerOrganizationID: systemOrgID, IsSystem: true, Domain: "ads", Segment: "default", Parameters: map[string]interface{}{
			"triggerFlag": "sl_b", "action": "PAUSE", "ruleCode": "sl_b", "reason": "Hệ thống đề xuất [SL-B]: Có spend nhưng 0 mess — Blitz/Protect", "freeze": false,
			"resultCheckConfig": map[string]interface{}{"afterHours": 4, "source": "siblings", "fields": []string{"cr", "orders"}},
		}},
		{ParamSetID: "PARAM_ADS_KILL_SL_C", ParamVersion: 1, OwnerOrganizationID: systemOrgID, IsSystem: true, Domain: "ads", Segment: "default", Parameters: map[string]interface{}{
			"triggerFlag": "sl_c", "action": "PAUSE", "ruleCode": "sl_c", "reason": "Hệ thống đề xuất [SL-C]: CTR thảm họa, CPM tăng bất thường", "freeze": false,
			"resultCheckConfig": map[string]interface{}{"afterHours": 4, "source": "siblings", "fields": []string{"cr", "orders"}},
		}},
		{ParamSetID: "PARAM_ADS_KILL_SL_D", ParamVersion: 1, OwnerOrganizationID: systemOrgID, IsSystem: true, Domain: "ads", Segment: "default", Parameters: map[string]interface{}{
			"triggerFlag": "sl_d", "action": "PAUSE", "ruleCode": "sl_d", "reason": "Hệ thống đề xuất [SL-D]: Mess Trap — mess đủ mẫu nhưng CR thấp", "freeze": true,
			"resultCheckConfig": map[string]interface{}{"afterHours": 4, "source": "siblings", "fields": []string{"cr", "orders"}},
		}},
		{ParamSetID: "PARAM_ADS_KILL_SL_E", ParamVersion: 1, OwnerOrganizationID: systemOrgID, IsSystem: true, Domain: "ads", Segment: "default", Parameters: map[string]interface{}{
			"triggerFlag": "sl_e", "action": "PAUSE", "ruleCode": "sl_e", "reason": "Hệ thống đề xuất [SL-E]: CPA Purchase vượt ngưỡng, CR thấp", "freeze": true,
			"resultCheckConfig": map[string]interface{}{"afterHours": 4, "source": "siblings", "fields": []string{"cr", "orders"}},
		}},
		{ParamSetID: "PARAM_ADS_KILL_CHS", ParamVersion: 1, OwnerOrganizationID: systemOrgID, IsSystem: true, Domain: "ads", Segment: "default", Parameters: map[string]interface{}{
			"triggerFlag": "chs_critical", "action": "PAUSE", "ruleCode": "chs_critical", "reason": "Hệ thống đề xuất [CHS]: Camp Health Score critical 2 checkpoint liên tiếp", "freeze": true,
			"resultCheckConfig": map[string]interface{}{"afterHours": 4, "source": "siblings", "fields": []string{"cr", "orders"}},
		}},
		{ParamSetID: "PARAM_ADS_KILL_KO_A", ParamVersion: 1, OwnerOrganizationID: systemOrgID, IsSystem: true, Domain: "ads", Segment: "default", Parameters: map[string]interface{}{
			"triggerFlag": "ko_a", "action": "PAUSE", "ruleCode": "ko_a", "reason": "Hệ thống đề xuất [KO-A]: Không delivery — LIMITED/NOT_DELIVERING", "freeze": false,
			"resultCheckConfig": map[string]interface{}{"afterHours": 4, "source": "siblings", "fields": []string{"cr", "orders"}},
		}},
		{ParamSetID: "PARAM_ADS_KILL_KO_B", ParamVersion: 1, OwnerOrganizationID: systemOrgID, IsSystem: true, Domain: "ads", Segment: "default", Parameters: map[string]interface{}{
			"triggerFlag": "ko_b", "action": "PAUSE", "ruleCode": "ko_b", "reason": "Hệ thống đề xuất [KO-B]: Traffic rác — CTR cao, msg rate thấp, 0 đơn", "freeze": true,
			"resultCheckConfig": map[string]interface{}{"afterHours": 4, "source": "siblings", "fields": []string{"cr", "orders"}},
		}},
		{ParamSetID: "PARAM_ADS_KILL_KO_C", ParamVersion: 1, OwnerOrganizationID: systemOrgID, IsSystem: true, Domain: "ads", Segment: "default", Parameters: map[string]interface{}{
			"triggerFlag": "ko_c", "action": "PAUSE", "ruleCode": "ko_c", "reason": "Hệ thống đề xuất [KO-C]: CPM bất thường, impressions thấp", "freeze": false,
			"resultCheckConfig": map[string]interface{}{"afterHours": 4, "source": "siblings", "fields": []string{"cr", "orders"}},
		}},
		{ParamSetID: "PARAM_ADS_KILL_TRIM", ParamVersion: 1, OwnerOrganizationID: systemOrgID, IsSystem: true, Domain: "ads", Segment: "default", Parameters: map[string]interface{}{
			"triggerFlag": "trim_eligible", "action": "PAUSE", "ruleCode": "trim_eligible", "reason": "Hệ thống đề xuất [Trim]: Frequency cao, CHS trung bình — Kill", "freeze": false,
			"resultCheckConfig": map[string]interface{}{"afterHours": 4, "source": "siblings", "fields": []string{"cr", "orders"}},
		}},
		// Decrease
		{ParamSetID: "PARAM_ADS_DECREASE_SL_A", ParamVersion: 1, OwnerOrganizationID: systemOrgID, IsSystem: true, Domain: "ads", Segment: "default", Parameters: map[string]interface{}{
			"triggerFlag": "sl_a_decrease", "action": "DECREASE", "ruleCode": "sl_a_decrease", "reason": "Hệ thống đề xuất [SL-A]: CPA mess cao nhưng MQS >= 2 — giảm budget 20% thay vì kill", "value": 20,
		}},
		{ParamSetID: "PARAM_ADS_DECREASE_MESS_TRAP", ParamVersion: 1, OwnerOrganizationID: systemOrgID, IsSystem: true, Domain: "ads", Segment: "default", Parameters: map[string]interface{}{
			"triggerFlag": "mess_trap_suspect", "action": "DECREASE", "ruleCode": "mess_trap_suspect", "reason": "Hệ thống đề xuất [Mess Trap]: Nghi ngờ bẫy mess — giảm budget 30%", "value": 30,
		}},
		{ParamSetID: "PARAM_ADS_DECREASE_TRIM", ParamVersion: 1, OwnerOrganizationID: systemOrgID, IsSystem: true, Domain: "ads", Segment: "default", Parameters: map[string]interface{}{
			"triggerFlag": "trim_eligible_decrease", "action": "DECREASE", "ruleCode": "trim_eligible_decrease", "reason": "Hệ thống đề xuất [Trim]: Frequency cao, có đơn — giảm budget 30% thay vì kill", "value": 30,
		}},
		{ParamSetID: "PARAM_ADS_DECREASE_CHS", ParamVersion: 1, OwnerOrganizationID: systemOrgID, IsSystem: true, Domain: "ads", Segment: "default", Parameters: map[string]interface{}{
			"requireFlags": []interface{}{"chs_warning", "cpa_mess_high"}, "action": "DECREASE", "ruleCode": "chs_warning", "reason": "Hệ thống đề xuất [CHS Warning]: CPA mess cao, CHS warning — giảm budget 15%", "value": 15,
		}},
		// Increase
		{ParamSetID: "PARAM_ADS_INCREASE_ELIGIBLE", ParamVersion: 1, OwnerOrganizationID: systemOrgID, IsSystem: true, Domain: "ads", Segment: "default", Parameters: map[string]interface{}{
			"triggerFlag": "increase_eligible", "action": "INCREASE", "ruleCode": "increase_eligible", "reason": "Hệ thống đề xuất [Increase]: Camp tốt — CR > 12%, CHS < 1.3, tăng budget 30%", "value": 30,
		}},
		{ParamSetID: "PARAM_ADS_INCREASE_SAFETY", ParamVersion: 1, OwnerOrganizationID: systemOrgID, IsSystem: true, Domain: "ads", Segment: "default", Parameters: map[string]interface{}{
			"triggerFlag": "safety_net", "action": "INCREASE", "ruleCode": "increase_safety_net", "reason": "Hệ thống đề xuất [Increase]: Safety Net — camp tốt, tăng 35%", "value": 35,
		}},
		// Scheduler rules (morning_on, noon_cut)
		{ParamSetID: "PARAM_ADS_RESUME_MORNING_ON", ParamVersion: 1, OwnerOrganizationID: systemOrgID, IsSystem: true, Domain: "ads", Segment: "default", Parameters: map[string]interface{}{
			"triggerFlag": "mo_eligible", "action": "RESUME", "ruleCode": "morning_on", "reason": "Hệ thống đề xuất [Morning On]: Camp đủ điều kiện bật lại sáng (MO-A)", "exceptionFlags": []interface{}{"sl_a", "sl_b", "sl_c", "sl_d", "sl_e", "chs_critical", "ko_a", "ko_b", "ko_c", "trim_eligible"},
		}},
		{ParamSetID: "PARAM_ADS_KILL_NOON_CUT", ParamVersion: 1, OwnerOrganizationID: systemOrgID, IsSystem: true, Domain: "ads", Segment: "default", Parameters: map[string]interface{}{
			"triggerFlag": "noon_cut_eligible", "action": "PAUSE", "ruleCode": "noon_cut", "reason": "Noon Cut — camp chết buổi trưa, bật lại 14:30", "exceptionFlags": []interface{}{"safety_net"},
			"resultCheckConfig": map[string]interface{}{"afterHours": 4, "source": "siblings", "fields": []string{"cr", "orders"}},
		}},
		{ParamSetID: "PARAM_ADS_FLAG_WINDOW_SHOPPING", ParamVersion: 1, OwnerOrganizationID: systemOrgID, IsSystem: true, Domain: "ads", Segment: "default", Parameters: map[string]interface{}{
			"th_mess_increase_pct": 1.5, "th_cr_07_12_max": 5, "th_cr_yesterday_1222_min": 10,
		}},
		{ParamSetID: "PARAM_ADS_FLAG_MO_ELIGIBLE", ParamVersion: 1, OwnerOrganizationID: systemOrgID, IsSystem: true, Domain: "ads", Segment: "default", Parameters: map[string]interface{}{
			"th_cpaMessMoMax": 216000,
		}},
		{ParamSetID: "PARAM_ADS_FLAG_SL_B", ParamVersion: 1, OwnerOrganizationID: systemOrgID, IsSystem: true, Domain: "ads", Segment: "default", Parameters: map[string]interface{}{
			"th_spendPctSlB": 0.30, "th_spendPctSlBBlitz": 0.20, "th_runtimeMinutesBase": 90,
		}},
		{ParamSetID: "PARAM_ADS_FLAG_NOON_CUT_ELIGIBLE", ParamVersion: 1, OwnerOrganizationID: systemOrgID, IsSystem: true, Domain: "ads", Segment: "default", Parameters: map[string]interface{}{
			"th_cpaMessNoonCutMin": 144000, "th_spendPctNoonCutMax": 0.55, "th_spendPctBase": 0.20,
		}},
		{ParamSetID: "PARAM_ADS_FLAG_SAFETY_NET", ParamVersion: 1, OwnerOrganizationID: systemOrgID, IsSystem: true, Domain: "ads", Segment: "default", Parameters: map[string]interface{}{
			"th_safetyNetOrdersMin": 3, "th_safetyNetCrMin": 0.10, "th_chsMin": 60,
		}},
		{ParamSetID: "PARAM_ADS_FLAG_INCREASE_ELIGIBLE", ParamVersion: 1, OwnerOrganizationID: systemOrgID, IsSystem: true, Domain: "ads", Segment: "default", Parameters: map[string]interface{}{}},
		// 23 flags còn lại
		{ParamSetID: "PARAM_ADS_FLAG_CPA_MESS_HIGH", ParamVersion: 1, OwnerOrganizationID: systemOrgID, IsSystem: true, Domain: "ads", Segment: "default", Parameters: map[string]interface{}{"th_cpaMessKill": 180000}},
		{ParamSetID: "PARAM_ADS_FLAG_CPA_PURCHASE_HIGH", ParamVersion: 1, OwnerOrganizationID: systemOrgID, IsSystem: true, Domain: "ads", Segment: "default", Parameters: map[string]interface{}{"th_cpaPurchaseHardStop": 1050000}},
		{ParamSetID: "PARAM_ADS_FLAG_CONV_RATE_LOW", ParamVersion: 1, OwnerOrganizationID: systemOrgID, IsSystem: true, Domain: "ads", Segment: "default", Parameters: map[string]interface{}{"th_convRateMessTrap": 0.05, "th_messTrapSlDMin": 15}},
		{ParamSetID: "PARAM_ADS_FLAG_CTR_CRITICAL", ParamVersion: 1, OwnerOrganizationID: systemOrgID, IsSystem: true, Domain: "ads", Segment: "default", Parameters: map[string]interface{}{"th_ctrKill": 0.0035}},
		{ParamSetID: "PARAM_ADS_FLAG_MSG_RATE_LOW", ParamVersion: 1, OwnerOrganizationID: systemOrgID, IsSystem: true, Domain: "ads", Segment: "default", Parameters: map[string]interface{}{"th_msgRateLow": 0.02}},
		{ParamSetID: "PARAM_ADS_FLAG_CPM_LOW", ParamVersion: 1, OwnerOrganizationID: systemOrgID, IsSystem: true, Domain: "ads", Segment: "default", Parameters: map[string]interface{}{"th_cpmMessTrapLow": 60000}},
		{ParamSetID: "PARAM_ADS_FLAG_CPM_HIGH", ParamVersion: 1, OwnerOrganizationID: systemOrgID, IsSystem: true, Domain: "ads", Segment: "default", Parameters: map[string]interface{}{"th_cpmHigh": 180000}},
		{ParamSetID: "PARAM_ADS_FLAG_FREQUENCY_HIGH", ParamVersion: 1, OwnerOrganizationID: systemOrgID, IsSystem: true, Domain: "ads", Segment: "default", Parameters: map[string]interface{}{"th_frequencyHigh": 3.0}},
		{ParamSetID: "PARAM_ADS_FLAG_CHS_CRITICAL", ParamVersion: 1, OwnerOrganizationID: systemOrgID, IsSystem: true, Domain: "ads", Segment: "default", Parameters: map[string]interface{}{"th_chsWarningThreshold": 40}},
		{ParamSetID: "PARAM_ADS_FLAG_CHS_WARNING", ParamVersion: 1, OwnerOrganizationID: systemOrgID, IsSystem: true, Domain: "ads", Segment: "default", Parameters: map[string]interface{}{"th_chsWarningThreshold": 40}},
		{ParamSetID: "PARAM_ADS_FLAG_SL_A", ParamVersion: 1, OwnerOrganizationID: systemOrgID, IsSystem: true, Domain: "ads", Segment: "default", Parameters: map[string]interface{}{"th_spendPctBase": 0.2, "th_runtimeMinutesBase": 90, "th_cpaMessKill": 180000, "th_mqsSlAMax": 1}},
		{ParamSetID: "PARAM_ADS_FLAG_SL_A_DECREASE", ParamVersion: 1, OwnerOrganizationID: systemOrgID, IsSystem: true, Domain: "ads", Segment: "default", Parameters: map[string]interface{}{"th_spendPctBase": 0.2, "th_runtimeMinutesBase": 90, "th_cpaMessKill": 180000, "th_mqsSlADecreaseMin": 2}},
		{ParamSetID: "PARAM_ADS_FLAG_SL_C", ParamVersion: 1, OwnerOrganizationID: systemOrgID, IsSystem: true, Domain: "ads", Segment: "default", Parameters: map[string]interface{}{"th_spendPctBase": 0.2, "th_runtimeMinutesBase": 90, "th_ctrKill": 0.0035, "th_spendPctSlC": 0.15, "th_cpmHigh": 180000, "th_msgRateLow": 0.02}},
		{ParamSetID: "PARAM_ADS_FLAG_SL_D", ParamVersion: 1, OwnerOrganizationID: systemOrgID, IsSystem: true, Domain: "ads", Segment: "default", Parameters: map[string]interface{}{"th_spendPctBase": 0.2, "th_runtimeMinutesBase": 90, "th_messTrapSlDMin": 15, "th_convRateMessTrap": 0.05, "th_spendPctSlD": 0.2}},
		{ParamSetID: "PARAM_ADS_FLAG_SL_E", ParamVersion: 1, OwnerOrganizationID: systemOrgID, IsSystem: true, Domain: "ads", Segment: "default", Parameters: map[string]interface{}{"th_spendPctBase": 0.2, "th_runtimeMinutesBase": 90, "th_cpaPurchaseHardStop": 1050000, "th_slEOrdersMin": 3, "th_slECrMax": 0.1, "th_mqsSlEMax": 1}},
		{ParamSetID: "PARAM_ADS_FLAG_KO_A", ParamVersion: 1, OwnerOrganizationID: systemOrgID, IsSystem: true, Domain: "ads", Segment: "default", Parameters: map[string]interface{}{"th_runtimeMinutesKoA": 120, "th_spendPctKoAMax": 0.08}},
		{ParamSetID: "PARAM_ADS_FLAG_KO_B", ParamVersion: 1, OwnerOrganizationID: systemOrgID, IsSystem: true, Domain: "ads", Segment: "default", Parameters: map[string]interface{}{"th_ctrTrafficRac": 0.018, "th_msgRateLow": 0.02, "th_spendPctKoB": 0.15, "th_mqsKoBMax": 0.5}},
		{ParamSetID: "PARAM_ADS_FLAG_KO_C", ParamVersion: 1, OwnerOrganizationID: systemOrgID, IsSystem: true, Domain: "ads", Segment: "default", Parameters: map[string]interface{}{"th_cpmHigh": 180000, "th_cpmKoCMultiplier": 2.5, "th_spendPctKoC": 0.1}},
		{ParamSetID: "PARAM_ADS_FLAG_MESS_TRAP_SUSPECT", ParamVersion: 1, OwnerOrganizationID: systemOrgID, IsSystem: true, Domain: "ads", Segment: "default", Parameters: map[string]interface{}{"th_cpaMessTrapLow": 60000, "th_convRateMessTrap6": 0.06, "th_messTrapSuspectMin": 20, "th_spendPctMessTrap": 0.15}},
		{ParamSetID: "PARAM_ADS_FLAG_TRIM_ELIGIBLE", ParamVersion: 1, OwnerOrganizationID: systemOrgID, IsSystem: true, Domain: "ads", Segment: "default", Parameters: map[string]interface{}{"th_frequencyTrim": 2.2, "th_trimOrdersMin": 3}},
		{ParamSetID: "PARAM_ADS_FLAG_TRIM_ELIGIBLE_DECREASE", ParamVersion: 1, OwnerOrganizationID: systemOrgID, IsSystem: true, Domain: "ads", Segment: "default", Parameters: map[string]interface{}{"th_frequencyTrim": 2.2, "th_trimOrdersMin": 3}},
		{ParamSetID: "PARAM_ADS_FLAG_CONV_RATE_STRONG", ParamVersion: 1, OwnerOrganizationID: systemOrgID, IsSystem: true, Domain: "ads", Segment: "default", Parameters: map[string]interface{}{"th_convRateStrong": 0.2}},
		{ParamSetID: "PARAM_ADS_FLAG_PORTFOLIO_ATTENTION", ParamVersion: 1, OwnerOrganizationID: systemOrgID, IsSystem: true, Domain: "ads", Segment: "default", Parameters: map[string]interface{}{}},
		{ParamSetID: "PARAM_ADS_RESUME_NOON_CUT", ParamVersion: 1, OwnerOrganizationID: systemOrgID, IsSystem: true, Domain: "ads", Segment: "default", Parameters: map[string]interface{}{
			"triggerFlag": "was_paused_by_noon_cut", "action": "RESUME", "ruleCode": "noon_cut_resume", "reason": "Noon Cut Resume 14:30 — bật lại camp đã tắt trưa",
		}},
		{ParamSetID: "PARAM_ADS_KILL_NIGHT_OFF", ParamVersion: 1, OwnerOrganizationID: systemOrgID, IsSystem: true, Domain: "ads", Segment: "default", Parameters: map[string]interface{}{
			"hour_protect": 21, "hour_efficiency": 22, "hour_normal": 22, "minute_normal": 30, "hour_blitz": 23,
		}},
		{ParamSetID: "PARAM_ADS_LAYER3", ParamVersion: 1, OwnerOrganizationID: systemOrgID, IsSystem: true, Domain: "ads", Segment: "default", Parameters: map[string]interface{}{}},
		{ParamSetID: "PARAM_ADS_LAYER2", ParamVersion: 1, OwnerOrganizationID: systemOrgID, IsSystem: true, Domain: "ads", Segment: "default", Parameters: map[string]interface{}{}},
		{ParamSetID: "PARAM_ADS_LAYER1", ParamVersion: 1, OwnerOrganizationID: systemOrgID, IsSystem: true, Domain: "ads", Segment: "default", Parameters: map[string]interface{}{}},
	}
	for _, ps := range sets {
		if _, err := svc.Upsert(ctx, bson.M{"param_set_id": ps.ParamSetID, "param_version": ps.ParamVersion}, ps); err != nil {
			return err
		}
	}
	return nil
}

func seedRuleDefinitions(ctx context.Context, systemOrgID primitive.ObjectID) error {
	svc, err := service.NewRuleDefinitionService()
	if err != nil {
		return err
	}
	rules := []models.RuleDefinition{
		{RuleID: "RULE_ADS_KILL_SL_A", RuleVersion: 1, RuleCode: "sl_a", Domain: "ads", FromLayer: "flag", ToLayer: "action", OwnerOrganizationID: systemOrgID, IsSystem: true, Priority: 1,
			InputRef: models.InputRef{SchemaRef: "schema_ads_layer1", RequiredFields: []string{"spendPct_7d", "runtimeMinutes", "cpaMess_7d", "convRate_7d", "mqs_7d", "lifecycle"}},
			LogicRef: models.LogicRef{LogicID: "LOGIC_ADS_KILL_SL_A", LogicVersion: 1}, ParamRef: models.ParamRef{ParamSetID: "PARAM_ADS_KILL_SL_A", ParamVersion: 1},
			OutputRef: models.OutputRef{OutputID: "OUT_ACTION_CANDIDATE", OutputVersion: 1}, Status: "active", Metadata: map[string]string{"label": "SL-A"}},
		{RuleID: "RULE_ADS_KILL_SL_B", RuleVersion: 1, RuleCode: "sl_b", Domain: "ads", FromLayer: "flag", ToLayer: "action", OwnerOrganizationID: systemOrgID, IsSystem: true, Priority: 2,
			InputRef: models.InputRef{SchemaRef: "schema_ads_flag", RequiredFields: []string{"sl_b"}},
			LogicRef: models.LogicRef{LogicID: "LOGIC_ADS_ACTION_FLAG_BASED", LogicVersion: 1}, ParamRef: models.ParamRef{ParamSetID: "PARAM_ADS_KILL_SL_B", ParamVersion: 1},
			OutputRef: models.OutputRef{OutputID: "OUT_ACTION_CANDIDATE", OutputVersion: 1}, Status: "active", Metadata: map[string]string{"label": "SL-B"}},
		{RuleID: "RULE_ADS_KILL_SL_C", RuleVersion: 1, RuleCode: "sl_c", Domain: "ads", FromLayer: "flag", ToLayer: "action", OwnerOrganizationID: systemOrgID, IsSystem: true, Priority: 3,
			InputRef: models.InputRef{SchemaRef: "schema_ads_flag", RequiredFields: []string{"sl_c"}},
			LogicRef: models.LogicRef{LogicID: "LOGIC_ADS_ACTION_FLAG_BASED", LogicVersion: 1}, ParamRef: models.ParamRef{ParamSetID: "PARAM_ADS_KILL_SL_C", ParamVersion: 1},
			OutputRef: models.OutputRef{OutputID: "OUT_ACTION_CANDIDATE", OutputVersion: 1}, Status: "active", Metadata: map[string]string{"label": "SL-C"}},
		{RuleID: "RULE_ADS_KILL_SL_D", RuleVersion: 1, RuleCode: "sl_d", Domain: "ads", FromLayer: "flag", ToLayer: "action", OwnerOrganizationID: systemOrgID, IsSystem: true, Priority: 4,
			InputRef: models.InputRef{SchemaRef: "schema_ads_flag", RequiredFields: []string{"sl_d"}},
			LogicRef: models.LogicRef{LogicID: "LOGIC_ADS_ACTION_FLAG_BASED", LogicVersion: 1}, ParamRef: models.ParamRef{ParamSetID: "PARAM_ADS_KILL_SL_D", ParamVersion: 1},
			OutputRef: models.OutputRef{OutputID: "OUT_ACTION_CANDIDATE", OutputVersion: 1}, Status: "active", Metadata: map[string]string{"label": "SL-D"}},
		{RuleID: "RULE_ADS_KILL_SL_E", RuleVersion: 1, RuleCode: "sl_e", Domain: "ads", FromLayer: "flag", ToLayer: "action", OwnerOrganizationID: systemOrgID, IsSystem: true, Priority: 5,
			InputRef: models.InputRef{SchemaRef: "schema_ads_flag", RequiredFields: []string{"sl_e"}},
			LogicRef: models.LogicRef{LogicID: "LOGIC_ADS_ACTION_FLAG_BASED", LogicVersion: 1}, ParamRef: models.ParamRef{ParamSetID: "PARAM_ADS_KILL_SL_E", ParamVersion: 1},
			OutputRef: models.OutputRef{OutputID: "OUT_ACTION_CANDIDATE", OutputVersion: 1}, Status: "active", Metadata: map[string]string{"label": "SL-E"}},
		{RuleID: "RULE_ADS_KILL_CHS", RuleVersion: 1, RuleCode: "chs_critical", Domain: "ads", FromLayer: "flag", ToLayer: "action", OwnerOrganizationID: systemOrgID, IsSystem: true, Priority: 6,
			InputRef: models.InputRef{SchemaRef: "schema_ads_flag", RequiredFields: []string{"chs_critical"}},
			LogicRef: models.LogicRef{LogicID: "LOGIC_ADS_ACTION_FLAG_BASED", LogicVersion: 1}, ParamRef: models.ParamRef{ParamSetID: "PARAM_ADS_KILL_CHS", ParamVersion: 1},
			OutputRef: models.OutputRef{OutputID: "OUT_ACTION_CANDIDATE", OutputVersion: 1}, Status: "active", Metadata: map[string]string{"label": "CHS Critical"}},
		{RuleID: "RULE_ADS_KILL_KO_A", RuleVersion: 1, RuleCode: "ko_a", Domain: "ads", FromLayer: "flag", ToLayer: "action", OwnerOrganizationID: systemOrgID, IsSystem: true, Priority: 7,
			InputRef: models.InputRef{SchemaRef: "schema_ads_flag", RequiredFields: []string{"ko_a"}},
			LogicRef: models.LogicRef{LogicID: "LOGIC_ADS_ACTION_FLAG_BASED", LogicVersion: 1}, ParamRef: models.ParamRef{ParamSetID: "PARAM_ADS_KILL_KO_A", ParamVersion: 1},
			OutputRef: models.OutputRef{OutputID: "OUT_ACTION_CANDIDATE", OutputVersion: 1}, Status: "active", Metadata: map[string]string{"label": "KO-A"}},
		{RuleID: "RULE_ADS_KILL_KO_B", RuleVersion: 1, RuleCode: "ko_b", Domain: "ads", FromLayer: "flag", ToLayer: "action", OwnerOrganizationID: systemOrgID, IsSystem: true, Priority: 8,
			InputRef: models.InputRef{SchemaRef: "schema_ads_flag", RequiredFields: []string{"ko_b"}},
			LogicRef: models.LogicRef{LogicID: "LOGIC_ADS_ACTION_FLAG_BASED", LogicVersion: 1}, ParamRef: models.ParamRef{ParamSetID: "PARAM_ADS_KILL_KO_B", ParamVersion: 1},
			OutputRef: models.OutputRef{OutputID: "OUT_ACTION_CANDIDATE", OutputVersion: 1}, Status: "active", Metadata: map[string]string{"label": "KO-B"}},
		{RuleID: "RULE_ADS_KILL_KO_C", RuleVersion: 1, RuleCode: "ko_c", Domain: "ads", FromLayer: "flag", ToLayer: "action", OwnerOrganizationID: systemOrgID, IsSystem: true, Priority: 9,
			InputRef: models.InputRef{SchemaRef: "schema_ads_flag", RequiredFields: []string{"ko_c"}},
			LogicRef: models.LogicRef{LogicID: "LOGIC_ADS_ACTION_FLAG_BASED", LogicVersion: 1}, ParamRef: models.ParamRef{ParamSetID: "PARAM_ADS_KILL_KO_C", ParamVersion: 1},
			OutputRef: models.OutputRef{OutputID: "OUT_ACTION_CANDIDATE", OutputVersion: 1}, Status: "active", Metadata: map[string]string{"label": "KO-C"}},
		{RuleID: "RULE_ADS_KILL_TRIM", RuleVersion: 1, RuleCode: "trim_eligible", Domain: "ads", FromLayer: "flag", ToLayer: "action", OwnerOrganizationID: systemOrgID, IsSystem: true, Priority: 10,
			InputRef: models.InputRef{SchemaRef: "schema_ads_flag", RequiredFields: []string{"trim_eligible"}},
			LogicRef: models.LogicRef{LogicID: "LOGIC_ADS_ACTION_FLAG_BASED", LogicVersion: 1}, ParamRef: models.ParamRef{ParamSetID: "PARAM_ADS_KILL_TRIM", ParamVersion: 1},
			OutputRef: models.OutputRef{OutputID: "OUT_ACTION_CANDIDATE", OutputVersion: 1}, Status: "active", Metadata: map[string]string{"label": "Trim"}},
		// Decrease
		{RuleID: "RULE_ADS_DECREASE_SL_A", RuleVersion: 1, RuleCode: "sl_a_decrease", Domain: "ads", FromLayer: "flag", ToLayer: "action", OwnerOrganizationID: systemOrgID, IsSystem: true, Priority: 11,
			InputRef: models.InputRef{SchemaRef: "schema_ads_flag", RequiredFields: []string{"sl_a_decrease"}},
			LogicRef: models.LogicRef{LogicID: "LOGIC_ADS_ACTION_FLAG_BASED", LogicVersion: 1}, ParamRef: models.ParamRef{ParamSetID: "PARAM_ADS_DECREASE_SL_A", ParamVersion: 1},
			OutputRef: models.OutputRef{OutputID: "OUT_ACTION_CANDIDATE", OutputVersion: 1}, Status: "active", Metadata: map[string]string{"label": "SL-A Decrease"}},
		{RuleID: "RULE_ADS_DECREASE_MESS_TRAP", RuleVersion: 1, RuleCode: "mess_trap_suspect", Domain: "ads", FromLayer: "flag", ToLayer: "action", OwnerOrganizationID: systemOrgID, IsSystem: true, Priority: 12,
			InputRef: models.InputRef{SchemaRef: "schema_ads_flag", RequiredFields: []string{"mess_trap_suspect"}},
			LogicRef: models.LogicRef{LogicID: "LOGIC_ADS_ACTION_FLAG_BASED", LogicVersion: 1}, ParamRef: models.ParamRef{ParamSetID: "PARAM_ADS_DECREASE_MESS_TRAP", ParamVersion: 1},
			OutputRef: models.OutputRef{OutputID: "OUT_ACTION_CANDIDATE", OutputVersion: 1}, Status: "active", Metadata: map[string]string{"label": "Mess Trap Suspect"}},
		{RuleID: "RULE_ADS_DECREASE_TRIM", RuleVersion: 1, RuleCode: "trim_eligible_decrease", Domain: "ads", FromLayer: "flag", ToLayer: "action", OwnerOrganizationID: systemOrgID, IsSystem: true, Priority: 13,
			InputRef: models.InputRef{SchemaRef: "schema_ads_flag", RequiredFields: []string{"trim_eligible_decrease"}},
			LogicRef: models.LogicRef{LogicID: "LOGIC_ADS_ACTION_FLAG_BASED", LogicVersion: 1}, ParamRef: models.ParamRef{ParamSetID: "PARAM_ADS_DECREASE_TRIM", ParamVersion: 1},
			OutputRef: models.OutputRef{OutputID: "OUT_ACTION_CANDIDATE", OutputVersion: 1}, Status: "active", Metadata: map[string]string{"label": "Trim Decrease"}},
		{RuleID: "RULE_ADS_DECREASE_CHS", RuleVersion: 1, RuleCode: "chs_warning", Domain: "ads", FromLayer: "flag", ToLayer: "action", OwnerOrganizationID: systemOrgID, IsSystem: true, Priority: 14,
			InputRef: models.InputRef{SchemaRef: "schema_ads_flag", RequiredFields: []string{"chs_warning", "cpa_mess_high"}},
			LogicRef: models.LogicRef{LogicID: "LOGIC_ADS_ACTION_FLAG_BASED", LogicVersion: 1}, ParamRef: models.ParamRef{ParamSetID: "PARAM_ADS_DECREASE_CHS", ParamVersion: 1},
			OutputRef: models.OutputRef{OutputID: "OUT_ACTION_CANDIDATE", OutputVersion: 1}, Status: "active", Metadata: map[string]string{"label": "CHS Warning"}},
		// Increase
		{RuleID: "RULE_ADS_INCREASE_ELIGIBLE", RuleVersion: 1, RuleCode: "increase_eligible", Domain: "ads", FromLayer: "flag", ToLayer: "action", OwnerOrganizationID: systemOrgID, IsSystem: true, Priority: 15,
			InputRef: models.InputRef{SchemaRef: "schema_ads_flag", RequiredFields: []string{"increase_eligible"}},
			LogicRef: models.LogicRef{LogicID: "LOGIC_ADS_ACTION_FLAG_BASED", LogicVersion: 1}, ParamRef: models.ParamRef{ParamSetID: "PARAM_ADS_INCREASE_ELIGIBLE", ParamVersion: 1},
			OutputRef: models.OutputRef{OutputID: "OUT_ACTION_CANDIDATE", OutputVersion: 1}, Status: "active", Metadata: map[string]string{"label": "Increase"}},
		{RuleID: "RULE_ADS_INCREASE_SAFETY", RuleVersion: 1, RuleCode: "increase_safety_net", Domain: "ads", FromLayer: "flag", ToLayer: "action", OwnerOrganizationID: systemOrgID, IsSystem: true, Priority: 16,
			InputRef: models.InputRef{SchemaRef: "schema_ads_flag", RequiredFields: []string{"safety_net"}},
			LogicRef: models.LogicRef{LogicID: "LOGIC_ADS_ACTION_FLAG_BASED", LogicVersion: 1}, ParamRef: models.ParamRef{ParamSetID: "PARAM_ADS_INCREASE_SAFETY", ParamVersion: 1},
			OutputRef: models.OutputRef{OutputID: "OUT_ACTION_CANDIDATE", OutputVersion: 1}, Status: "active", Metadata: map[string]string{"label": "Increase Safety Net"}},
		// Scheduler rules
		{RuleID: "RULE_ADS_RESUME_MORNING_ON", RuleVersion: 1, RuleCode: "morning_on", Domain: "ads", FromLayer: "flag", ToLayer: "action", OwnerOrganizationID: systemOrgID, IsSystem: true, Priority: 17,
			InputRef: models.InputRef{SchemaRef: "schema_ads_flag", RequiredFields: []string{"mo_eligible"}},
			LogicRef: models.LogicRef{LogicID: "LOGIC_ADS_ACTION_FLAG_BASED", LogicVersion: 1}, ParamRef: models.ParamRef{ParamSetID: "PARAM_ADS_RESUME_MORNING_ON", ParamVersion: 1},
			OutputRef: models.OutputRef{OutputID: "OUT_ACTION_CANDIDATE", OutputVersion: 1}, Status: "active", Metadata: map[string]string{"label": "Morning On"}},
		{RuleID: "RULE_ADS_KILL_NOON_CUT", RuleVersion: 1, RuleCode: "noon_cut", Domain: "ads", FromLayer: "flag", ToLayer: "action", OwnerOrganizationID: systemOrgID, IsSystem: true, Priority: 18,
			InputRef: models.InputRef{SchemaRef: "schema_ads_flag", RequiredFields: []string{"noon_cut_eligible"}},
			LogicRef: models.LogicRef{LogicID: "LOGIC_ADS_ACTION_FLAG_BASED", LogicVersion: 1}, ParamRef: models.ParamRef{ParamSetID: "PARAM_ADS_KILL_NOON_CUT", ParamVersion: 1},
			OutputRef: models.OutputRef{OutputID: "OUT_ACTION_CANDIDATE", OutputVersion: 1}, Status: "active", Metadata: map[string]string{"label": "Noon Cut"}},
		// Interpretation: metric/params → flag
		{RuleID: "RULE_ADS_FLAG_WINDOW_SHOPPING", RuleVersion: 1, RuleCode: "window_shopping_pattern", Domain: "ads", FromLayer: "raw", ToLayer: "flag", OwnerOrganizationID: systemOrgID, IsSystem: true, Priority: 19,
			InputRef: models.InputRef{SchemaRef: "schema_ads_window_shopping", RequiredFields: []string{"mess_07_12_today", "mess_07_12_yesterday", "orders_07_12_today", "mess_12_22_yesterday", "orders_12_22_yesterday", "in_event_window"}},
			LogicRef: models.LogicRef{LogicID: "LOGIC_ADS_FLAG_WINDOW_SHOPPING", LogicVersion: 1}, ParamRef: models.ParamRef{ParamSetID: "PARAM_ADS_FLAG_WINDOW_SHOPPING", ParamVersion: 1},
			OutputRef: models.OutputRef{OutputID: "OUT_FLAG_CANDIDATE", OutputVersion: 1}, Status: "active", Metadata: map[string]string{"label": "Window Shopping Pattern"}},
		{RuleID: "RULE_ADS_FLAG_MO_ELIGIBLE", RuleVersion: 1, RuleCode: "mo_eligible", Domain: "ads", FromLayer: "layer1", ToLayer: "flag", OwnerOrganizationID: systemOrgID, IsSystem: true, Priority: 20,
			InputRef: models.InputRef{SchemaRef: "schema_ads_layer1", RequiredFields: []string{"cpaMess_7d", "convRate_7d", "mess", "orders", "frequency"}},
			LogicRef: models.LogicRef{LogicID: "LOGIC_ADS_FLAG_MO_ELIGIBLE", LogicVersion: 1}, ParamRef: models.ParamRef{ParamSetID: "PARAM_ADS_FLAG_MO_ELIGIBLE", ParamVersion: 1},
			OutputRef: models.OutputRef{OutputID: "OUT_FLAG_CANDIDATE", OutputVersion: 1}, Status: "active", Metadata: map[string]string{"label": "Morning On Eligible"}},
		{RuleID: "RULE_ADS_FLAG_SL_B", RuleVersion: 1, RuleCode: "sl_b", Domain: "ads", FromLayer: "layer1", ToLayer: "flag", OwnerOrganizationID: systemOrgID, IsSystem: true, Priority: 21,
			InputRef: models.InputRef{SchemaRef: "schema_ads_layer1", RequiredFields: []string{"spendPct_7d", "runtimeMinutes", "mess"}},
			LogicRef: models.LogicRef{LogicID: "LOGIC_ADS_FLAG_SL_B", LogicVersion: 1}, ParamRef: models.ParamRef{ParamSetID: "PARAM_ADS_FLAG_SL_B", ParamVersion: 1},
			OutputRef: models.OutputRef{OutputID: "OUT_FLAG_CANDIDATE", OutputVersion: 1}, Status: "active", Metadata: map[string]string{"label": "SL-B"}},
		{RuleID: "RULE_ADS_FLAG_NOON_CUT_ELIGIBLE", RuleVersion: 1, RuleCode: "noon_cut_eligible", Domain: "ads", FromLayer: "layer1", ToLayer: "flag", OwnerOrganizationID: systemOrgID, IsSystem: true, Priority: 22,
			InputRef: models.InputRef{SchemaRef: "schema_ads_layer1", RequiredFields: []string{"cpaMess_7d", "spendPct_7d", "healthState"}},
			LogicRef: models.LogicRef{LogicID: "LOGIC_ADS_FLAG_NOON_CUT_ELIGIBLE", LogicVersion: 1}, ParamRef: models.ParamRef{ParamSetID: "PARAM_ADS_FLAG_NOON_CUT_ELIGIBLE", ParamVersion: 1},
			OutputRef: models.OutputRef{OutputID: "OUT_FLAG_CANDIDATE", OutputVersion: 1}, Status: "active", Metadata: map[string]string{"label": "Noon Cut Eligible"}},
		{RuleID: "RULE_ADS_FLAG_SAFETY_NET", RuleVersion: 1, RuleCode: "safety_net", Domain: "ads", FromLayer: "layer1", ToLayer: "flag", OwnerOrganizationID: systemOrgID, IsSystem: true, Priority: 23,
			InputRef: models.InputRef{SchemaRef: "schema_ads_layer1", RequiredFields: []string{"orders", "convRate_7d", "chs"}},
			LogicRef: models.LogicRef{LogicID: "LOGIC_ADS_FLAG_SAFETY_NET", LogicVersion: 1}, ParamRef: models.ParamRef{ParamSetID: "PARAM_ADS_FLAG_SAFETY_NET", ParamVersion: 1},
			OutputRef: models.OutputRef{OutputID: "OUT_FLAG_CANDIDATE", OutputVersion: 1}, Status: "active", Metadata: map[string]string{"label": "Safety Net"}},
		{RuleID: "RULE_ADS_FLAG_INCREASE_ELIGIBLE", RuleVersion: 1, RuleCode: "increase_eligible", Domain: "ads", FromLayer: "layer1", ToLayer: "flag", OwnerOrganizationID: systemOrgID, IsSystem: true, Priority: 24,
			InputRef: models.InputRef{SchemaRef: "schema_ads_layer1", RequiredFields: []string{"convRate_7d", "frequency", "spendPct_7d", "chs"}},
			LogicRef: models.LogicRef{LogicID: "LOGIC_ADS_FLAG_INCREASE_ELIGIBLE", LogicVersion: 1}, ParamRef: models.ParamRef{ParamSetID: "PARAM_ADS_FLAG_INCREASE_ELIGIBLE", ParamVersion: 1},
			OutputRef: models.OutputRef{OutputID: "OUT_FLAG_CANDIDATE", OutputVersion: 1}, Status: "active", Metadata: map[string]string{"label": "Increase Eligible"}},
		// 23 flags còn lại
		{RuleID: "RULE_ADS_FLAG_CPA_MESS_HIGH", RuleVersion: 1, RuleCode: "cpa_mess_high", Domain: "ads", FromLayer: "layer1", ToLayer: "flag", OwnerOrganizationID: systemOrgID, IsSystem: true, Priority: 25,
			InputRef: models.InputRef{SchemaRef: "schema_ads_layer1", RequiredFields: []string{"cpaMess_7d", "mess"}},
			LogicRef: models.LogicRef{LogicID: "LOGIC_ADS_FLAG_CPA_MESS_HIGH", LogicVersion: 1}, ParamRef: models.ParamRef{ParamSetID: "PARAM_ADS_FLAG_CPA_MESS_HIGH", ParamVersion: 1},
			OutputRef: models.OutputRef{OutputID: "OUT_FLAG_CANDIDATE", OutputVersion: 1}, Status: "active", Metadata: map[string]string{"label": "CPA Mess cao"}},
		{RuleID: "RULE_ADS_FLAG_CPA_PURCHASE_HIGH", RuleVersion: 1, RuleCode: "cpa_purchase_high", Domain: "ads", FromLayer: "layer1", ToLayer: "flag", OwnerOrganizationID: systemOrgID, IsSystem: true, Priority: 26,
			InputRef: models.InputRef{SchemaRef: "schema_ads_layer1", RequiredFields: []string{"cpaPurchase_7d", "orders"}},
			LogicRef: models.LogicRef{LogicID: "LOGIC_ADS_FLAG_CPA_PURCHASE_HIGH", LogicVersion: 1}, ParamRef: models.ParamRef{ParamSetID: "PARAM_ADS_FLAG_CPA_PURCHASE_HIGH", ParamVersion: 1},
			OutputRef: models.OutputRef{OutputID: "OUT_FLAG_CANDIDATE", OutputVersion: 1}, Status: "active", Metadata: map[string]string{"label": "CPA Purchase cao"}},
		{RuleID: "RULE_ADS_FLAG_CONV_RATE_LOW", RuleVersion: 1, RuleCode: "conv_rate_low", Domain: "ads", FromLayer: "layer1", ToLayer: "flag", OwnerOrganizationID: systemOrgID, IsSystem: true, Priority: 27,
			InputRef: models.InputRef{SchemaRef: "schema_ads_layer1", RequiredFields: []string{"convRate_7d", "mess"}},
			LogicRef: models.LogicRef{LogicID: "LOGIC_ADS_FLAG_CONV_RATE_LOW", LogicVersion: 1}, ParamRef: models.ParamRef{ParamSetID: "PARAM_ADS_FLAG_CONV_RATE_LOW", ParamVersion: 1},
			OutputRef: models.OutputRef{OutputID: "OUT_FLAG_CANDIDATE", OutputVersion: 1}, Status: "active", Metadata: map[string]string{"label": "Conv Rate thấp"}},
		{RuleID: "RULE_ADS_FLAG_CTR_CRITICAL", RuleVersion: 1, RuleCode: "ctr_critical", Domain: "ads", FromLayer: "layer1", ToLayer: "flag", OwnerOrganizationID: systemOrgID, IsSystem: true, Priority: 28,
			InputRef: models.InputRef{SchemaRef: "schema_ads_layer1", RequiredFields: []string{"ctr"}},
			LogicRef: models.LogicRef{LogicID: "LOGIC_ADS_FLAG_CTR_CRITICAL", LogicVersion: 1}, ParamRef: models.ParamRef{ParamSetID: "PARAM_ADS_FLAG_CTR_CRITICAL", ParamVersion: 1},
			OutputRef: models.OutputRef{OutputID: "OUT_FLAG_CANDIDATE", OutputVersion: 1}, Status: "active", Metadata: map[string]string{"label": "CTR thảm họa"}},
		{RuleID: "RULE_ADS_FLAG_MSG_RATE_LOW", RuleVersion: 1, RuleCode: "msg_rate_low", Domain: "ads", FromLayer: "layer1", ToLayer: "flag", OwnerOrganizationID: systemOrgID, IsSystem: true, Priority: 29,
			InputRef: models.InputRef{SchemaRef: "schema_ads_layer1", RequiredFields: []string{"msgRate_7d"}},
			LogicRef: models.LogicRef{LogicID: "LOGIC_ADS_FLAG_MSG_RATE_LOW", LogicVersion: 1}, ParamRef: models.ParamRef{ParamSetID: "PARAM_ADS_FLAG_MSG_RATE_LOW", ParamVersion: 1},
			OutputRef: models.OutputRef{OutputID: "OUT_FLAG_CANDIDATE", OutputVersion: 1}, Status: "active", Metadata: map[string]string{"label": "Msg Rate thấp"}},
		{RuleID: "RULE_ADS_FLAG_CPM_LOW", RuleVersion: 1, RuleCode: "cpm_low", Domain: "ads", FromLayer: "layer1", ToLayer: "flag", OwnerOrganizationID: systemOrgID, IsSystem: true, Priority: 30,
			InputRef: models.InputRef{SchemaRef: "schema_ads_layer1", RequiredFields: []string{"cpm"}},
			LogicRef: models.LogicRef{LogicID: "LOGIC_ADS_FLAG_CPM_LOW", LogicVersion: 1}, ParamRef: models.ParamRef{ParamSetID: "PARAM_ADS_FLAG_CPM_LOW", ParamVersion: 1},
			OutputRef: models.OutputRef{OutputID: "OUT_FLAG_CANDIDATE", OutputVersion: 1}, Status: "active", Metadata: map[string]string{"label": "CPM thấp"}},
		{RuleID: "RULE_ADS_FLAG_CPM_HIGH", RuleVersion: 1, RuleCode: "cpm_high", Domain: "ads", FromLayer: "layer1", ToLayer: "flag", OwnerOrganizationID: systemOrgID, IsSystem: true, Priority: 31,
			InputRef: models.InputRef{SchemaRef: "schema_ads_layer1", RequiredFields: []string{"cpm"}},
			LogicRef: models.LogicRef{LogicID: "LOGIC_ADS_FLAG_CPM_HIGH", LogicVersion: 1}, ParamRef: models.ParamRef{ParamSetID: "PARAM_ADS_FLAG_CPM_HIGH", ParamVersion: 1},
			OutputRef: models.OutputRef{OutputID: "OUT_FLAG_CANDIDATE", OutputVersion: 1}, Status: "active", Metadata: map[string]string{"label": "CPM cao"}},
		{RuleID: "RULE_ADS_FLAG_FREQUENCY_HIGH", RuleVersion: 1, RuleCode: "frequency_high", Domain: "ads", FromLayer: "layer1", ToLayer: "flag", OwnerOrganizationID: systemOrgID, IsSystem: true, Priority: 32,
			InputRef: models.InputRef{SchemaRef: "schema_ads_layer1", RequiredFields: []string{"frequency"}},
			LogicRef: models.LogicRef{LogicID: "LOGIC_ADS_FLAG_FREQUENCY_HIGH", LogicVersion: 1}, ParamRef: models.ParamRef{ParamSetID: "PARAM_ADS_FLAG_FREQUENCY_HIGH", ParamVersion: 1},
			OutputRef: models.OutputRef{OutputID: "OUT_FLAG_CANDIDATE", OutputVersion: 1}, Status: "active", Metadata: map[string]string{"label": "Frequency cao"}},
		{RuleID: "RULE_ADS_FLAG_CHS_CRITICAL", RuleVersion: 1, RuleCode: "chs_critical", Domain: "ads", FromLayer: "layer1", ToLayer: "flag", OwnerOrganizationID: systemOrgID, IsSystem: true, Priority: 33,
			InputRef: models.InputRef{SchemaRef: "schema_ads_layer1", RequiredFields: []string{"healthState", "chs"}},
			LogicRef: models.LogicRef{LogicID: "LOGIC_ADS_FLAG_CHS_CRITICAL", LogicVersion: 1}, ParamRef: models.ParamRef{ParamSetID: "PARAM_ADS_FLAG_CHS_CRITICAL", ParamVersion: 1},
			OutputRef: models.OutputRef{OutputID: "OUT_FLAG_CANDIDATE", OutputVersion: 1}, Status: "active", Metadata: map[string]string{"label": "CHS Critical"}},
		{RuleID: "RULE_ADS_FLAG_CHS_WARNING", RuleVersion: 1, RuleCode: "chs_warning", Domain: "ads", FromLayer: "layer1", ToLayer: "flag", OwnerOrganizationID: systemOrgID, IsSystem: true, Priority: 34,
			InputRef: models.InputRef{SchemaRef: "schema_ads_layer1", RequiredFields: []string{"healthState", "chs"}},
			LogicRef: models.LogicRef{LogicID: "LOGIC_ADS_FLAG_CHS_WARNING", LogicVersion: 1}, ParamRef: models.ParamRef{ParamSetID: "PARAM_ADS_FLAG_CHS_WARNING", ParamVersion: 1},
			OutputRef: models.OutputRef{OutputID: "OUT_FLAG_CANDIDATE", OutputVersion: 1}, Status: "active", Metadata: map[string]string{"label": "CHS Warning"}},
		{RuleID: "RULE_ADS_FLAG_SL_A", RuleVersion: 1, RuleCode: "sl_a", Domain: "ads", FromLayer: "layer1", ToLayer: "flag", OwnerOrganizationID: systemOrgID, IsSystem: true, Priority: 35,
			InputRef: models.InputRef{SchemaRef: "schema_ads_layer1", RequiredFields: []string{"spendPct_7d", "runtimeMinutes", "cpaMess_7d", "mess", "mqs_7d"}},
			LogicRef: models.LogicRef{LogicID: "LOGIC_ADS_FLAG_SL_A", LogicVersion: 1}, ParamRef: models.ParamRef{ParamSetID: "PARAM_ADS_FLAG_SL_A", ParamVersion: 1},
			OutputRef: models.OutputRef{OutputID: "OUT_FLAG_CANDIDATE", OutputVersion: 1}, Status: "active", Metadata: map[string]string{"label": "SL-A"}},
		{RuleID: "RULE_ADS_FLAG_SL_A_DECREASE", RuleVersion: 1, RuleCode: "sl_a_decrease", Domain: "ads", FromLayer: "layer1", ToLayer: "flag", OwnerOrganizationID: systemOrgID, IsSystem: true, Priority: 36,
			InputRef: models.InputRef{SchemaRef: "schema_ads_layer1", RequiredFields: []string{"spendPct_7d", "runtimeMinutes", "cpaMess_7d", "mess", "mqs_7d"}},
			LogicRef: models.LogicRef{LogicID: "LOGIC_ADS_FLAG_SL_A_DECREASE", LogicVersion: 1}, ParamRef: models.ParamRef{ParamSetID: "PARAM_ADS_FLAG_SL_A_DECREASE", ParamVersion: 1},
			OutputRef: models.OutputRef{OutputID: "OUT_FLAG_CANDIDATE", OutputVersion: 1}, Status: "active", Metadata: map[string]string{"label": "SL-A Decrease"}},
		{RuleID: "RULE_ADS_FLAG_SL_C", RuleVersion: 1, RuleCode: "sl_c", Domain: "ads", FromLayer: "layer1", ToLayer: "flag", OwnerOrganizationID: systemOrgID, IsSystem: true, Priority: 37,
			InputRef: models.InputRef{SchemaRef: "schema_ads_layer1", RequiredFields: []string{"spendPct_7d", "runtimeMinutes", "ctr", "cpm", "msgRate_7d"}},
			LogicRef: models.LogicRef{LogicID: "LOGIC_ADS_FLAG_SL_C", LogicVersion: 1}, ParamRef: models.ParamRef{ParamSetID: "PARAM_ADS_FLAG_SL_C", ParamVersion: 1},
			OutputRef: models.OutputRef{OutputID: "OUT_FLAG_CANDIDATE", OutputVersion: 1}, Status: "active", Metadata: map[string]string{"label": "SL-C"}},
		{RuleID: "RULE_ADS_FLAG_SL_D", RuleVersion: 1, RuleCode: "sl_d", Domain: "ads", FromLayer: "layer1", ToLayer: "flag", OwnerOrganizationID: systemOrgID, IsSystem: true, Priority: 38,
			InputRef: models.InputRef{SchemaRef: "schema_ads_layer1", RequiredFields: []string{"spendPct_7d", "runtimeMinutes", "mess", "convRate_7d"}},
			LogicRef: models.LogicRef{LogicID: "LOGIC_ADS_FLAG_SL_D", LogicVersion: 1}, ParamRef: models.ParamRef{ParamSetID: "PARAM_ADS_FLAG_SL_D", ParamVersion: 1},
			OutputRef: models.OutputRef{OutputID: "OUT_FLAG_CANDIDATE", OutputVersion: 1}, Status: "active", Metadata: map[string]string{"label": "SL-D"}},
		{RuleID: "RULE_ADS_FLAG_SL_E", RuleVersion: 1, RuleCode: "sl_e", Domain: "ads", FromLayer: "layer1", ToLayer: "flag", OwnerOrganizationID: systemOrgID, IsSystem: true, Priority: 39,
			InputRef: models.InputRef{SchemaRef: "schema_ads_layer1", RequiredFields: []string{"spendPct_7d", "runtimeMinutes", "cpaPurchase_7d", "orders", "convRate_7d", "mqs_7d"}},
			LogicRef: models.LogicRef{LogicID: "LOGIC_ADS_FLAG_SL_E", LogicVersion: 1}, ParamRef: models.ParamRef{ParamSetID: "PARAM_ADS_FLAG_SL_E", ParamVersion: 1},
			OutputRef: models.OutputRef{OutputID: "OUT_FLAG_CANDIDATE", OutputVersion: 1}, Status: "active", Metadata: map[string]string{"label": "SL-E"}},
		{RuleID: "RULE_ADS_FLAG_KO_A", RuleVersion: 1, RuleCode: "ko_a", Domain: "ads", FromLayer: "layer1", ToLayer: "flag", OwnerOrganizationID: systemOrgID, IsSystem: true, Priority: 40,
			InputRef: models.InputRef{SchemaRef: "schema_ads_layer1", RequiredFields: []string{"deliveryStatus", "runtimeMinutes", "spendPct_7d"}},
			LogicRef: models.LogicRef{LogicID: "LOGIC_ADS_FLAG_KO_A", LogicVersion: 1}, ParamRef: models.ParamRef{ParamSetID: "PARAM_ADS_FLAG_KO_A", ParamVersion: 1},
			OutputRef: models.OutputRef{OutputID: "OUT_FLAG_CANDIDATE", OutputVersion: 1}, Status: "active", Metadata: map[string]string{"label": "KO-A"}},
		{RuleID: "RULE_ADS_FLAG_KO_B", RuleVersion: 1, RuleCode: "ko_b", Domain: "ads", FromLayer: "layer1", ToLayer: "flag", OwnerOrganizationID: systemOrgID, IsSystem: true, Priority: 41,
			InputRef: models.InputRef{SchemaRef: "schema_ads_layer1", RequiredFields: []string{"ctr", "msgRate_7d", "orders", "spendPct_7d", "mqs_7d"}},
			LogicRef: models.LogicRef{LogicID: "LOGIC_ADS_FLAG_KO_B", LogicVersion: 1}, ParamRef: models.ParamRef{ParamSetID: "PARAM_ADS_FLAG_KO_B", ParamVersion: 1},
			OutputRef: models.OutputRef{OutputID: "OUT_FLAG_CANDIDATE", OutputVersion: 1}, Status: "active", Metadata: map[string]string{"label": "KO-B"}},
		{RuleID: "RULE_ADS_FLAG_KO_C", RuleVersion: 1, RuleCode: "ko_c", Domain: "ads", FromLayer: "layer1", ToLayer: "flag", OwnerOrganizationID: systemOrgID, IsSystem: true, Priority: 42,
			InputRef: models.InputRef{SchemaRef: "schema_ads_layer1", RequiredFields: []string{"cpm", "impressions", "spendPct_7d"}},
			LogicRef: models.LogicRef{LogicID: "LOGIC_ADS_FLAG_KO_C", LogicVersion: 1}, ParamRef: models.ParamRef{ParamSetID: "PARAM_ADS_FLAG_KO_C", ParamVersion: 1},
			OutputRef: models.OutputRef{OutputID: "OUT_FLAG_CANDIDATE", OutputVersion: 1}, Status: "active", Metadata: map[string]string{"label": "KO-C"}},
		{RuleID: "RULE_ADS_FLAG_MESS_TRAP_SUSPECT", RuleVersion: 1, RuleCode: "mess_trap_suspect", Domain: "ads", FromLayer: "layer1", ToLayer: "flag", OwnerOrganizationID: systemOrgID, IsSystem: true, Priority: 43,
			InputRef: models.InputRef{SchemaRef: "schema_ads_layer1", RequiredFields: []string{"cpaMess_7d", "convRate_7d", "mess", "orders", "spendPct_7d"}},
			LogicRef: models.LogicRef{LogicID: "LOGIC_ADS_FLAG_MESS_TRAP_SUSPECT", LogicVersion: 1}, ParamRef: models.ParamRef{ParamSetID: "PARAM_ADS_FLAG_MESS_TRAP_SUSPECT", ParamVersion: 1},
			OutputRef: models.OutputRef{OutputID: "OUT_FLAG_CANDIDATE", OutputVersion: 1}, Status: "active", Metadata: map[string]string{"label": "Mess Trap Suspect"}},
		{RuleID: "RULE_ADS_FLAG_TRIM_ELIGIBLE", RuleVersion: 1, RuleCode: "trim_eligible", Domain: "ads", FromLayer: "layer1", ToLayer: "flag", OwnerOrganizationID: systemOrgID, IsSystem: true, Priority: 44,
			InputRef: models.InputRef{SchemaRef: "schema_ads_layer1", RequiredFields: []string{"inTrimWindow", "frequency", "chs", "orders"}},
			LogicRef: models.LogicRef{LogicID: "LOGIC_ADS_FLAG_TRIM_ELIGIBLE", LogicVersion: 1}, ParamRef: models.ParamRef{ParamSetID: "PARAM_ADS_FLAG_TRIM_ELIGIBLE", ParamVersion: 1},
			OutputRef: models.OutputRef{OutputID: "OUT_FLAG_CANDIDATE", OutputVersion: 1}, Status: "active", Metadata: map[string]string{"label": "Trim Kill"}},
		{RuleID: "RULE_ADS_FLAG_TRIM_ELIGIBLE_DECREASE", RuleVersion: 1, RuleCode: "trim_eligible_decrease", Domain: "ads", FromLayer: "layer1", ToLayer: "flag", OwnerOrganizationID: systemOrgID, IsSystem: true, Priority: 45,
			InputRef: models.InputRef{SchemaRef: "schema_ads_layer1", RequiredFields: []string{"inTrimWindow", "frequency", "chs", "orders"}},
			LogicRef: models.LogicRef{LogicID: "LOGIC_ADS_FLAG_TRIM_ELIGIBLE_DECREASE", LogicVersion: 1}, ParamRef: models.ParamRef{ParamSetID: "PARAM_ADS_FLAG_TRIM_ELIGIBLE_DECREASE", ParamVersion: 1},
			OutputRef: models.OutputRef{OutputID: "OUT_FLAG_CANDIDATE", OutputVersion: 1}, Status: "active", Metadata: map[string]string{"label": "Trim Decrease"}},
		{RuleID: "RULE_ADS_FLAG_CONV_RATE_STRONG", RuleVersion: 1, RuleCode: "conv_rate_strong", Domain: "ads", FromLayer: "layer1", ToLayer: "flag", OwnerOrganizationID: systemOrgID, IsSystem: true, Priority: 46,
			InputRef: models.InputRef{SchemaRef: "schema_ads_layer1", RequiredFields: []string{"convRate_7d"}},
			LogicRef: models.LogicRef{LogicID: "LOGIC_ADS_FLAG_CONV_RATE_STRONG", LogicVersion: 1}, ParamRef: models.ParamRef{ParamSetID: "PARAM_ADS_FLAG_CONV_RATE_STRONG", ParamVersion: 1},
			OutputRef: models.OutputRef{OutputID: "OUT_FLAG_CANDIDATE", OutputVersion: 1}, Status: "active", Metadata: map[string]string{"label": "Conv Rate Strong"}},
		{RuleID: "RULE_ADS_FLAG_PORTFOLIO_ATTENTION", RuleVersion: 1, RuleCode: "portfolio_attention", Domain: "ads", FromLayer: "layer1", ToLayer: "flag", OwnerOrganizationID: systemOrgID, IsSystem: true, Priority: 47,
			InputRef: models.InputRef{SchemaRef: "schema_ads_layer1", RequiredFields: []string{"portfolioCell"}},
			LogicRef: models.LogicRef{LogicID: "LOGIC_ADS_FLAG_PORTFOLIO_ATTENTION", LogicVersion: 1}, ParamRef: models.ParamRef{ParamSetID: "PARAM_ADS_FLAG_PORTFOLIO_ATTENTION", ParamVersion: 1},
			OutputRef: models.OutputRef{OutputID: "OUT_FLAG_CANDIDATE", OutputVersion: 1}, Status: "active", Metadata: map[string]string{"label": "Portfolio Attention"}},
		{RuleID: "RULE_ADS_RESUME_NOON_CUT", RuleVersion: 1, RuleCode: "noon_cut_resume", Domain: "ads", FromLayer: "flag", ToLayer: "action", OwnerOrganizationID: systemOrgID, IsSystem: true, Priority: 20,
			InputRef: models.InputRef{SchemaRef: "schema_ads_flag", RequiredFields: []string{"was_paused_by_noon_cut"}},
			LogicRef: models.LogicRef{LogicID: "LOGIC_ADS_ACTION_FLAG_BASED", LogicVersion: 1}, ParamRef: models.ParamRef{ParamSetID: "PARAM_ADS_RESUME_NOON_CUT", ParamVersion: 1},
			OutputRef: models.OutputRef{OutputID: "OUT_ACTION_CANDIDATE", OutputVersion: 1}, Status: "active", Metadata: map[string]string{"label": "Noon Cut Resume"}},
		{RuleID: "RULE_ADS_KILL_NIGHT_OFF", RuleVersion: 1, RuleCode: "night_off", Domain: "ads", FromLayer: "raw", ToLayer: "action", OwnerOrganizationID: systemOrgID, IsSystem: true, Priority: 21,
			InputRef: models.InputRef{SchemaRef: "schema_ads_night_off", RequiredFields: []string{"account_mode", "hour", "minute"}},
			LogicRef: models.LogicRef{LogicID: "LOGIC_ADS_KILL_NIGHT_OFF", LogicVersion: 1}, ParamRef: models.ParamRef{ParamSetID: "PARAM_ADS_KILL_NIGHT_OFF", ParamVersion: 1},
			OutputRef: models.OutputRef{OutputID: "OUT_ACTION_CANDIDATE", OutputVersion: 1}, Status: "active", Metadata: map[string]string{"label": "Night Off"}},
		{RuleID: "RULE_ADS_LAYER3", RuleVersion: 1, RuleCode: "layer3", Domain: "ads", FromLayer: "layer2", ToLayer: "layer3", OwnerOrganizationID: systemOrgID, IsSystem: true, Priority: 1,
			InputRef: models.InputRef{SchemaRef: "schema_ads_layer1", RequiredFields: []string{"roas_7d", "lifecycle"}},
			LogicRef: models.LogicRef{LogicID: "LOGIC_ADS_LAYER3", LogicVersion: 1}, ParamRef: models.ParamRef{ParamSetID: "PARAM_ADS_LAYER3", ParamVersion: 1},
			OutputRef: models.OutputRef{OutputID: "OUT_LAYER3", OutputVersion: 1}, Status: "active", Metadata: map[string]string{"label": "Layer 3 (CHS, healthState, portfolioCell)"}},
		{RuleID: "RULE_ADS_LAYER2", RuleVersion: 1, RuleCode: "layer2", Domain: "ads", FromLayer: "raw", ToLayer: "layer2", OwnerOrganizationID: systemOrgID, IsSystem: true, Priority: 1,
			InputRef: models.InputRef{SchemaRef: "schema_ads_raw", RequiredFields: []string{"meta", "layer1"}},
			LogicRef: models.LogicRef{LogicID: "LOGIC_ADS_LAYER2", LogicVersion: 1}, ParamRef: models.ParamRef{ParamSetID: "PARAM_ADS_LAYER2", ParamVersion: 1},
			OutputRef: models.OutputRef{OutputID: "OUT_LAYER2", OutputVersion: 1}, Status: "active", Metadata: map[string]string{"label": "Layer 2 (scores)"}},
		{RuleID: "RULE_ADS_LAYER1", RuleVersion: 1, RuleCode: "layer1", Domain: "ads", FromLayer: "raw", ToLayer: "layer1", OwnerOrganizationID: systemOrgID, IsSystem: true, Priority: 1,
			InputRef: models.InputRef{SchemaRef: "schema_ads_raw", RequiredFields: []string{"meta", "pancake"}},
			LogicRef: models.LogicRef{LogicID: "LOGIC_ADS_LAYER1", LogicVersion: 1}, ParamRef: models.ParamRef{ParamSetID: "PARAM_ADS_LAYER1", ParamVersion: 1},
			OutputRef: models.OutputRef{OutputID: "OUT_LAYER1", OutputVersion: 1}, Status: "active", Metadata: map[string]string{"label": "Layer 1 (metrics)"}},
	}
	for _, r := range rules {
		if _, err := svc.Upsert(ctx, bson.M{"rule_id": r.RuleID, "domain": r.Domain}, r); err != nil {
			return err
		}
	}
	return nil
}
