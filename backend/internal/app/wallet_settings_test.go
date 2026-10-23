// Package app 验证支付与商家转账配置仅限平台管理员，个人钱包不会泄露商户配置。
package app

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestWalletSettingsNewTransferLimits 对齐新版转账场景36字符及报备字段名15字的官方边界。
func TestWalletSettingsNewTransferLimits(t *testing.T) {
	v := WalletSettings{TransferSceneID: strings.Repeat("a", 36), TransferSceneReportInfos: []WalletSceneReport{{InfoType: strings.Repeat("类", 15), InfoContent: strings.Repeat("内", 32)}}}
	if err := validateWalletSettings(v); err != nil {
		t.Fatal(err)
	}
	v.TransferSceneID += "a"
	if err := validateWalletSettings(v); err == nil {
		t.Fatal("场景编号超过36字符仍通过")
	}
	v.TransferSceneID = "1000"
	v.TransferSceneReportInfos[0].InfoType += "名"
	if err := validateWalletSettings(v); err == nil {
		t.Fatal("报备字段名称超过15字仍通过")
	}
}

// TestIntegrationWalletSettingsAdminOnly 覆盖访客、买家、打手及俱乐部管理员读写后台配置均被拒绝。
func TestIntegrationWalletSettingsAdminOnly(t *testing.T) {
	api := newTestAPI(t)
	for _, actor := range []string{"", "player", "worker", "clubowner", "outsider"} {
		code := 403
		if actor == "" {
			code = 401
		}
		for _, route := range []string{"/admin/wallet-settings", "/admin/payment-settings"} {
			api.call(actor, "GET", route, nil, code, nil)
			api.call(actor, "PATCH", route, map[string]any{}, code, nil)
		}
		api.call(actor, "GET", "/admin/wallet/withdrawals", nil, code, nil)
	}
	var settings map[string]any
	api.call("admin", "GET", "/admin/wallet-settings", nil, 200, &settings)
	if _, ok := settings["settings"]; !ok {
		t.Fatal("管理员未取得配置结构")
	}
	var wallet map[string]any
	api.call("player", "GET", "/wallet", nil, 200, &wallet)
	raw, err := json.Marshal(wallet)
	if err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"merchant_id", "transfer_scene_id", "certificate_serial", "private_key", "api_v3_key", "configuration_notice"} {
		if strings.Contains(string(raw), field) {
			t.Fatal("个人钱包泄露后台配置字段:", field)
		}
	}
}
