// Package app 验证可配置平台最低押金、按差额补缴、公开撤标、旧意图恢复与不改动原资金来源。
package app

import (
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"sync"
	"testing"
)

// pendingDepositPolicyTopup 构造与正式幂等键相同的原付款意图，以验证提额后的按原单恢复路径。
func pendingDepositPolicyTopup(t *testing.T, api *testAPI, user string, amount int64) WalletTopup {
	t.Helper()
	request := "policy-pending-" + newID()
	id := walletKey("topup", user, request)
	p := WalletTopup{ID: id, UserID: user, AccountID: user, RequestID: request, Purpose: "deposit", AmountCents: amount, Status: "pending_payment", OrderNo: "WT" + strings.ToUpper(id[:30]), CreatedAt: now(), UpdatedAt: now()}
	if e := put(t.Context(), api.App.DB, "wallet_topups", id, p); e != nil {
		t.Fatal(e)
	}
	return p
}

// TestDepositPolicyAmounts 验证足额等于门槛、零钱差额和零门槛没有押金时不显示认证。
func TestDepositPolicyAmounts(t *testing.T) {
	for _, tc := range []struct {
		name                        string
		amount, required, shortfall int64
		qualified                   int64
	}{
		{"未缴纳", 0, 10000, 10000, 0},
		{"恰好足额", 10000, 10000, 0, 10000},
		{"差一分", 9999, 10000, 1, 0},
		{"超过门槛", 12000, 10000, 0, 12000},
		{"零门槛未缴", 0, 0, 0, 0},
		{"零门槛已缴", 1, 0, 0, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := depositShortfall(tc.amount, tc.required); got != tc.shortfall {
				t.Fatalf("补缴差额=%d，预期%d", got, tc.shortfall)
			}
			if got := qualifiedPlatformDeposit(Wallet{DepositCents: tc.amount}, tc.required); got != tc.qualified {
				t.Fatalf("公开有效押金=%d，预期%d", got, tc.qualified)
			}
		})
	}
}

// TestIntegrationDepositPolicyPersonalTopup 验证上调门槛只撤标、不退资金，补齐差额后恢复认证且原单不改。
func TestIntegrationDepositPolicyPersonalTopup(t *testing.T) {
	api := newTestAPI(t)
	settings := FinancialSettings{PersonalPlatformDepositCents: 1000, ClubCreationDepositCents: 2000}
	api.call("admin", "PATCH", "/admin/financial-settings", settings, 200, nil)
	api.call("worker", "PATCH", "/admin/financial-settings", settings, 403, nil)
	api.call("worker", "POST", "/wallet/topups", map[string]any{"purpose": "deposit", "amount_cents": 999, "request_id": "policy-personal-under"}, 400, nil)
	payment := testTopup(api, "worker", "deposit", "policy-personal-exact", 1000)
	before, e := get[WalletTopup](t.Context(), api.App.DB, "wallet_topups", payment.ID, false)
	if e != nil {
		t.Fatal(e)
	}
	var summary map[string]any
	api.call("worker", "GET", "/wallet", nil, 200, &summary)
	if summary["deposit_required_cents"] != float64(1000) || summary["deposit_shortfall_cents"] != float64(0) || summary["deposit_qualified"] != true {
		t.Fatalf("初次足额状态错误:%v", summary)
	}
	var service Service
	api.call("", "GET", "/services/delta-service-1", nil, 200, &service)
	if service.ProviderDepositCents != 1000 {
		t.Fatal("足额押金没有公开显示")
	}
	settings.PersonalPlatformDepositCents = 1051
	api.call("admin", "PATCH", "/admin/financial-settings", settings, 200, nil)
	api.call("worker", "GET", "/wallet", nil, 200, &summary)
	if summary["deposit_cents"] != float64(1000) || summary["deposit_shortfall_cents"] != float64(51) || summary["deposit_qualified"] != false || summary["recharge_cents"] != float64(0) || summary["available_cents"] != float64(0) {
		t.Fatalf("上调门槛错误退回或增加收入:%v", summary)
	}
	api.call("", "GET", "/services/delta-service-1", nil, 200, &service)
	if service.ProviderDepositCents != 0 {
		t.Fatal("未足额服务仍显示押金认证")
	}
	var profile struct {
		Profile map[string]any `json:"profile"`
	}
	api.call("", "GET", "/players/worker", nil, 200, &profile)
	if profile.Profile["platform_deposit_cents"] != float64(0) {
		t.Fatalf("未足额个人主页仍显示押金:%v", profile.Profile)
	}
	stored, e := get[WalletTopup](t.Context(), api.App.DB, "wallet_topups", payment.ID, false)
	if e != nil || stored != before {
		t.Fatalf("门槛调整改写原支付凭据:%+v %v", stored, e)
	}
	api.call("worker", "POST", "/wallet/topups", map[string]any{"purpose": "deposit", "amount_cents": 50, "request_id": "policy-personal-short"}, 400, nil)
	testTopup(api, "worker", "deposit", "policy-personal-difference", 51)
	api.call("worker", "GET", "/wallet", nil, 200, &summary)
	if summary["deposit_cents"] != float64(1051) || summary["deposit_shortfall_cents"] != float64(0) || summary["deposit_qualified"] != true {
		t.Fatalf("零钱差额补缴失败:%v", summary)
	}
	api.call("", "GET", "/services/delta-service-1", nil, 200, &service)
	if service.ProviderDepositCents != 1051 {
		t.Fatal("补缴后未恢复认证")
	}
	api.call("worker", "POST", "/wallet/topups", map[string]any{"purpose": "balance", "amount_cents": 51, "request_id": "policy-recharge-original-min"}, 400, nil)
}

// TestIntegrationDepositPolicyOldIntent 验证管理员提额后旧幂等意图与付款回调仍只按原金额入账，不重复充值。
func TestIntegrationDepositPolicyOldIntent(t *testing.T) {
	api := newTestAPI(t)
	p := pendingDepositPolicyTopup(t, api, "outsider", 100)
	api.call("admin", "PATCH", "/admin/financial-settings", FinancialSettings{PersonalPlatformDepositCents: 500}, 200, nil)
	body := map[string]any{"purpose": "deposit", "amount_cents": p.AmountCents, "request_id": p.RequestID}
	for i := 0; i < 2; i++ {
		api.call("outsider", "POST", "/wallet/topups", body, 200, nil)
		api.call("outsider", "POST", "/wallet/topups/"+p.ID+"/pay", map[string]any{}, 200, nil)
	}
	stored, e := get[WalletTopup](t.Context(), api.App.DB, "wallet_topups", p.ID, false)
	if e != nil || stored.AmountCents != 100 || stored.Status != "paid" || stored.TransactionID != "DEV"+p.ID {
		t.Fatalf("旧付款意图未恢复:%+v %v", stored, e)
	}
	if e = api.App.applyWalletTopup(t.Context(), stored, wxTransaction{TransactionID: stored.TransactionID}, true); e != nil {
		t.Fatal(e)
	}
	var summary map[string]any
	api.call("outsider", "GET", "/wallet", nil, 200, &summary)
	if summary["deposit_cents"] != float64(100) || summary["deposit_shortfall_cents"] != float64(400) || summary["deposit_qualified"] != false {
		t.Fatalf("旧付款重复入账或误认证:%v", summary)
	}
	api.call("outsider", "POST", "/wallet/topups", map[string]any{"purpose": "deposit", "amount_cents": 100, "request_id": "policy-new-intent-under-min"}, 400, nil)
}

// TestIntegrationDepositPolicyClubCurrentThreshold 验证组织与个人门槛独立、草稿使用当前门槛及组织不足时按差额补缴。
func TestIntegrationDepositPolicyClubCurrentThreshold(t *testing.T) {
	api := newTestAPI(t)
	settings := FinancialSettings{PersonalPlatformDepositCents: 999, ClubCreationDepositCents: 200}
	api.call("admin", "PATCH", "/admin/financial-settings", settings, 200, nil)
	var draft Club
	api.call("outsider", "POST", "/clubs", map[string]any{"name": "最新门槛补缴俱乐部", "description": "验证历史创建快照不阻挡当前最低押金规则", "game_ids": []string{"delta"}}, 200, &draft)
	settings.ClubCreationDepositCents = 100
	api.call("admin", "PATCH", "/admin/financial-settings", settings, 200, nil)
	api.call("outsider", "POST", "/wallet/topups", map[string]any{"purpose": "club_platform", "club_id": draft.ID, "amount_cents": 99, "request_id": "policy-club-under-min"}, 400, nil)
	api.call("outsider", "POST", "/wallet/topups", map[string]any{"purpose": "club_platform", "club_id": draft.ID, "amount_cents": 100, "request_id": "policy-club-exact-min"}, 200, nil)
	current, e := get[Club](t.Context(), api.App.DB, "clubs", draft.ID, false)
	if e != nil || current.Status != "pending" || current.CreationDepositRequiredCents != 200 {
		t.Fatalf("当前门槛未生效或历史快照被改写:%+v %v", current, e)
	}
	api.call("admin", "PATCH", "/admin/clubs/"+draft.ID, map[string]any{"status": "active", "verified": true}, 200, nil)
	settings.ClubCreationDepositCents = 151
	api.call("admin", "PATCH", "/admin/financial-settings", settings, 200, nil)
	var deposits struct {
		Platform map[string]any `json:"platform"`
	}
	api.call("outsider", "GET", "/clubs/"+draft.ID+"/deposits", nil, 200, &deposits)
	if deposits.Platform["amount_cents"] != float64(100) || deposits.Platform["required_cents"] != float64(151) || deposits.Platform["shortfall_cents"] != float64(51) || deposits.Platform["qualified"] != false {
		t.Fatalf("组织补缴金额错误:%v", deposits.Platform)
	}
	var response struct {
		Club Club `json:"club"`
	}
	api.call("", "GET", "/clubs/"+draft.ID, nil, 200, &response)
	public := response.Club
	if public.PlatformDepositCents != 0 || public.PlatformDepositRequiredCents != 151 || public.PlatformDepositShortfallCents != 51 || public.DepositReady {
		t.Fatalf("组织未足额仍认证:%+v", public)
	}
	api.call("outsider", "POST", "/wallet/topups", map[string]any{"purpose": "club_platform", "club_id": draft.ID, "amount_cents": 50, "request_id": "policy-club-difference-under"}, 400, nil)
	api.call("outsider", "POST", "/wallet/topups", map[string]any{"purpose": "club_platform", "club_id": draft.ID, "amount_cents": 51, "request_id": "policy-club-difference-exact"}, 200, nil)
	api.call("", "GET", "/clubs/"+draft.ID, nil, 200, &response)
	public = response.Club
	if public.PlatformDepositCents != 151 || !public.DepositReady || public.PlatformDepositShortfallCents != 0 {
		t.Fatalf("组织足额未恢复:%+v", public)
	}
	if wallet := testWallet(api, "outsider"); wallet.DepositCents != 0 || wallet.RechargeCents != 0 || wallet.AvailableCents != 0 {
		t.Fatalf("组织押金混入个人余额:%+v", wallet)
	}
}

// TestIntegrationDepositPolicyConcurrentTopup 验证同一补缴业务号并发提交只增加一次押金，门槛检查不阻断已保存原单。
func TestIntegrationDepositPolicyConcurrentTopup(t *testing.T) {
	api := newTestAPI(t)
	testTopup(api, "outsider", "deposit", "policy-concurrent-original", 100)
	api.call("admin", "PATCH", "/admin/financial-settings", FinancialSettings{PersonalPlatformDepositCents: 101}, 200, nil)
	var wg sync.WaitGroup
	issues := make(chan string, 12)
	for range 12 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			status, raw, e := api.request("outsider", "POST", "/wallet/topups", map[string]any{"purpose": "deposit", "amount_cents": 1, "request_id": "policy-concurrent-one-cent"})
			if e != nil || status != 200 {
				issues <- fmt.Sprintf("并发补缴失败:%d %s %v", status, raw, e)
			}
		}()
	}
	wg.Wait()
	close(issues)
	for issue := range issues {
		t.Error(issue)
	}
	if wallet := testWallet(api, "outsider"); wallet.DepositCents != 101 || wallet.AvailableCents != 0 || wallet.RechargeCents != 0 {
		t.Fatalf("同单并发重复入账:%+v", wallet)
	}
}

// TestIntegrationDepositPolicyPendingRefundAndWXCallback 验证提额后迟到真实通知保留入账，退还中资金不计足额，降低及清零门槛实时生效。
func TestIntegrationDepositPolicyPendingRefundAndWXCallback(t *testing.T) {
	api, c, key := productionWalletAPI(t)
	club, e := get[Club](t.Context(), api.App.DB, "clubs", "shanhai", false)
	if e != nil {
		t.Fatal(e)
	}
	club.ID, club.OwnerID, club.Name = "policy-refund-club", "outsider", "押金待退门槛俱乐部"
	if e = put(t.Context(), api.App.DB, "clubs", club.ID, club); e != nil {
		t.Fatal(e)
	}
	personal := pendingWXTopup(t, api, "worker", "deposit", 200)
	organization := pendingWXTopup(t, api, "outsider", "club_platform", 200)
	organization.AccountID, organization.ClubID = clubDepositAccount(club.ID), club.ID
	if e = put(t.Context(), api.App.DB, "wallet_topups", organization.ID, organization); e != nil {
		t.Fatal(e)
	}
	settings := FinancialSettings{PersonalPlatformDepositCents: 300, ClubCreationDepositCents: 300}
	api.call("admin", "PATCH", "/admin/financial-settings", settings, 200, nil)
	for _, p := range []WalletTopup{personal, organization} {
		for range 2 {
			if status := deliverWalletNotice(t, api, c, key, "TRANSACTION.SUCCESS", wxTopupTransaction(c, p), ""); status != 204 {
				t.Fatalf("新门槛拒绝已发生实付或重复通知:%d", status)
			}
		}
	}
	var summary map[string]any
	api.call("worker", "GET", "/wallet", nil, 200, &summary)
	if summary["deposit_cents"] != float64(200) || summary["deposit_shortfall_cents"] != float64(100) || summary["deposit_qualified"] != false {
		t.Fatalf("迟到付款没有保留或误认证:%v", summary)
	}
	settings.PersonalPlatformDepositCents, settings.ClubCreationDepositCents = 100, 100
	api.call("admin", "PATCH", "/admin/financial-settings", settings, 200, nil)
	api.App.HTTPClient = &http.Client{Transport: paymentTransport(func(r *http.Request) (*http.Response, error) {
		return nil, errors.New("受控测试：退款请求结果未知，保留原单冻结")
	})}
	api.call("worker", "POST", "/wallet/deposit-refunds", map[string]any{"amount_cents": 150, "request_id": "policy-personal-refund-pending"}, 502, nil)
	api.call("outsider", "POST", "/wallet/deposit-refunds", map[string]any{"scope": "club_platform", "club_id": club.ID, "amount_cents": 150, "request_id": "policy-club-refund-pending"}, 502, nil)
	api.call("worker", "GET", "/wallet", nil, 200, &summary)
	if summary["deposit_cents"] != float64(50) || summary["deposit_refunding_cents"] != float64(150) || summary["deposit_shortfall_cents"] != float64(50) || summary["deposit_qualified"] != false {
		t.Fatalf("个人将退款冻结额计入有效押金:%v", summary)
	}
	var service Service
	api.call("", "GET", "/services/delta-service-1", nil, 200, &service)
	if service.ProviderDepositCents != 0 {
		t.Fatal("退还中个人押金仍显示足额认证")
	}
	var response struct {
		Club Club `json:"club"`
	}
	api.call("", "GET", "/clubs/"+club.ID, nil, 200, &response)
	if response.Club.PlatformDepositCents != 0 || response.Club.PlatformDepositShortfallCents != 50 || response.Club.DepositReady {
		t.Fatalf("组织将退款冻结额计入认证:%+v", response.Club)
	}
	var deposits struct {
		Platform map[string]any `json:"platform"`
	}
	api.call("outsider", "GET", "/clubs/"+club.ID+"/deposits", nil, 200, &deposits)
	if deposits.Platform["amount_cents"] != float64(50) || deposits.Platform["refunding_cents"] != float64(150) || deposits.Platform["shortfall_cents"] != float64(50) {
		t.Fatalf("组织有效额和补缴错误:%v", deposits.Platform)
	}
	for _, minimum := range []int64{50, 0} {
		settings.PersonalPlatformDepositCents, settings.ClubCreationDepositCents = minimum, minimum
		api.call("admin", "PATCH", "/admin/financial-settings", settings, 200, nil)
		api.call("worker", "GET", "/wallet", nil, 200, &summary)
		if summary["deposit_shortfall_cents"] != float64(0) || summary["deposit_qualified"] != true || summary["deposit_cents"] != float64(50) || summary["deposit_refunding_cents"] != float64(150) {
			t.Fatalf("降到%d分错误改变原资金或未恢复足额:%v", minimum, summary)
		}
		api.call("", "GET", "/services/delta-service-1", nil, 200, &service)
		if service.ProviderDepositCents != 50 {
			t.Fatal("降门槛后个人有效押金未实时恢复")
		}
		api.call("", "GET", "/clubs/"+club.ID, nil, 200, &response)
		if response.Club.PlatformDepositCents != 50 || response.Club.PlatformDepositShortfallCents != 0 {
			t.Fatalf("降门槛后组织有效押金未实时恢复:%+v", response.Club)
		}
	}
}

// TestIntegrationDepositPolicyValidation 验证两档最低押金金额边界和公开规则不含私密商户信息。
func TestIntegrationDepositPolicyValidation(t *testing.T) {
	api := newTestAPI(t)
	for _, settings := range []FinancialSettings{{PersonalPlatformDepositCents: -1}, {PersonalPlatformDepositCents: 10000001}, {ClubCreationDepositCents: -1}, {ClubCreationDepositCents: 10000001}} {
		api.call("admin", "PATCH", "/admin/financial-settings", settings, 400, nil)
	}
	settings := FinancialSettings{PersonalPlatformDepositCents: 10000000, ClubCreationDepositCents: 1}
	api.call("admin", "PATCH", "/admin/financial-settings", settings, 200, nil)
	var got FinancialSettings
	api.call("", "GET", "/financial-rules", nil, 200, &got)
	if !reflect.DeepEqual(got, settings) {
		t.Fatalf("两档公开最低押金不一致:%+v", got)
	}
	api.call("admin", "PATCH", "/admin/financial-settings", map[string]any{"free_platform_rate_bps": 100, "club_platform_rate_bps": 200, "club_creation_deposit_cents": 50}, 200, nil)
	api.call("admin", "GET", "/admin/financial-settings", nil, 200, &got)
	if got.PersonalPlatformDepositCents != settings.PersonalPlatformDepositCents || got.ClubCreationDepositCents != 50 || got.FreePlatformRateBPS != 100 {
		t.Fatalf("旧客户端漏传新增字段清零了最低押金:%+v", got)
	}
	api.call("admin", "PATCH", "/admin/financial-settings", map[string]any{"personal_platform_deposit_cents": 0, "club_creation_deposit_cents": 50}, 200, nil)
	api.call("admin", "GET", "/admin/financial-settings", nil, 200, &got)
	if got.PersonalPlatformDepositCents != 0 {
		t.Fatalf("明确配置零未清除最低押金:%+v", got)
	}
}
