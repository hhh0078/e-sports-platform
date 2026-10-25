// Package app 在隔离真实数据库中验证副管理员授权、管理分页、组织资金隔离及原负责人实名收款链路。
package app

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha1"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"
)

// TestManagementRoleCompatibility 验证历史根管理员保留权限，普通记录不能携带伪造管理标志。
func TestManagementRoleCompatibility(t *testing.T) {
	for _, sample := range []struct{ raw, role string }{{`{"role":"admin"}`, "root"}, {`{"role":"admin","platform_admin_role":"deputy"}`, "deputy"}, {`{"role":"user","platform_admin_role":"root"}`, ""}} {
		var u User
		if err := json.Unmarshal([]byte(sample.raw), &u); err != nil {
			t.Fatal(err)
		}
		if u.PlatformAdminRole != sample.role {
			t.Fatalf("角色兼容错误:%+v", u)
		}
	}
	deputy := User{ID: "owner", Role: "admin", PlatformAdminRole: "deputy", Status: "active"}
	if canManageClubAdmins(deputy, Club{OwnerID: "owner"}) {
		t.Fatal("平台副管理员不能借owner身份授撤管理角色")
	}
}

// managementFixtureMember 只在当前独立测试schema中建立真实组织成员关系。
func managementFixtureMember(t *testing.T, api *testAPI, club, user string) {
	t.Helper()
	v := ClubMember{ClubID: club, UserID: user, Status: "active", CreatedAt: now()}
	if err := put(t.Context(), api.App.DB, "members", club+":"+user, v); err != nil {
		t.Fatal(err)
	}
}

// TestIntegrationPlatformUserManagement 覆盖即时授权撤权、隐藏身份、手机号搜索、资料和定向消息与分页。
func TestIntegrationPlatformUserManagement(t *testing.T) {
	api := newTestAPI(t)
	for _, actor := range []string{"", "worker", "clubowner", "outsider"} {
		code := 403
		if actor == "" {
			code = 401
		}
		api.call(actor, "GET", "/admin/users/worker", nil, code, nil)
	}
	var deputy User
	api.call("admin", "PATCH", "/admin/users/player/deputy", map[string]bool{"enabled": true}, 200, &deputy)
	if deputy.Role != "admin" || deputy.PlatformAdminRole != "deputy" {
		t.Fatal(deputy)
	}
	api.call("player", "GET", "/admin/users/worker", nil, 200, nil)
	api.call("player", "PATCH", "/admin/users/worker/deputy", map[string]bool{"enabled": true}, 403, nil)
	api.call("player", "PATCH", "/admin/users/admin", map[string]string{"status": "frozen"}, 403, nil)
	api.call("player", "PATCH", "/admin/users/admin/profile", map[string]string{"nickname": "不能编辑根管理员"}, 403, nil)
	api.call("admin", "PATCH", "/admin/users/admin/deputy", map[string]bool{"enabled": false}, 403, nil)
	for _, field := range []string{"role", "platform_admin_role", "phone_number", "openid", "available_cents"} {
		api.call("player", "PATCH", "/admin/users/worker/profile", map[string]any{field: "forbidden"}, 400, nil)
	}
	api.call("worker", "POST", "/me/phone/dev", map[string]string{"phone_number": "13912345678"}, 200, nil)
	var users []map[string]any
	api.call("player", "GET", "/admin/users?q=1391234", nil, 200, &users)
	if len(users) != 1 || users[0]["id"] != "worker" {
		t.Fatalf("手机号先搜再分页错误:%v", users)
	}
	for _, key := range []string{"bio", "openid", "phone_number"} {
		if _, ok := users[0][key]; ok {
			t.Fatal("列表泄露字段", key)
		}
	}
	before, _ := get[User](t.Context(), api.App.DB, "users", "worker", false)
	avatar := api.uploadCommunityImage("player", "avatar", "", communityPNG(t, 128, 128), 200)
	var edited User
	api.call("player", "PATCH", "/admin/users/worker/profile", map[string]string{"nickname": "后台编辑玩家", "bio": "管理员更新展示资料", "avatar": avatar}, 200, &edited)
	if edited.Avatar != avatar || edited.Role != before.Role || edited.PhoneMasked != before.PhoneMasked || edited.PlatformAdminRole != "" {
		t.Fatalf("展示编辑改变身份:%+v", edited)
	}
	api.call("player", "POST", "/admin/users/worker/messages", map[string]string{"title": "管理通知", "body": "仅向所选用户发送站内消息"}, 200, nil)
	api.call("player", "POST", "/admin/users/worker/messages", map[string]string{"title": "通知", "body": "正文", "recipient_id": "outsider"}, 400, nil)
	var inbox struct {
		Items []InboxMessage `json:"items"`
	}
	api.call("worker", "GET", "/messages", nil, 200, &inbox)
	found := false
	for _, item := range inbox.Items {
		if item.Title == "管理通知" {
			found = true
		}
	}
	if !found {
		t.Fatal("定向通知未投递")
	}
	testTopup(api, "worker", "balance", "management-recharge-only", 1234)
	var wallet map[string]any
	api.call("player", "GET", "/admin/users/worker/wallet", nil, 200, &wallet)
	if wallet["recharge_cents"] != float64(1234) {
		t.Fatal(wallet)
	}
	var entries ManagementPage[WalletEntry]
	api.call("player", "GET", "/admin/users/worker/wallet/entries?limit=1", nil, 200, &entries)
	if entries.Total < 1 || len(entries.Items) != 1 {
		t.Fatal(entries)
	}
	var tasks ManagementPage[map[string]any]
	api.call("player", "GET", "/admin/users/worker/moderation?limit=1", nil, 200, &tasks)
	if tasks.Total < 2 || len(tasks.Items) != 1 || !tasks.HasMore {
		t.Fatal(tasks)
	}
	api.call("admin", "PATCH", "/admin/users/player/deputy", map[string]bool{"enabled": false}, 200, nil)
	api.call("player", "GET", "/admin/users/worker", nil, 403, nil)
}

// TestIntegrationClubDeputyScope 验证多组织授权、成员状态撤权及副管理员不能间接移除其它管理员。
func TestIntegrationClubDeputyScope(t *testing.T) {
	api := newTestAPI(t)
	managementFixtureMember(t, api, "shanhai", "player")
	managementFixtureMember(t, api, "wild", "worker")
	api.call("clubowner", "PATCH", "/clubs/shanhai/deputies/worker", map[string]bool{"enabled": true}, 200, nil)
	api.call("worker", "GET", "/clubs/shanhai/members", nil, 200, nil)
	api.call("worker", "GET", "/clubs/wild/members", nil, 403, nil)
	api.call("worker", "PATCH", "/clubs/shanhai/deputies/player", map[string]bool{"enabled": true}, 403, nil)
	api.call("clubowner", "PATCH", "/clubs/shanhai/deputies/player", map[string]bool{"enabled": true}, 200, nil)
	api.call("worker", "POST", "/clubs/shanhai/members/player", map[string]string{"status": "removed", "review_note": "不能间接撤权"}, 403, nil)
	api.call("admin", "PATCH", "/admin/users/outsider/deputy", map[string]bool{"enabled": true}, 200, nil)
	api.call("outsider", "PATCH", "/clubs/shanhai/deputies/worker", map[string]bool{"enabled": false}, 403, nil)
	api.call("outsider", "PATCH", "/clubs/wild/deputies/worker", map[string]bool{"enabled": true}, 403, nil)
	for _, field := range []string{"owner_id", "role", "platform_admin_role"} {
		api.call("worker", "PATCH", "/clubs/shanhai", map[string]string{field: "outsider"}, 400, nil)
	}
	api.call("worker", "GET", "/clubs/shanhai/members/outsider", nil, 404, nil)
	api.call("worker", "POST", "/clubs/shanhai/members/player/messages", map[string]string{"title": "组织通知", "body": "该成员所属组织的通知"}, 200, nil)
	api.call("worker", "POST", "/clubs/wild/members/worker/messages", map[string]string{"title": "越界通知", "body": "不能发送"}, 403, nil)
	api.call("clubowner", "POST", "/clubs/shanhai/members/player", map[string]string{"status": "removed", "review_note": "负责人解除认证同时撤销管理授权"}, 200, nil)
	api.call("player", "GET", "/clubs/shanhai/members", nil, 403, nil)
	api.call("worker", "POST", "/clubs/shanhai/leave", map[string]any{}, 200, nil)
	api.call("worker", "GET", "/clubs/shanhai/members", nil, 403, nil)
	var record ClubDeputy
	v, err := get[ClubDeputy](t.Context(), api.App.DB, "club_deputies", "shanhai:worker", false)
	record = v
	if err != nil || record.Status != "revoked" {
		t.Fatal(record, err)
	}
	api.call("clubowner", "PATCH", "/clubs/shanhai/deputies/worker", map[string]bool{"enabled": true}, 409, nil)
}

// TestIntegrationClubMemberFinanceIsolation 验证本组织收入、调价与押金查询不会泄露个人充值、自由或其它组织收入。
func TestIntegrationClubMemberFinanceIsolation(t *testing.T) {
	api := newTestAPI(t)
	managementFixtureMember(t, api, "wild", "worker")
	api.call("clubowner", "PATCH", "/clubs/shanhai/deputies/worker", map[string]bool{"enabled": true}, 200, nil)
	service, err := get[Service](t.Context(), api.App.DB, "services", "delta-service-1", false)
	if err != nil {
		t.Fatal(err)
	}
	service.PriceCents = 1000
	if err = put(t.Context(), api.App.DB, "services", service.ID, service); err != nil {
		t.Fatal(err)
	}
	own := financialOrder(api, "player", service.ID, "dev", "")
	for _, club := range []string{"", "wild"} {
		copy := service
		copy.ID = "scope-service-" + club
		copy.ClubID = club
		if err = put(t.Context(), api.App.DB, "services", copy.ID, copy); err != nil {
			t.Fatal(err)
		}
		financialOrder(api, "player", copy.ID, "dev", "")
	}
	testIncome(api, "worker", 9900)
	testTopup(api, "worker", "balance", "private-recharge-excluded", 4500)
	api.call("worker", "POST", "/wallet/topups", map[string]any{"purpose": "member_club", "club_id": "shanhai", "amount_cents": 275, "request_id": "member-scope-deposit-1"}, 200, nil)
	api.call("worker", "POST", "/wallet/topups", map[string]any{"purpose": "member_club", "club_id": "wild", "amount_cents": 375, "request_id": "member-scope-deposit-2"}, 200, nil)
	var detail struct {
		Finance ScopedMemberFinance `json:"scope_finance"`
	}
	api.call("clubowner", "GET", "/clubs/shanhai/members/worker", nil, 200, &detail)
	if detail.Finance.OngoingCents != 1000 || detail.Finance.ClubDepositCents != 275 || detail.Finance.SettledIncomeCents != 0 {
		t.Fatal(detail)
	}
	matureFinancialOrder(api, own, "worker", "player")
	testWallet(api, "worker")
	api.call("clubowner", "GET", "/clubs/shanhai/members/worker", nil, 200, &detail)
	if detail.Finance.SettledIncomeCents != 1000 || detail.Finance.OngoingCents != 0 {
		t.Fatal(detail)
	}
	var rows ManagementPage[map[string]any]
	api.call("clubowner", "GET", "/clubs/shanhai/members/worker/finance/entries?limit=1", nil, 200, &rows)
	if rows.Total != 3 || len(rows.Items) != 1 || !rows.HasMore {
		t.Fatal(rows)
	}
	for offset := 0; offset < rows.Total; offset++ {
		var page ManagementPage[map[string]any]
		api.call("worker", "GET", fmt.Sprintf("/clubs/shanhai/members/worker/finance/entries?limit=1&offset=%d", offset), nil, 200, &page)
		item := page.Items[0]
		if item["scope"] == "club_order_income" && item["order_id"] != own.ID {
			t.Fatal("泄露组织外收入", item)
		}
		if item["scope"] == "club_member_deposit" && item["amount_cents"] != float64(275) {
			t.Fatal("泄露其它托管押金", item)
		}
	}
	var orders ManagementPage[Order]
	api.call("clubowner", "GET", "/clubs/shanhai/members/worker/orders?limit=1", nil, 200, &orders)
	if orders.Total != 1 || orders.Items[0].ID != own.ID || orders.Items[0].PayOpenID != "" {
		t.Fatal(orders)
	}
	for i, club := range []string{"shanhai", "wild", ""} {
		task := ModerationTask{ID: fmt.Sprintf("scoped-task-%d", i), OwnerID: "worker", ActorID: "worker", ClubID: club, TargetType: "services", TargetID: fmt.Sprintf("service-%d", i), FieldLabel: "服务说明", Kind: "text", Status: "manual_pending", Preview: "原始内容不能泄露", CreatedAt: now()}
		if err = put(t.Context(), api.App.DB, "moderation_tasks", task.ID, task); err != nil {
			t.Fatal(err)
		}
	}
	var reviews ManagementPage[map[string]any]
	api.call("clubowner", "GET", "/clubs/shanhai/members/worker/moderation?limit=1", nil, 200, &reviews)
	if reviews.Total < 1 || reviews.Items[0]["id"] != "scoped-task-0" || reviews.Items[0]["preview"] != nil {
		t.Fatal(reviews)
	}
	api.call("clubowner", "GET", "/clubs/shanhai/members/worker/moderation?limit=100", nil, 200, &reviews)
	for _, review := range reviews.Items {
		if review["id"] == "scoped-task-1" || review["id"] == "scoped-task-2" || review["preview"] != nil {
			t.Fatal("审核查询泄露其它组织或原证据", review)
		}
	}
	api.call("outsider", "GET", "/clubs/shanhai/members/worker/finance/entries", nil, 403, nil)
}

// TestIntegrationClubDeputyWXRecipient 用真实签名微信协议替身核验申请人、原负责人openid、实名密文及确认参数隔离。
func TestIntegrationClubDeputyWXRecipient(t *testing.T) {
	api, c, key := productionWalletAPI(t)
	ctx := t.Context()
	identity, _ := get[map[string]string](ctx, api.App.DB, "identities", "clubowner", false)
	identity["openid"] = "wallet-openid-original-owner"
	identity["phone_source"] = "wechat"
	if err := put(ctx, api.App.DB, "identities", "clubowner", identity); err != nil {
		t.Fatal(err)
	}
	api.call("clubowner", "PATCH", "/clubs/shanhai/deputies/worker", map[string]bool{"enabled": true}, 200, nil)
	club, _ := get[Club](ctx, api.App.DB, "clubs", "shanhai", false)
	club.CommissionRateBPS = 1000
	if err := put(ctx, api.App.DB, "clubs", club.ID, club); err != nil {
		t.Fatal(err)
	}
	settings := WalletSettings{WithdrawalEnabled: true, TransferSceneID: "test-approved-scene", TransferNotifyURL: "https://example.com/api/payments/wechat/transfer-notify", TransferRemark: "服务收入提现", TransferSceneReportInfos: []WalletSceneReport{{InfoType: "岗位类型", InfoContent: "测试俱乐部佣金"}}}
	api.call("admin", "PATCH", "/admin/wallet-settings", settings, 200, nil)
	var transfer wxTransfer
	posts := 0
	api.App.HTTPClient = &http.Client{Transport: paymentTransport(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path == "/v3/pay/transactions/jsapi" {
			return signedAdjustmentResponse(t, c, key, map[string]string{"prepay_id": "management-real-protocol"}), nil
		}
		if r.Method == "POST" {
			var body struct {
				OutBillNo string `json:"out_bill_no"`
				OpenID    string `json:"openid"`
				Amount    int64  `json:"transfer_amount"`
				Name      string `json:"user_name"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			if body.OpenID != "wallet-openid-original-owner" {
				t.Error("副管理员替换了收款openid")
			}
			encrypted, e := base64.StdEncoding.DecodeString(body.Name)
			if e != nil {
				t.Error(e)
			}
			name, e := rsa.DecryptOAEP(sha1.New(), rand.Reader, key, encrypted, nil)
			if e != nil || string(name) != "原负责人实名" {
				t.Error("实名未绑定负责人输入", e)
			}
			posts++
			transfer = wxTransfer{OutBillNo: body.OutBillNo, TransferBillNo: "SIGNEDOWNERTRANSFER", State: "WAIT_USER_CONFIRM", MerchantID: c.MerchantID, AppID: c.AppID, OpenID: body.OpenID, AmountCents: body.Amount, PackageInfo: "only-owner-confirmation"}
		}
		return signedAdjustmentResponse(t, c, key, transfer), nil
	})}
	order := paidWXAdjustmentOrder(t, api, c, key)
	api.action("worker", order.ID, "accept", "", 200)
	api.action("worker", order.ID, "start", "", 200)
	api.action("worker", order.ID, "deliver", "已完成俱乐部服务", 200)
	api.action("player", order.ID, "confirm", "", 200)
	// 使用数据库完整原单仅调整售后测试时间，不能把已脱敏的HTTP响应覆盖原支付凭证。
	paid, err := get[Order](ctx, api.App.DB, "orders", order.ID, false)
	if err != nil {
		t.Fatal(err)
	}
	paid.CompletedAt = time.Now().Add(-8 * 24 * time.Hour).Format(time.RFC3339Nano)
	if err = put(ctx, api.App.DB, "orders", paid.ID, paid); err != nil {
		t.Fatal(err)
	}
	api.call("clubowner", "GET", "/clubs/shanhai/wallet", nil, 200, nil)
	api.call("worker", "POST", "/clubs/shanhai/withdrawals", map[string]any{"amount_cents": 100, "request_id": "deputy-wx-spoofed-1", "user_id": "worker"}, 400, nil)
	var v WalletWithdrawal
	body := map[string]any{"amount_cents": 100, "request_id": "deputy-wx-owner-real-1", "user_name": "原负责人实名"}
	api.call("worker", "POST", "/clubs/shanhai/withdrawals", body, 200, &v)
	stored, err := get[WalletWithdrawal](ctx, api.App.DB, "wallet_withdrawals", v.ID, false)
	if err != nil || stored.UserID != "clubowner" || stored.RequestedBy != "worker" || stored.PayOpenID != "wallet-openid-original-owner" {
		t.Fatal(stored, err)
	}
	api.call("worker", "POST", "/clubs/shanhai/withdrawals", body, 200, nil)
	api.call("clubowner", "POST", "/clubs/shanhai/withdrawals", body, 409, nil)
	api.call("worker", "POST", "/wallet/withdrawals", map[string]any{"amount_cents": 100, "request_id": "deputy-direct-wallet-spoof", "account_id": clubWalletID("shanhai")}, 400, nil)
	club.OwnerID = "outsider"
	put(ctx, api.App.DB, "clubs", club.ID, club)
	api.call("admin", "POST", "/admin/wallet/withdrawals/"+v.ID+"/execute", map[string]any{}, 409, nil)
	if posts != 0 {
		t.Fatal("负责人变更后仍发起转账")
	}
	club.OwnerID = "clubowner"
	put(ctx, api.App.DB, "clubs", club.ID, club)
	api.call("admin", "POST", "/admin/wallet/withdrawals/"+v.ID+"/execute", map[string]any{}, 200, nil)
	if posts != 1 {
		t.Fatal(posts)
	}
	for _, actor := range []string{"worker", "clubowner"} {
		var history []map[string]any
		api.call(actor, "GET", "/clubs/shanhai/withdrawals", nil, 200, &history)
		if len(history) != 1 || (history[0]["confirmation"] != nil) != (actor == "clubowner") {
			t.Fatal(actor, history)
		}
		var response map[string]any
		api.call(actor, "POST", "/wallet/withdrawals/"+v.ID+"/sync", map[string]any{}, 200, &response)
		if (response["confirmation"] != nil) != (actor == "clubowner") {
			t.Fatal(actor, response)
		}
		for _, key := range []string{"pay_open_id", "encrypted_name", "package_info"} {
			if response[key] != nil {
				t.Fatal("泄露实名或确认包", key)
			}
		}
	}
	api.call("outsider", "POST", "/wallet/withdrawals/"+v.ID+"/sync", map[string]any{}, 403, nil)
	api.call("clubowner", "PATCH", "/clubs/shanhai/deputies/worker", map[string]bool{"enabled": false}, 200, nil)
	api.call("worker", "POST", "/wallet/withdrawals/"+v.ID+"/sync", map[string]any{}, 403, nil)
}
