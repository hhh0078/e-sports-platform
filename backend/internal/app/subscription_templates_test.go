// Package app 验证官方订阅模板字段解析、管理员权限和上游响应边界，不调用真实微信。
package app

import (
	"context"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
)

// TestSubscriptionTemplateFields 验证中文标点、字段顺序和重复字段不会虚增映射。
func TestSubscriptionTemplateFields(t *testing.T) {
	fields := subscriptionTemplateFields("订单名称:{{thing5.DATA}}\n金额：{{amount3.DATA}}\n时间:{{time4.DATA}}\n重复:{{thing5.DATA}}\n{{phrase2.DATA}}")
	if len(fields) != 4 || fields[0].Key != "thing5" || fields[0].Label != "订单名称" || fields[1].Type != "amount" || fields[1].Label != "金额" || fields[3].Label != "phrase2" {
		t.Fatalf("字段解析不符：%+v", fields)
	}
}

// TestSubscriptionTemplateOfficialList 验证固定官方地址、隐藏示例、未知用户隔离和失效凭证错误。
func TestSubscriptionTemplateOfficialList(t *testing.T) {
	api := newTestAPI(t)
	for _, actor := range []string{"", "player", "worker", "clubowner"} {
		status := http.StatusForbidden
		if actor == "" {
			status = http.StatusUnauthorized
		}
		api.call(actor, "GET", "/admin/subscriptions/templates", nil, status, nil)
	}
	api.App.Config.WechatAppID = "subscription-test-app"
	api.App.Config.WechatSecret = "subscription-test-secret"
	var refresh atomic.Bool
	api.App.HTTPClient = &http.Client{Transport: paymentTransport(func(r *http.Request) (*http.Response, error) {
		if r.URL.Host != "api.weixin.qq.com" || r.URL.Scheme != "https" {
			t.Fatal("官方模板请求地址异常")
		}
		body := `{"errcode":0,"data":[{"priTmplId":"template-test-id","title":"交易提醒","type":2,"content":"商品信息:{{thing2.DATA}}\n支付金额:{{amount4.DATA}}","example":"不应返回的示例联系方式"}]}`
		if r.URL.Path == "/cgi-bin/stable_token" {
			body = `{"access_token":"test-subscription-access","expires_in":7200}`
		} else if r.URL.Path != "/wxaapi/newtmpl/gettemplate" {
			t.Fatal("模板读取发起了无关微信请求")
		} else if refresh.Load() {
			body = `{"errcode":48001,"errmsg":"secret-never-return"}`
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: http.Header{}}, nil
	})}
	var result struct {
		Items []SubscriptionWechatTemplate `json:"items"`
	}
	api.call("admin", "GET", "/admin/subscriptions/templates", nil, 200, &result)
	if len(result.Items) != 1 || result.Items[0].Title != "交易提醒" || len(result.Items[0].Fields) != 2 {
		t.Fatalf("模板列表不符：%+v", result)
	}
	api.App.Config.DevMode = false
	settings := defaultSubscriptionSettings()
	settings.Enabled = true
	settings.Scenes[0] = SubscriptionScene{Scene: "provider_order", Enabled: true, TemplateID: "template-test-id", Fields: map[string]string{"thing2": "title", "amount4": "amount"}}
	if err := api.App.validateOfficialSubscriptionSettings(t.Context(), settings); err != nil {
		t.Fatal(err)
	}
	settings.Scenes[0].Fields = map[string]string{"thing2": "title"}
	if err := api.App.validateOfficialSubscriptionSettings(t.Context(), settings); err == nil {
		t.Fatal("正式启用不得遗漏官方必填字段")
	}
	settings.Scenes[0].TemplateID = "template-another-app"
	if err := api.App.validateOfficialSubscriptionSettings(t.Context(), settings); err == nil {
		t.Fatal("正式启用不得借用其它小程序模板")
	}
	refresh.Store(true)
	_, err := api.App.fetchSubscriptionTemplates(context.Background())
	if err == nil || strings.Contains(err.Error(), "secret-never-return") {
		t.Fatal("上游错误应脱敏后返回")
	}
}
