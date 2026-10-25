// Package app 用确定性数据库行锁验证已入场管理请求在撤权或冻结提交后不能继续修改用户、发消息或发布运营内容。
package app

import (
	"crypto/rsa"
	"errors"
	"net/http"
	"sync/atomic"
	"testing"
	"time"
)

// managementWaitForBlocker 等待实际HTTP处理器被本测试持有的锁阻塞，避免以睡眠猜测授权时序。
func managementWaitForBlocker(t *testing.T, api *testAPI, blocker uint32) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		var waiting bool
		if err := api.App.DB.QueryRow(t.Context(), "SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE $1::integer=ANY(pg_blocking_pids(pid)))", int(blocker)).Scan(&waiting); err != nil {
			t.Fatal(err)
		}
		if waiting {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("管理请求未进入预期数据库锁等待")
}

// managementWithdrawalFixture 只在隔离schema建立可核验来源的正式协议提现，测试传输层从不访问真实微信。
func managementWithdrawalFixture(t *testing.T) (*testAPI, wxPayConfig, *rsa.PrivateKey, WalletWithdrawal) {
	t.Helper()
	api, config, key := productionWalletAPI(t)
	testIncome(api, "worker", 1000)
	settings := WalletSettings{WithdrawalEnabled: true, TransferSceneID: "test-approved-scene", TransferNotifyURL: "https://example.com/api/payments/wechat/transfer-notify", TransferRemark: "服务收入提现", TransferSceneReportInfos: []WalletSceneReport{{InfoType: "岗位类型", InfoContent: "隔离测试服务收入"}}}
	api.call("admin", "PATCH", "/admin/wallet-settings", settings, 200, nil)
	var withdrawal WalletWithdrawal
	api.call("worker", "POST", "/wallet/withdrawals", map[string]any{"amount_cents": 400, "request_id": "management-revocation-withdrawal"}, 200, &withdrawal)
	stored, err := get[WalletWithdrawal](t.Context(), api.App.DB, "wallet_withdrawals", withdrawal.ID, false)
	if err != nil {
		t.Fatal(err)
	}
	api.call("admin", "PATCH", "/admin/users/player/deputy", map[string]bool{"enabled": true}, 200, nil)
	return api, config, key, stored
}

// TestIntegrationWithdrawalActorRevokedDuringWait 验证提现审批等待钱包锁时完成撤权，不出网、不拒绝、不改变冻结款，合法审批仍可继续。
func TestIntegrationWithdrawalActorRevokedDuringWait(t *testing.T) {
	for _, action := range []string{"execute", "reject"} {
		t.Run(action, func(t *testing.T) {
			api, config, key, withdrawal := managementWithdrawalFixture(t)
			var requests atomic.Int32
			api.App.HTTPClient = &http.Client{Transport: paymentTransport(func(r *http.Request) (*http.Response, error) {
				requests.Add(1)
				result := wxTransfer{OutBillNo: withdrawal.OutBillNo, TransferBillNo: "AUDIT-VALID-TRANSFER", State: "SUCCESS", MerchantID: config.MerchantID, AppID: config.AppID, OpenID: withdrawal.PayOpenID, AmountCents: withdrawal.AmountCents}
				return signedAdjustmentResponse(t, config, key, result), nil
			})}
			blocker, err := api.App.DB.Begin(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			defer blocker.Rollback(t.Context())
			if err = lockWalletUsers(t.Context(), blocker, withdrawal.UserID, withdrawalAccount(withdrawal)); err != nil {
				t.Fatal(err)
			}
			body := map[string]any{}
			if action == "reject" {
				body["reason"] = "撤权后不应拒绝"
			}
			finished, errCh := make(chan int, 1), make(chan error, 1)
			go func() {
				code, _, err := api.request("player", "POST", "/admin/wallet/withdrawals/"+withdrawal.ID+"/"+action, body)
				finished <- code
				errCh <- err
			}()
			managementWaitForBlocker(t, api, blocker.Conn().PgConn().PID())
			api.call("admin", "PATCH", "/admin/users/player/deputy", map[string]bool{"enabled": false}, 200, nil)
			if err = blocker.Rollback(t.Context()); err != nil {
				t.Fatal(err)
			}
			code := <-finished
			if err = <-errCh; err != nil || code != 403 || requests.Load() != 0 {
				t.Fatalf("撤权审批仍出款或拒绝：状态=%d 请求=%d 错误=%v", code, requests.Load(), err)
			}
			stored, err := get[WalletWithdrawal](t.Context(), api.App.DB, "wallet_withdrawals", withdrawal.ID, false)
			wallet := testWallet(api, "worker")
			if err != nil || stored.Status != "pending_review" || wallet.AvailableCents != 600 || wallet.WithdrawalFrozenCents != 400 {
				t.Fatal("无权审批改变了提现状态或冻结金额", stored.Status, wallet, err)
			}
			api.call("admin", "POST", "/admin/wallet/withdrawals/"+withdrawal.ID+"/"+action, body, 200, nil)
			wallet = testWallet(api, "worker")
			if action == "execute" && (wallet.AvailableCents != 600 || wallet.WithdrawalFrozenCents != 0 || requests.Load() != 2) || action == "reject" && (wallet.AvailableCents != 1000 || wallet.WithdrawalFrozenCents != 0 || requests.Load() != 0) {
				t.Fatal("有效管理员不能继续原提现审批", wallet, requests.Load())
			}
		})
	}
}

// TestIntegrationWithdrawalRecoveryAfterApproverRevoked 验证已发送后超时仍保持冻结，撤权后收款人可查询原单并幂等恢复。
func TestIntegrationWithdrawalRecoveryAfterApproverRevoked(t *testing.T) {
	api, config, key, withdrawal := managementWithdrawalFixture(t)
	var posts, queries atomic.Int32
	api.App.HTTPClient = &http.Client{Transport: paymentTransport(func(r *http.Request) (*http.Response, error) {
		if r.Method == "POST" {
			posts.Add(1)
			return nil, errors.New("隔离测试：微信受理后响应超时")
		}
		queries.Add(1)
		result := wxTransfer{OutBillNo: withdrawal.OutBillNo, TransferBillNo: "AUDIT-RECOVERED-TRANSFER", State: "SUCCESS", MerchantID: config.MerchantID, AppID: config.AppID, OpenID: withdrawal.PayOpenID, AmountCents: withdrawal.AmountCents}
		return signedAdjustmentResponse(t, config, key, result), nil
	})}
	api.call("player", "POST", "/admin/wallet/withdrawals/"+withdrawal.ID+"/execute", map[string]any{}, 502, nil)
	if wallet := testWallet(api, "worker"); wallet.WithdrawalFrozenCents != 400 || posts.Load() != 1 {
		t.Fatal("已发送未知结果未保留原冻结", wallet, posts.Load())
	}
	api.call("admin", "PATCH", "/admin/users/player/deputy", map[string]bool{"enabled": false}, 200, nil)
	for index := 0; index < 2; index++ {
		api.call("worker", "POST", "/wallet/withdrawals/"+withdrawal.ID+"/sync", map[string]any{}, 200, nil)
	}
	if wallet := testWallet(api, "worker"); wallet.AvailableCents != 600 || wallet.WithdrawalFrozenCents != 0 || posts.Load() != 1 || queries.Load() != 1 {
		t.Fatal("撤权后原单恢复失败、重发或重复扣除", wallet, posts.Load(), queries.Load())
	}
}

// TestIntegrationManagementActorRevokedDuringTargetWait 验证旧冻结、资料与消息请求等待目标行期间，真实根接口撤权后不会继续写。
func TestIntegrationManagementActorRevokedDuringTargetWait(t *testing.T) {
	for _, sample := range []struct {
		name, method, suffix string
		body                 map[string]any
	}{{"freeze", "PATCH", "", map[string]any{"status": "frozen"}}, {"profile", "PATCH", "/profile", map[string]any{"nickname": "不应保存的昵称"}}, {"message", "POST", "/messages", map[string]any{"title": "不应发送的通知", "body": "撤权后不得投递"}}} {
		t.Run(sample.name, func(t *testing.T) {
			api := newTestAPI(t)
			api.call("admin", "PATCH", "/admin/users/player/deputy", map[string]bool{"enabled": true}, 200, nil)
			target := User{ID: "aaa-management-target", Nickname: "原始玩家", Role: "user", Status: "active", CreatedAt: now()}
			if err := put(t.Context(), api.App.DB, "users", target.ID, target); err != nil {
				t.Fatal(err)
			}
			blocker, err := api.App.DB.Begin(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			defer blocker.Rollback(t.Context())
			if _, err = get[User](t.Context(), blocker, "users", target.ID, true); err != nil {
				t.Fatal(err)
			}
			type response struct {
				code int
				err  error
			}
			finished := make(chan response, 1)
			go func() {
				code, _, err := api.request("player", sample.method, "/admin/users/"+target.ID+sample.suffix, sample.body)
				finished <- response{code, err}
			}()
			managementWaitForBlocker(t, api, blocker.Conn().PgConn().PID())
			api.call("admin", "PATCH", "/admin/users/player/deputy", map[string]bool{"enabled": false}, 200, nil)
			if err = blocker.Rollback(t.Context()); err != nil {
				t.Fatal(err)
			}
			result := <-finished
			if result.err != nil || result.code != 403 {
				t.Fatalf("撤权后的在途请求仍可写入：状态=%d 错误=%v", result.code, result.err)
			}
			current, err := get[User](t.Context(), api.App.DB, "users", target.ID, false)
			if err != nil || current.Nickname != target.Nickname || current.Status != "active" {
				t.Fatal("撤权请求修改了目标", current, err)
			}
			var messages int
			if err = api.App.DB.QueryRow(t.Context(), "SELECT count(*) FROM inbox WHERE data->>'user_id'=$1", target.ID).Scan(&messages); err != nil || messages != 0 {
				t.Fatal("撤权请求仍发送了消息", messages, err)
			}
		})
	}
}

// TestIntegrationPublicContentActorRevokedDuringWait 验证运营配置等待自己的串行锁时完成真实撤权，旧权限不能发布。
func TestIntegrationPublicContentActorRevokedDuringWait(t *testing.T) {
	api := newTestAPI(t)
	api.call("admin", "PATCH", "/admin/users/player/deputy", map[string]bool{"enabled": true}, 200, nil)
	blocker, err := api.App.DB.Begin(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	defer blocker.Rollback(t.Context())
	if _, err = blocker.Exec(t.Context(), "SELECT pg_advisory_xact_lock(8173081)"); err != nil {
		t.Fatal(err)
	}
	finished := make(chan int, 1)
	errCh := make(chan error, 1)
	go func() {
		code, _, err := api.request("player", "PATCH", "/admin/public-content", map[string]any{"revision": 0, "service_notice": "撤权后不应发布"})
		finished <- code
		errCh <- err
	}()
	managementWaitForBlocker(t, api, blocker.Conn().PgConn().PID())
	api.call("admin", "PATCH", "/admin/users/player/deputy", map[string]bool{"enabled": false}, 200, nil)
	if err = blocker.Rollback(t.Context()); err != nil {
		t.Fatal(err)
	}
	code := <-finished
	if err = <-errCh; err != nil || code != 403 {
		t.Fatalf("撤权后公共配置仍保存：状态=%d 错误=%v", code, err)
	}
	var cfg PublicContentSettings
	api.call("admin", "GET", "/admin/public-content", nil, 200, &cfg)
	if cfg.Revision != 0 || cfg.ServiceNotice != "" {
		t.Fatal("无权运营保存改变了配置")
	}
}

// TestIntegrationArticleActorFrozenDuringWait 验证文章创建在操作者行锁后重新读取状态，冻结后不能发布或生成审核任务。
func TestIntegrationArticleActorFrozenDuringWait(t *testing.T) {
	api := newTestAPI(t)
	api.call("admin", "PATCH", "/admin/users/player/deputy", map[string]bool{"enabled": true}, 200, nil)
	blocker, err := api.App.DB.Begin(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	defer blocker.Rollback(t.Context())
	actor, err := get[User](t.Context(), blocker, "users", "player", true)
	if err != nil {
		t.Fatal(err)
	}
	finished := make(chan int, 1)
	errCh := make(chan error, 1)
	go func() {
		code, _, err := api.request("player", "POST", "/admin/articles", map[string]any{"title": "不应发表文章", "summary": "", "body_markdown": "正常文字正文", "cover_url": "", "status": "published"})
		finished <- code
		errCh <- err
	}()
	managementWaitForBlocker(t, api, blocker.Conn().PgConn().PID())
	actor.Status = "frozen"
	if err = putRaw(t.Context(), blocker, "users", actor.ID, actor); err != nil {
		t.Fatal(err)
	}
	if err = blocker.Commit(t.Context()); err != nil {
		t.Fatal(err)
	}
	code := <-finished
	if err = <-errCh; err != nil || code != 403 {
		t.Fatalf("冻结后文章仍发布：状态=%d 错误=%v", code, err)
	}
	var articles, tasks int
	if err = api.App.DB.QueryRow(t.Context(), "SELECT count(*) FROM articles").Scan(&articles); err != nil {
		t.Fatal(err)
	}
	if err = api.App.DB.QueryRow(t.Context(), "SELECT count(*) FROM moderation_tasks WHERE data->>'target_type'='articles'").Scan(&tasks); err != nil || articles != 0 || tasks != 0 {
		t.Fatal("冻结请求产生文章或审核任务", articles, tasks, err)
	}
}
