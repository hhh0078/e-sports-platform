// Package app 验证一次性授权候选、事务通知、并发发送、失败恢复与验签回调；微信HTTP由受控替身提供。
package app

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// subscriptionTestSettings 构造一个启用新订单模板的完整配置，其余场景保持关闭。
func subscriptionTestSettings() SubscriptionSettings {
	s := defaultSubscriptionSettings()
	s.Enabled = true
	s.Scenes[0] = SubscriptionScene{Scene: "provider_order", Enabled: true, TemplateID: "test_template_new_order", Fields: map[string]string{"thing1": "title", "time2": "time"}}
	return s
}

// subscriptionTestNotice 提供不含用户原文或私密身份的固定业务提醒。
func subscriptionTestNotice(id string) DomainNotice {
	return DomainNotice{ID: id, Scene: "provider_order", Event: "order_paid", UserIDs: []string{"worker"}, Title: "收到新订单", Body: "有已付款订单需要处理", ActionPath: "/pages/orders/detail?id=test-order", Values: map[string]string{"title": "收到新订单"}}
}

// subscriptionTestReset 在独立schema重置本测试场景并绑定模拟官方身份，不触及开发数据。
func subscriptionTestReset(t *testing.T, api *testAPI) {
	t.Helper()
	for _, table := range []string{"subscription_deliveries", "subscription_authorizations", "subscription_preferences", "subscription_receipts", "subscription_callbacks", "inbox"} {
		if _, err := api.App.DB.Exec(t.Context(), "DELETE FROM "+table); err != nil {
			t.Fatal(err)
		}
	}
	api.App.Config.WechatAppID = "wx_subscription_test"
	api.App.Config.WechatSecret = "test-secret"
	if err := putRaw(t.Context(), api.App.DB, "orders", "test-order", Order{ID: "test-order", BuyerID: "player", ProviderID: "worker", PaymentStatus: "paid", Status: "open", RefundStatus: "none"}); err != nil {
		t.Fatal(err)
	}
	if err := putRaw(t.Context(), api.App.DB, "settings", "wechat_subscriptions", subscriptionTestSettings()); err != nil {
		t.Fatal(err)
	}
	if err := mergeIdentity(t.Context(), api.App.DB, "worker", map[string]string{"openid": "test-private-openid", "appid": api.App.Config.WechatAppID}); err != nil {
		t.Fatal(err)
	}
	iter := api.App.Redis.Scan(t.Context(), 0, api.App.redisKey("wechat-access:*"), 100).Iterator()
	for iter.Next(t.Context()) {
		api.App.Redis.Del(t.Context(), iter.Val())
	}
}

// subscriptionTestAuthorize 通过本人HTTP接口记录一次显式授权结果。
func subscriptionTestAuthorize(api *testAPI, id, status string) {
	api.call("worker", "POST", "/subscriptions/authorizations", map[string]any{"request_id": id, "results": []map[string]string{{"scene": "provider_order", "template_id": "test_template_new_order", "status": status}}}, 200, nil)
}

// subscriptionTestTransport 模拟官方token和发送接口并计数，不产生任何真实微信推送。
func subscriptionTestTransport(calls *atomic.Int32, handler func(*http.Request) (*http.Response, error)) *http.Client {
	return &http.Client{Transport: paymentTransport(func(r *http.Request) (*http.Response, error) {
		if r.URL.Scheme != "https" || r.URL.Host != "api.weixin.qq.com" {
			return nil, fmt.Errorf("非官方目标")
		}
		if r.URL.Path == "/cgi-bin/stable_token" {
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"access_token":"mock-only-token","expires_in":7200}`))}, nil
		}
		if r.URL.Path != "/cgi-bin/message/subscribe/send" {
			return nil, fmt.Errorf("意外接口")
		}
		calls.Add(1)
		return handler(r)
	})}
}

// subscriptionTestDelivery 读取单业务提醒的独立持久发送记录。
func subscriptionTestDelivery(t *testing.T, api *testAPI) subscriptionStoredDelivery {
	t.Helper()
	items, err := list[subscriptionStoredDelivery](t.Context(), api.App.DB, "subscription_deliveries", "ORDER BY id")
	if err != nil || len(items) != 1 {
		t.Fatalf("记录数量=%d err=%v", len(items), err)
	}
	return items[0]
}

// subscriptionTestDrain 处理单个已入队事件并断言没有后台错误。
func subscriptionTestDrain(t *testing.T, api *testAPI) {
	t.Helper()
	ok, err := api.App.processSubscriptionOne(t.Context())
	if err != nil || !ok {
		t.Fatalf("处理结果=%v err=%v", ok, err)
	}
}

// TestSubscriptionValidation 验证配置、字段长度、落地路径和重试边界的固定限制。
func TestSubscriptionValidation(t *testing.T) {
	s := subscriptionTestSettings()
	if err := validateSubscriptionSettings(s); err != nil {
		t.Fatal(err)
	}
	s.Scenes[0].Scene = "old_scene"
	if validateSubscriptionSettings(s) == nil {
		t.Fatal("未知场景应拒绝")
	}
	s = subscriptionTestSettings()
	s.Scenes[0].Fields["evil1"] = "title"
	if validateSubscriptionSettings(s) == nil {
		t.Fatal("未知字段应拒绝")
	}
	if value, err := subscriptionFieldValue("thing1", strings.Repeat("中", 30)); err != nil || len([]rune(value)) != 20 {
		t.Fatal("中文按rune截断")
	}
	if _, err := subscriptionFieldValue("amount1", "尚未结算"); err == nil {
		t.Fatal("不得编造金额")
	}
	if _, err := subscriptionFieldValue("character_string2", "恶意标题"); err == nil {
		t.Fatal("受限字符串应拒绝")
	}
	if subscriptionPage("https://evil.test/a") != "pages/account/messages/index" {
		t.Fatal("外链不得作为落地")
	}
	if subscriptionPage("/pages/orders/detail?id=abc") != "pages/orders/detail?id=abc" {
		t.Fatal("合法页面失效")
	}
	d := subscriptionStoredDelivery{SubscriptionDelivery: SubscriptionDelivery{Status: "uncertain", ExpiresAt: time.Now().Add(time.Hour).Format(time.RFC3339Nano)}}
	if subscriptionRetryable(d) {
		t.Fatal("未知结果不可重试")
	}
}

// TestIntegrationSubscriptions 在真实PG和Redis验证发送、权限、幂等、拒收、并发和故障路径。
func TestIntegrationSubscriptions(t *testing.T) {
	api := newTestAPI(t)
	t.Run("默认配置关闭且权限隔离", func(t *testing.T) {
		api.call("worker", "GET", "/subscriptions", nil, 200, nil)
		api.call("worker", "GET", "/admin/subscriptions/settings", nil, 403, nil)
		api.call("worker", "GET", "/admin/subscriptions/deliveries", nil, 403, nil)
		api.call("worker", "POST", "/admin/subscriptions/deliveries/missing/retry", nil, 403, nil)
		api.call("", "GET", "/subscriptions", nil, 401, nil)
	})
	t.Run("事务回滚与读后删除幂等", func(t *testing.T) {
		subscriptionTestReset(t, api)
		n := subscriptionTestNotice("txn-once")
		tx, err := api.App.DB.Begin(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		if err = queueDomainNotice(t.Context(), tx, n); err != nil {
			t.Fatal(err)
		}
		tx.Rollback(t.Context())
		var count int
		api.App.DB.QueryRow(t.Context(), "SELECT count(*) FROM subscription_deliveries").Scan(&count)
		if count != 0 {
			t.Fatal("回滚仍发通知")
		}
		if err = queueDomainNotice(t.Context(), api.App.DB, n); err != nil {
			t.Fatal(err)
		}
		api.App.DB.Exec(t.Context(), "DELETE FROM inbox")
		if err = queueDomainNotice(t.Context(), api.App.DB, n); err != nil {
			t.Fatal(err)
		}
		api.App.DB.QueryRow(t.Context(), "SELECT count(*) FROM inbox").Scan(&count)
		if count != 0 {
			t.Fatal("已读删除后重复事件重建消息")
		}
		subscriptionTestDelivery(t, api)
	})
	t.Run("授权请求幂等并发且模板共享", func(t *testing.T) {
		subscriptionTestReset(t, api)
		body := map[string]any{"request_id": "same-request-1234", "results": []map[string]string{{"scene": "provider_order", "template_id": "test_template_new_order", "status": "accept"}}}
		var wg sync.WaitGroup
		failures := make(chan string, 8)
		for i := 0; i < 8; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				status, raw, err := api.request("worker", "POST", "/subscriptions/authorizations", body)
				if err != nil || status != 200 {
					failures <- fmt.Sprintf("%d %s %v", status, raw, err)
				}
			}()
		}
		wg.Wait()
		close(failures)
		for fail := range failures {
			t.Error(fail)
		}
		var count int
		api.App.DB.QueryRow(t.Context(), "SELECT count(*) FROM subscription_authorizations").Scan(&count)
		if count != 1 {
			t.Fatalf("重复候选%d", count)
		}
		body["results"] = []map[string]string{{"scene": "provider_order", "template_id": "test_template_new_order", "status": "reject"}}
		api.call("worker", "POST", "/subscriptions/authorizations", body, 409, nil)
	})
	t.Run("两个消费者只发送一次且不泄露身份", func(t *testing.T) {
		subscriptionTestReset(t, api)
		subscriptionTestAuthorize(api, "send-once-123", "accept")
		var calls atomic.Int32
		api.App.HTTPClient = subscriptionTestTransport(&calls, func(r *http.Request) (*http.Response, error) {
			var body map[string]any
			json.NewDecoder(r.Body).Decode(&body)
			if body["touser"] != "test-private-openid" || body["page"] != "pages/orders/detail?id=test-order" {
				t.Error("官方身份或页面错误")
			}
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"errcode":0,"errmsg":"ok"}`))}, nil
		})
		if err := queueDomainNotice(t.Context(), api.App.DB, subscriptionTestNotice("send-once")); err != nil {
			t.Fatal(err)
		}
		var wg sync.WaitGroup
		for i := 0; i < 2; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				if _, err := api.App.processSubscriptionOne(t.Context()); err != nil {
					t.Error(err)
				}
			}()
		}
		wg.Wait()
		d := subscriptionTestDelivery(t, api)
		if d.Status != "sent" || calls.Load() != 1 {
			t.Fatalf("status=%s calls=%d", d.Status, calls.Load())
		}
		api.call("admin", "POST", "/admin/subscriptions/deliveries/"+d.ID+"/retry", nil, 409, nil)
		_, raw, _ := api.request("admin", "GET", "/admin/subscriptions/deliveries", nil)
		for _, secret := range []string{"test-private-openid", "mock-only-token", "authorization_id", "values", "lease_token"} {
			if bytes.Contains(raw, []byte(secret)) {
				t.Fatalf("日志泄露%s", secret)
			}
		}
	})
	t.Run("拒绝偏好身份配置事件过期均不外发", func(t *testing.T) {
		cases := []struct {
			name, want string
			setup      func()
		}{{"refused", "skipped_no_authorization", func() { subscriptionTestAuthorize(api, "deny-once-123", "reject") }}, {"preference", "skipped_preference", func() {
			api.call("worker", "PATCH", "/subscriptions/preferences", map[string]any{"preferences": map[string]bool{"provider_order": false}}, 200, nil)
		}}, {"identity", "skipped_identity", func() { mergeIdentity(t.Context(), api.App.DB, "worker", map[string]string{"appid": "other-app"}) }}, {"disabled", "skipped_disabled", func() {
			s := subscriptionTestSettings()
			s.Enabled = false
			putRaw(t.Context(), api.App.DB, "settings", "wechat_subscriptions", s)
		}}, {"event", "skipped_config", func() {
			s := subscriptionTestSettings()
			s.Scenes[0].Fields = map[string]string{}
			putRaw(t.Context(), api.App.DB, "settings", "wechat_subscriptions", s)
		}}, {"expired", "expired", func() {
			api.App.DB.Exec(t.Context(), `UPDATE subscription_deliveries SET data=jsonb_set(data,'{expires_at}',to_jsonb($1::text))`, time.Now().Add(-time.Hour).Format(time.RFC3339Nano))
		}}}
		for _, c := range cases {
			t.Run(c.name, func(t *testing.T) {
				subscriptionTestReset(t, api)
				var calls atomic.Int32
				api.App.HTTPClient = subscriptionTestTransport(&calls, func(*http.Request) (*http.Response, error) { return nil, fmt.Errorf("不应发送") })
				if err := queueDomainNotice(t.Context(), api.App.DB, subscriptionTestNotice("skip-"+c.name)); err != nil {
					t.Fatal(err)
				}
				c.setup()
				subscriptionTestDrain(t, api)
				d := subscriptionTestDelivery(t, api)
				if d.Status != c.want || calls.Load() != 0 {
					t.Fatalf("%s calls=%d", d.Status, calls.Load())
				}
			})
		}
	})
	t.Run("超时结果未知永不盲目重试", func(t *testing.T) {
		subscriptionTestReset(t, api)
		subscriptionTestAuthorize(api, "timeout-auth-123", "accept")
		var calls atomic.Int32
		api.App.HTTPClient = subscriptionTestTransport(&calls, func(*http.Request) (*http.Response, error) { return nil, context.DeadlineExceeded })
		queueDomainNotice(t.Context(), api.App.DB, subscriptionTestNotice("timeout"))
		subscriptionTestDrain(t, api)
		d := subscriptionTestDelivery(t, api)
		if d.Status != "uncertain" || calls.Load() != 1 {
			t.Fatalf("%s calls=%d", d.Status, calls.Load())
		}
		api.call("admin", "POST", "/admin/subscriptions/deliveries/"+d.ID+"/retry", nil, 409, nil)
		api.App.processSubscriptionOne(t.Context())
		if calls.Load() != 1 {
			t.Fatal("未知结果重复发送")
		}
	})
	t.Run("官方明确拒绝后安全重试", func(t *testing.T) {
		subscriptionTestReset(t, api)
		subscriptionTestAuthorize(api, "retry-auth-123", "accept")
		var calls atomic.Int32
		api.App.HTTPClient = subscriptionTestTransport(&calls, func(*http.Request) (*http.Response, error) {
			code := 40037
			if calls.Load() > 1 {
				code = 0
			}
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(fmt.Sprintf(`{"errcode":%d}`, code)))}, nil
		})
		queueDomainNotice(t.Context(), api.App.DB, subscriptionTestNotice("safe-retry"))
		subscriptionTestDrain(t, api)
		d := subscriptionTestDelivery(t, api)
		if d.Status != "failed_retryable" {
			t.Fatal(d.Status)
		}
		api.call("admin", "POST", "/admin/subscriptions/deliveries/"+d.ID+"/retry", nil, 200, nil)
		subscriptionTestDrain(t, api)
		d = subscriptionTestDelivery(t, api)
		if d.Status != "sent" || calls.Load() != 2 {
			t.Fatalf("%s calls=%d", d.Status, calls.Load())
		}
	})
	t.Run("准备租约可恢复发送租约变未知", func(t *testing.T) {
		subscriptionTestReset(t, api)
		subscriptionTestAuthorize(api, "lease-auth-123", "accept")
		queueDomainNotice(t.Context(), api.App.DB, subscriptionTestNotice("lease-recovery"))
		d, err := api.App.claimSubscriptionDelivery(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		d, _, _, err = api.App.prepareSubscriptionSend(t.Context(), d)
		if err != nil {
			t.Fatal(err)
		}
		d.LeaseUntil = time.Now().Add(-time.Minute).Format(time.RFC3339Nano)
		putRaw(t.Context(), api.App.DB, "subscription_deliveries", d.ID, d)
		var calls atomic.Int32
		api.App.HTTPClient = subscriptionTestTransport(&calls, func(*http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"errcode":0}`))}, nil
		})
		subscriptionTestDrain(t, api)
		if subscriptionTestDelivery(t, api).Status != "sent" {
			t.Fatal("准备阶段恢复失败")
		}
		d = subscriptionTestDelivery(t, api)
		d.Status = "sending"
		d.LeaseUntil = time.Now().Add(-time.Minute).Format(time.RFC3339Nano)
		putRaw(t.Context(), api.App.DB, "subscription_deliveries", d.ID, d)
		api.App.processSubscriptionOne(t.Context())
		if subscriptionTestDelivery(t, api).Status != "uncertain" || calls.Load() != 1 {
			t.Fatal("发送阶段崩溃盲重试")
		}
	})
	t.Run("管理员申请逐个绑定且重复事件不增发", func(t *testing.T) {
		subscriptionTestReset(t, api)
		for _, id := range []string{"todo1", "todo2"} {
			n := DomainNotice{ID: id, Scene: "admin_club", Event: "admin_club", Admins: true, Title: "俱乐部申请待审核", Body: "请进入管理台查看", ActionPath: "/pages/manage/clubs?id=" + id}
			if err := queueDomainNotice(t.Context(), api.App.DB, n); err != nil {
				t.Fatal(err)
			}
			if err := queueDomainNotice(t.Context(), api.App.DB, n); err != nil {
				t.Fatal(err)
			}
		}
		items, err := list[subscriptionStoredDelivery](t.Context(), api.App.DB, "subscription_deliveries", "")
		if err != nil || len(items) != 2 || items[0].ActionPath == items[1].ActionPath || items[0].Values["count"] != "1" || items[1].Values["count"] != "1" {
			t.Fatal("不同申请被合并或重复事件增发")
		}

	})
}

// TestIntegrationSubscriptionRoleVisibility 验证六类入口按本人当前管理身份展示，不能代其他角色上报授权。
func TestIntegrationSubscriptionRoleVisibility(t *testing.T) {
	api := newTestAPI(t)
	for _, item := range []struct {
		user  string
		count int
	}{{"player", 3}, {"worker", 3}, {"clubowner", 5}, {"admin", 4}} {
		var out struct {
			Scenes []struct {
				Scene string `json:"scene"`
			} `json:"scenes"`
		}
		api.call(item.user, "GET", "/subscriptions", nil, 200, &out)
		if len(out.Scenes) != item.count {
			t.Fatalf("%s scenes=%d want%d", item.user, len(out.Scenes), item.count)
		}
	}
	api.call("worker", "PATCH", "/subscriptions/preferences", map[string]any{"preferences": map[string]bool{"admin_club": true}}, 400, nil)
	api.call("worker", "PATCH", "/subscriptions/preferences", map[string]any{"preferences": map[string]bool{"club_member": true}}, 400, nil)
	api.call("worker", "POST", "/subscriptions/authorizations", map[string]any{"request_id": "wrong-role-123", "results": []map[string]string{{"scene": "admin_club", "template_id": "test_template_admin", "status": "accept"}}}, 403, nil)
}

// TestIntegrationSubscriptionOfficialFailure 验证微信拒收、异常响应和跳转均不能变成可重复发送成功。
func TestIntegrationSubscriptionOfficialFailure(t *testing.T) {
	api := newTestAPI(t)
	for _, item := range []struct {
		name, body, want string
		httpStatus       int
	}{{"no-grant", `{"errcode":43101}`, "skipped_no_authorization", 200}, {"malformed", `{"errmsg":"ok"}`, "uncertain", 200}, {"redirect", ``, "uncertain", 307}, {"error-status", `{"errcode":0}`, "uncertain", 500}} {
		t.Run(item.name, func(t *testing.T) {
			subscriptionTestReset(t, api)
			subscriptionTestAuthorize(api, "official-"+item.name, "accept")
			var calls atomic.Int32
			api.App.HTTPClient = subscriptionTestTransport(&calls, func(*http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: item.httpStatus, Header: http.Header{"Location": []string{"https://not-wechat.invalid/steal"}}, Body: io.NopCloser(strings.NewReader(item.body))}, nil
			})
			if err := queueDomainNotice(t.Context(), api.App.DB, subscriptionTestNotice(item.name)); err != nil {
				t.Fatal(err)
			}
			subscriptionTestDrain(t, api)
			d := subscriptionTestDelivery(t, api)
			if d.Status != item.want || calls.Load() != 1 {
				t.Fatalf("%s calls%d", d.Status, calls.Load())
			}
			if item.want == "uncertain" && subscriptionRetryable(d) {
				t.Fatal("未知响应不可重试")
			}
		})
	}
}

// TestIntegrationSubscriptionSharedTemplate 验证多个业务场景共享官方模板时，同一次微信选择只能消费一次。
func TestIntegrationSubscriptionSharedTemplate(t *testing.T) {
	api := newTestAPI(t)
	subscriptionTestReset(t, api)
	settings := subscriptionTestSettings()
	settings.Scenes[1] = SubscriptionScene{Scene: "provider_refund", Enabled: true, TemplateID: settings.Scenes[0].TemplateID, Fields: settings.Scenes[0].Fields}
	if err := putRaw(t.Context(), api.App.DB, "settings", "wechat_subscriptions", settings); err != nil {
		t.Fatal(err)
	}
	api.call("worker", "POST", "/subscriptions/authorizations", map[string]any{"request_id": "shared-template-123", "results": []map[string]string{{"scene": "provider_order", "template_id": settings.Scenes[0].TemplateID, "status": "accept"}, {"scene": "provider_refund", "template_id": settings.Scenes[0].TemplateID, "status": "accept"}}}, 200, nil)
	var candidates int
	api.App.DB.QueryRow(t.Context(), "SELECT count(*) FROM subscription_authorizations").Scan(&candidates)
	if candidates != 1 {
		t.Fatalf("一个微信模板被放大成%d候选", candidates)
	}
	var calls atomic.Int32
	api.App.HTTPClient = subscriptionTestTransport(&calls, func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"errcode":0}`))}, nil
	})
	first := subscriptionTestNotice("shared-first")
	second := subscriptionTestNotice("shared-second")
	second.Scene = "provider_refund"
	second.Event = "refund_requested"
	second.ActionPath = "/pages/orders/detail?id=test-refund"
	if err := putRaw(t.Context(), api.App.DB, "orders", "test-refund", Order{ID: "test-refund", OrderNo: "SUB-REFUND-1", BuyerID: "player", ProviderID: "worker", PaymentStatus: "paid", Status: "accepted", RefundStatus: "requested"}); err != nil {
		t.Fatal(err)
	}
	for _, n := range []DomainNotice{first, second} {
		if err := queueDomainNotice(t.Context(), api.App.DB, n); err != nil {
			t.Fatal(err)
		}
	}
	subscriptionTestDrain(t, api)
	subscriptionTestDrain(t, api)
	items, err := list[subscriptionStoredDelivery](t.Context(), api.App.DB, "subscription_deliveries", "")
	if err != nil {
		t.Fatal(err)
	}
	statuses := map[string]int{}
	for _, item := range items {
		statuses[item.Status]++
	}
	if calls.Load() != 1 || statuses["sent"] != 1 || statuses["skipped_no_authorization"] != 1 {
		t.Fatalf("跨场景重复消费calls=%d status=%v", calls.Load(), statuses)
	}
}

// TestIntegrationSubscriptionRecipientChanged 验证订单归属在排队后改变时，旧接收人不能继续收到微信提醒。
func TestIntegrationSubscriptionRecipientChanged(t *testing.T) {
	api := newTestAPI(t)
	subscriptionTestReset(t, api)
	subscriptionTestAuthorize(api, "recipient-change-123", "accept")
	if err := queueDomainNotice(t.Context(), api.App.DB, subscriptionTestNotice("recipient-change")); err != nil {
		t.Fatal(err)
	}
	if err := putRaw(t.Context(), api.App.DB, "orders", "test-order", Order{ID: "test-order", BuyerID: "player", ProviderID: "outsider"}); err != nil {
		t.Fatal(err)
	}
	var calls atomic.Int32
	api.App.HTTPClient = subscriptionTestTransport(&calls, func(*http.Request) (*http.Response, error) { return nil, fmt.Errorf("旧接收人不可发送") })
	subscriptionTestDrain(t, api)
	if d := subscriptionTestDelivery(t, api); d.Status != "skipped_identity" || calls.Load() != 0 {
		t.Fatalf("%s calls=%d", d.Status, calls.Load())
	}
}
