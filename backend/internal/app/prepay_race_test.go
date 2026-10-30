// Package app 通过阻塞真实协议传输验证预支付与取消互斥，以及超时后的关闭支付能力。
package app

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// TestIntegrationPrepayCancelSerialization 验证取消必须等待预支付建单结束，不能先关不存在的单再建单。
func TestIntegrationPrepayCancelSerialization(t *testing.T) {
	api := newTestAPI(t)
	c, key := testPayConfig(t)
	configurePayEnvironment(t, c)
	api.App.Config.WechatAppID = c.AppID
	api.App.Config.DevMode = false
	api.App.commerceConfig = &commerceSettings{Mode: "official_account", FeeBearer: "platform", OfficialAppID: c.AppID}
	if err := put(t.Context(), api.App.DB, "identities", "player", testVerifiedWechatIdentity("payer-openid")); err != nil {
		t.Fatal(err)
	}
	var o Order
	api.call("player", "POST", "/orders", map[string]any{"service_id": "delta-service-1", "quantity": 1}, 200, &o)
	started, release := make(chan struct{}), make(chan struct{})
	var closes atomic.Int32
	api.App.HTTPClient = &http.Client{Transport: paymentTransport(func(req *http.Request) (*http.Response, error) {
		if strings.HasSuffix(req.URL.Path, "/jsapi") {
			close(started)
			select {
			case <-release:
			case <-req.Context().Done():
				return nil, req.Context().Err()
			}
			body := []byte(`{"prepay_id":"prepay-real-protocol"}`)
			return &http.Response{StatusCode: 200, Header: signedHeader(t, c, key, body), Body: io.NopCloser(bytes.NewReader(body))}, nil
		}
		if strings.HasSuffix(req.URL.Path, "/close") {
			closes.Add(1)
			return &http.Response{StatusCode: 204, Header: signedHeader(t, c, key, nil), Body: io.NopCloser(bytes.NewReader(nil))}, nil
		}
		t.Errorf("非预期微信接口:%s", req.URL.Path)
		return nil, context.Canceled
	})}
	payResult, cancelResult := make(chan int, 1), make(chan int, 1)
	go func() {
		code, _, _ := api.request("player", "POST", "/orders/"+o.ID+"/pay", map[string]any{})
		payResult <- code
	}()
	select {
	case <-started:
	case <-time.After(3 * time.Second):
		t.Fatal("预支付请求没有到达")
	}
	go func() {
		code, _, _ := api.request("player", "POST", "/orders/"+o.ID+"/actions", map[string]string{"action": "cancel"})
		cancelResult <- code
	}()
	select {
	case code := <-cancelResult:
		close(release)
		t.Fatalf("建单仍阻塞时取消已提前返回:%d", code)
	case <-time.After(100 * time.Millisecond):
	}
	if closes.Load() != 0 {
		close(release)
		t.Fatal("预支付完成前不应调用微信关单")
	}
	close(release)
	if code := <-payResult; code != 200 {
		t.Fatalf("预支付失败:%d", code)
	}
	if code := <-cancelResult; code != 200 {
		t.Fatalf("取消失败:%d", code)
	}
	saved, err := get[Order](t.Context(), api.App.DB, "orders", o.ID, false)
	if err != nil || saved.Status != "cancelled" || saved.PayOpenID == "" || closes.Load() != 1 {
		t.Fatalf("取消或意图持久化异常:%+v %v", saved, err)
	}
}

// TestIntegrationPrepayTimeoutKeepsIntent 确认上游超时后仍保存预支付身份，取消会执行真实关单。
func TestIntegrationPrepayTimeoutKeepsIntent(t *testing.T) {
	api := newTestAPI(t)
	c, key := testPayConfig(t)
	configurePayEnvironment(t, c)
	api.App.Config.WechatAppID = c.AppID
	api.App.Config.DevMode = false
	api.App.commerceConfig = &commerceSettings{Mode: "official_account", FeeBearer: "platform", OfficialAppID: c.AppID}
	put(t.Context(), api.App.DB, "identities", "player", testVerifiedWechatIdentity("payer-openid"))
	var o Order
	api.call("player", "POST", "/orders", map[string]any{"service_id": "delta-service-1", "quantity": 1}, 200, &o)
	var closes atomic.Int32
	api.App.HTTPClient = &http.Client{Transport: paymentTransport(func(req *http.Request) (*http.Response, error) {
		if strings.HasSuffix(req.URL.Path, "/jsapi") {
			return nil, context.DeadlineExceeded
		}
		closes.Add(1)
		return &http.Response{StatusCode: 204, Header: signedHeader(t, c, key, nil), Body: io.NopCloser(bytes.NewReader(nil))}, nil
	})}
	api.call("player", "POST", "/orders/"+o.ID+"/pay", map[string]any{}, 502, nil)
	saved, err := get[Order](t.Context(), api.App.DB, "orders", o.ID, false)
	if err != nil || saved.PayOpenID != "payer-openid" || saved.Status != "pending_payment" {
		t.Fatalf("超时后预支付意图丢失:%+v %v", saved, err)
	}
	api.action("player", o.ID, "cancel", "", 200)
	if closes.Load() != 1 {
		t.Fatal("超时预支付没有被正确关闭")
	}
}

// TestIntegrationEvidenceLinkRenewal 验证链接续签需要订单权限，且不能跨单引用图片。
func TestIntegrationEvidenceLinkRenewal(t *testing.T) {
	api := newTestAPI(t)
	o := api.newOrder()
	imageURL := api.uploadTestImage("player", o.ID)
	id := strings.TrimPrefix(strings.Split(imageURL, "?")[0], "/api/evidence/")
	var result map[string]string
	api.call("worker", "GET", "/orders/"+o.ID+"/evidence-link?evidence_id="+id, nil, 200, &result)
	if !strings.Contains(result["url"], "signature=") {
		t.Fatal("续签未返回签名地址")
	}
	api.call("outsider", "GET", "/orders/"+o.ID+"/evidence-link?evidence_id="+id, nil, 403, nil)
	other := api.newOrder()
	api.call("player", "GET", "/orders/"+other.ID+"/evidence-link?evidence_id="+id, nil, 403, nil)
}

// TestIntegrationPrepayWithSingleDBConnection 确认持有订单事务时不会再次向连接池借连接而饥饿。
func TestIntegrationPrepayWithSingleDBConnection(t *testing.T) {
	database, e := url.Parse(LoadConfig().DatabaseURL)
	if e != nil {
		t.Fatal(e)
	}
	params := database.Query()
	params.Set("pool_max_conns", "1")
	database.RawQuery = params.Encode()
	t.Setenv("DATABASE_URL", database.String())
	api := newTestAPI(t)
	if api.App.DB.Config().MaxConns != 1 {
		t.Fatal("测试必须只有一个数据库连接")
	}
	api.Server.Client().Timeout = 2 * time.Second
	c, key := testPayConfig(t)
	configurePayEnvironment(t, c)
	api.App.Config.DevMode = false
	api.App.commerceConfig = &commerceSettings{Mode: "official_account", FeeBearer: "platform", OfficialAppID: c.AppID}
	api.App.Config.WechatAppID = c.AppID
	if e = put(t.Context(), api.App.DB, "identities", "player", testVerifiedWechatIdentity("single-connection-openid")); e != nil {
		t.Fatal(e)
	}
	api.App.HTTPClient = &http.Client{Transport: paymentTransport(func(r *http.Request) (*http.Response, error) {
		body := []byte(`{"prepay_id":"single-connection-prepay"}`)
		return &http.Response{StatusCode: 200, Header: signedHeader(t, c, key, body), Body: io.NopCloser(bytes.NewReader(body))}, nil
	})}
	var o Order
	api.call("player", "POST", "/orders", map[string]any{"service_id": "delta-service-1", "quantity": 1}, 200, &o)
	api.call("player", "POST", "/orders/"+o.ID+"/pay", map[string]any{}, 200, nil)
	saved, e := get[Order](t.Context(), api.App.DB, "orders", o.ID, false)
	if e != nil || saved.PayOpenID != "single-connection-openid" || saved.PaymentStatus != "unpaid" {
		t.Fatalf("预支付意图未正确持久化:%+v %v", saved, e)
	}
}
