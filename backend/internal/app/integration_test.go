// Package app 在真实 PostgreSQL 和 Redis 中验证四类身份、并发交易与证据隔离。
package app

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/jackc/pgx/v5/pgxpool"
	"image"
	"image/png"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
)

type testAPI struct {
	t      *testing.T
	Server *httptest.Server
	App    *App
	Tokens map[string]string
}

// newTestAPI 使用独立临时 schema，不触及开发演示记录。
func newTestAPI(t *testing.T) *testAPI {
	t.Helper()
	if os.Getenv("TEST_INTEGRATION") != "true" {
		t.Skip("设置 TEST_INTEGRATION=true 运行真实数据库集成测试")
	}
	cfg := LoadConfig()
	cfg.DevMode = true
	cfg.Env = "test"
	base, err := pgxpool.New(t.Context(), cfg.DatabaseURL)
	if err != nil {
		t.Fatal(err)
	}
	schema := "test_" + newID()
	cfg.RedisPrefix = "jijie:" + schema + ":"
	if _, err = base.Exec(t.Context(), "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	u, err := url.Parse(cfg.DatabaseURL)
	if err != nil {
		t.Fatal(err)
	}
	query := u.Query()
	query.Set("search_path", schema)
	u.RawQuery = query.Encode()
	cfg.DatabaseURL = u.String()
	a, err := New(t.Context(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	s := httptest.NewServer(a.Handler())
	api := &testAPI{t: t, Server: s, App: a, Tokens: map[string]string{}}
	t.Cleanup(func() {
		s.Close()
		// 清理该测试唯一前缀下的会话、短票据与限流缓存，不触及演示或其他测试数据。
		iter := a.Redis.Scan(context.Background(), 0, a.redisKey("*"), 100).Iterator()
		for iter.Next(context.Background()) {
			a.Redis.Del(context.Background(), iter.Val())
		}
		a.Close()
		base.Exec(context.Background(), "DROP SCHEMA "+schema+" CASCADE")
		base.Close()
	})
	for _, id := range []string{"player", "worker", "clubowner", "admin", "outsider"} {
		var result struct {
			Token string `json:"token"`
		}
		api.call("", "POST", "/auth/dev", map[string]any{"user_id": id}, 200, &result)
		api.Tokens[id] = result.Token
		api.call(id, "POST", "/me/phone/dev", map[string]string{"phone_number": "13800138000"}, 200, nil)
	}
	return api
}

// request 发起完整 HTTP 请求并返回原始响应，供并发测试安全使用。
func (api *testAPI) request(user, method, path string, body any) (int, []byte, error) {
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return 0, nil, err
		}
		reader = bytes.NewReader(raw)
	}
	req, err := http.NewRequest(method, api.Server.URL+"/api"+path, reader)
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if user != "" {
		req.Header.Set("Authorization", "Bearer "+api.Tokens[user])
	}
	res, err := api.Server.Client().Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer res.Body.Close()
	raw, err := io.ReadAll(res.Body)
	return res.StatusCode, raw, err
}

// call 断言响应状态并解包标准 data 字段。
func (api *testAPI) call(user, method, path string, body any, status int, out any) {
	api.t.Helper()
	code, raw, err := api.request(user, method, path, body)
	if err != nil {
		api.t.Fatal(err)
	}
	if code != status {
		api.t.Fatalf("%s %s expected %d got %d: %s", method, path, status, code, raw)
	}
	if out != nil {
		var envelope struct {
			Data json.RawMessage `json:"data"`
		}
		if err = json.Unmarshal(raw, &envelope); err != nil {
			api.t.Fatal(err)
		}
		if err = json.Unmarshal(envelope.Data, out); err != nil {
			api.t.Fatal(err)
		}
	}
}

// newOrder 创建种子服务订单并支付。
func (api *testAPI) newOrder() Order {
	api.t.Helper()
	var o Order
	api.call("player", "POST", "/orders", map[string]any{"service_id": "delta-service-1", "quantity": 2, "requirements": "今晚一起护航撤离"}, 200, &o)
	api.call("player", "POST", "/orders/"+o.ID+"/pay", map[string]any{}, 200, nil)
	return o
}

// action 调用状态机并断言结果。
func (api *testAPI) action(user, id, action, note string, status int) Order {
	api.t.Helper()
	var o Order
	var out any
	if status == 200 {
		out = &o
	}
	api.call(user, "POST", "/orders/"+id+"/actions", map[string]any{"action": action, "note": note}, status, out)
	return o
}

// TestIntegrationOrderRoles 验证四方沟通、角色越权和完整交付评价流程。
func TestIntegrationOrderRoles(t *testing.T) {
	api := newTestAPI(t)
	api.call("player", "GET", "/admin/users", nil, 403, nil)
	api.call("player", "POST", "/orders", map[string]any{"service_id": "delta-service-1", "quantity": 1, "amount_cents": 1}, 400, nil)
	api.call("worker", "POST", "/orders", map[string]any{"service_id": "delta-service-1", "quantity": 1}, 400, nil)
	o := api.newOrder()
	if o.AmountCents != 3800 {
		t.Fatal("价格必须读取服务端并乘数量")
	}
	for _, role := range []string{"player", "worker", "clubowner", "admin"} {
		api.call(role, "GET", "/orders/"+o.ID, nil, 200, nil)
		var m Message
		api.call(role, "POST", "/orders/"+o.ID+"/messages", map[string]any{"content": "身份：" + role}, 200, &m)
		expected := map[string]string{"player": "buyer", "worker": "provider", "clubowner": "club_admin", "admin": "admin"}
		if m.SenderRole != expected[role] {
			t.Fatalf("聊天角色错误: %+v", m)
		}
	}
	api.call("outsider", "GET", "/orders/"+o.ID, nil, 403, nil)
	api.call("outsider", "GET", "/orders/"+o.ID+"/messages", nil, 403, nil)
	api.call("outsider", "POST", "/orders/"+o.ID+"/messages", map[string]string{"content": "越权"}, 403, nil)
	api.action("player", o.ID, "accept", "", 403)
	api.action("worker", o.ID, "deliver", "交付说明", 409)
	api.action("worker", o.ID, "accept", "", 200)
	api.action("worker", o.ID, "start", "", 200)
	api.action("worker", o.ID, "deliver", "全部目标已经完成，请查看证据", 200)
	api.action("worker", o.ID, "confirm", "", 403)
	api.action("player", o.ID, "confirm", "", 200)
	api.call("player", "POST", "/orders/"+o.ID+"/review", map[string]any{"rating": 5, "content": "耐心靠谱的队友"}, 200, nil)
	api.call("player", "POST", "/orders/"+o.ID+"/review", map[string]any{"rating": 5, "content": "重复评价"}, 409, nil)
	var stats map[string]int64
	api.call("worker", "GET", "/me/stats", nil, 200, &stats)
	if stats["earnings_cents"] != 3800 {
		t.Fatalf("结算收入不正确: %v", stats)
	}
	var msgs []Message
	api.call("admin", "GET", "/orders/"+o.ID+"/messages", nil, 200, &msgs)
	systems := 0
	for _, m := range msgs {
		if m.Kind == "system" {
			systems++
		}
	}
	if systems != 6 {
		t.Fatalf("系统事件应为创建付款接受开始交付确认共6条，实际 %d", systems)
	}
}

// TestIntegrationConcurrentClaims 验证重复支付与并发抢单只有一次实际生效。
func TestIntegrationConcurrentClaims(t *testing.T) {
	api := newTestAPI(t)
	var o Order
	api.call("player", "POST", "/requests", map[string]any{"game_id": "delta", "title": "周末任务协作", "requirements": "一起完成本周任务并复盘路线", "amount_cents": 2500}, 200, &o)
	api.action("worker", o.ID, "claim", "", 409)
	var wg sync.WaitGroup
	var mu sync.Mutex
	success := 0
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			code, _, err := api.request("player", "POST", "/orders/"+o.ID+"/pay", map[string]any{})
			if err != nil || code != 200 {
				t.Errorf("重复付款异常: %d %v", code, err)
			}
		}()
	}
	wg.Wait()
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			user := []string{"worker", "outsider"}[i%2]
			code, raw, err := api.request(user, "POST", "/orders/"+o.ID+"/actions", map[string]string{"action": "claim"})
			if err != nil {
				t.Error(err)
				return
			}
			if code == 200 {
				mu.Lock()
				success++
				mu.Unlock()
			} else if code != 409 {
				t.Errorf("抢单异常 %d %s", code, raw)
			}
		}(i)
	}
	wg.Wait()
	if success != 1 {
		t.Fatalf("必须只有一个抢单成功，实际 %d", success)
	}
	var messages []Message
	api.call("player", "GET", "/orders/"+o.ID+"/messages", nil, 200, &messages)
	if len(messages) != 3 {
		t.Fatalf("重复付款不应产生重复事件，实际 %d", len(messages))
	}
}

// TestIntegrationRefundAndIsolation 验证退款暂停交付、平台介入、历史组织隔离及收入回冲。
func TestIntegrationRefundAndIsolation(t *testing.T) {
	api := newTestAPI(t)
	o := api.newOrder()
	api.action("worker", o.ID, "accept", "", 200)
	api.action("worker", o.ID, "start", "", 200)
	api.action("player", o.ID, "request_refund", "服务进度与约定不一致", 200)
	api.action("worker", o.ID, "deliver", "尝试绕过退款交付", 409)
	api.action("worker", o.ID, "reject_refund", "有证据说明已经按约定执行", 200)
	api.action("player", o.ID, "escalate", "请平台查看聊天证据", 200)
	api.call("admin", "POST", "/admin/orders/"+o.ID+"/resolve", map[string]string{"resolution": "complete", "note": "不能将未交付订单强行结算"}, 409, nil)
	api.call("admin", "POST", "/admin/orders/"+o.ID+"/resolve", map[string]string{"resolution": "resume", "note": "证据暂不支持退款，继续履约"}, 200, nil)
	api.action("worker", o.ID, "deliver", "已完成全部约定并提交证据", 200)
	api.action("player", o.ID, "confirm", "", 200)
	api.action("player", o.ID, "request_refund", "完成后发现服务结果存在问题", 200)
	api.action("worker", o.ID, "approve_refund", "同意售后退款", 200)
	var stats map[string]int64
	api.call("worker", "GET", "/me/stats", nil, 200, &stats)
	if stats["earnings_cents"] != 0 {
		t.Fatal("退款必须冲回已完成收入")
	}
	api.call("worker", "POST", "/clubs/shanhai/leave", map[string]any{}, 200, nil)
	var s Service
	api.call("", "GET", "/services/delta-service-1", nil, 200, &s)
	if s.ClubVerified {
		t.Fatal("退出后认证必须立即撤销")
	}
	api.call("clubowner", "GET", "/orders/"+o.ID, nil, 200, nil)
	api.call("outsider", "GET", "/clubs/shanhai/orders", nil, 403, nil)
	api.call("clubowner", "GET", "/clubs/wild/orders", nil, 403, nil)
	o2 := api.newOrder()
	api.action("player", o2.ID, "dispute", "付款后发现问题需要平台处理", 200)
	for i := 0; i < 2; i++ {
		api.call("admin", "POST", "/admin/orders/"+o2.ID+"/resolve", map[string]string{"resolution": "refund", "note": "证据明确，同意退款"}, 200, nil)
	}
}

// uploadTestImage 创建最小有效 PNG 并提交订单图片证据。
func (api *testAPI) uploadTestImage(user, orderID string) string {
	api.t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", "proof.png")
	if err != nil {
		api.t.Fatal(err)
	}
	if err = png.Encode(part, image.NewRGBA(image.Rect(0, 0, 2, 2))); err != nil {
		api.t.Fatal(err)
	}
	writer.Close()
	req, _ := http.NewRequest("POST", api.Server.URL+"/api/orders/"+orderID+"/evidence", &body)
	req.Header.Set("Authorization", "Bearer "+api.Tokens[user])
	req.Header.Set("Content-Type", writer.FormDataContentType())
	res, err := api.Server.Client().Do(req)
	if err != nil {
		api.t.Fatal(err)
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)
	if res.StatusCode != 200 {
		api.t.Fatalf("上传失败: %s", raw)
	}
	var envelope struct {
		Data struct {
			URL    string `json:"url"`
			SHA256 string `json:"sha256"`
		} `json:"data"`
	}
	if err = json.Unmarshal(raw, &envelope); err != nil {
		api.t.Fatal(err)
	}
	if len(envelope.Data.SHA256) != 64 {
		api.t.Fatal("必须记录原始证据摘要")
	}
	return envelope.Data.URL
}

// TestIntegrationEvidenceAndFreeze 验证图片签名、跨订单引用保护和既有令牌冻结。
func TestIntegrationEvidenceAndFreeze(t *testing.T) {
	api := newTestAPI(t)
	o := api.newOrder()
	imageURL := api.uploadTestImage("player", o.ID)
	var m Message
	api.call("player", "POST", "/orders/"+o.ID+"/messages", map[string]string{"content": "服务约定截图", "image_url": imageURL}, 200, &m)
	if m.Kind != "image" {
		t.Fatal("图片消息类型错误")
	}
	res, err := http.Get(api.Server.URL + imageURL)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != 200 {
		t.Fatal("有效签名应能查看图片")
	}
	tampered := strings.Replace(imageURL, "signature=", "signature=0", 1)
	res, err = http.Get(api.Server.URL + tampered)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != 401 {
		t.Fatal("篡改签名不应访问图片")
	}
	o2 := api.newOrder()
	api.call("player", "POST", "/orders/"+o2.ID+"/messages", map[string]string{"image_url": imageURL}, 403, nil)
	api.call("player", "POST", "/orders/"+o.ID+"/messages", map[string]string{"content": "伪造系统消息", "sender_role": "system"}, 400, nil)
	api.call("admin", "PATCH", "/admin/users/player", map[string]string{"status": "frozen"}, 200, nil)
	api.call("player", "GET", "/me", nil, 403, nil)
	api.call("player", "GET", "/orders/"+o.ID, nil, 403, nil)
	api.call("admin", "PATCH", "/admin/users/admin", map[string]string{"status": "frozen"}, 400, nil)
	api.call("admin", "PATCH", "/admin/users/player", map[string]string{"status": "active"}, 200, nil)
	api.call("player", "GET", "/me", nil, 200, nil)
}

// TestIntegrationPublicationAndProduction 验证服务重审、组织审核隔离、生产关闭开发入口。
func TestIntegrationPublicationAndProduction(t *testing.T) {
	api := newTestAPI(t)
	var s Service
	api.call("worker", "POST", "/services", map[string]any{"game_id": "delta", "title": "测试自定义服务", "description": "明确提供陪练和任务协作服务，不索取游戏密码。", "category": "任务协作", "price_cents": 1900, "unit": "小时", "cover_url": "/static/covers/official/delta-1.jpg", "labels": []CoverLabel{{Text: "认真陪练", Color: "#FFFFFF", Background: "transparent", FontSize: 16}}, "tags": []string{"耐心"}, "club_id": "shanhai"}, 200, &s)
	if s.Status != "active" {
		t.Fatal("后置审核应先正常发布")
	}
	api.call("", "GET", "/services/"+s.ID, nil, 200, nil)
	api.call("outsider", "PATCH", "/services/"+s.ID, map[string]any{"game_id": "delta"}, 403, nil)
	api.call("admin", "PATCH", "/admin/services/"+s.ID, map[string]string{"status": "active"}, 200, nil)
	api.call("", "GET", "/services/"+s.ID, nil, 200, nil)
	var c Club
	api.call("player", "POST", "/clubs", map[string]any{"name": "新的俱乐部", "description": "这是一个独立创建的俱乐部，欢迎志同道合的伙伴", "game_ids": []string{"delta"}}, 200, &c)
	api.call("admin", "PATCH", "/admin/clubs/"+c.ID, map[string]any{"status": "active", "verified": true}, 200, nil)
	api.call("outsider", "POST", "/clubs/"+c.ID+"/join", map[string]any{}, 200, nil)
	api.call("clubowner", "POST", "/clubs/"+c.ID+"/members/outsider", map[string]string{"status": "active"}, 403, nil)
	api.call("player", "POST", "/clubs/"+c.ID+"/members/outsider", map[string]string{"status": "active"}, 200, nil)
	var auditEntries []Audit
	api.call("admin", "GET", "/admin/audit", nil, 200, &auditEntries)
	if len(auditEntries) < 3 {
		t.Fatal("管理操作必须保留审计")
	}
	api.App.Config.DevMode = false
	api.call("", "POST", "/auth/dev", map[string]string{"user_id": "admin"}, 404, nil)
	api.call("", "GET", "/dev/accounts", nil, 404, nil)
	api.App.Config.WechatSecret = ""
	api.call("", "POST", "/auth/wechat/prepare", map[string]string{"code": "fake"}, 503, nil)
	if err := put(t.Context(), api.App.DB, "identities", "player", testVerifiedWechatIdentity("test-only-player")); err != nil {
		t.Fatal(err)
	}
	var o Order
	api.call("player", "POST", "/orders", map[string]any{"service_id": "delta-service-1", "quantity": 1}, 200, &o)
	api.call("player", "POST", "/orders/"+o.ID+"/pay", map[string]any{}, 503, nil)
	var saved Order
	api.call("player", "GET", "/orders/"+o.ID, nil, 200, &saved)
	if saved.PaymentStatus != "unpaid" {
		t.Fatal("未配置真实支付不能伪成功")
	}
	fmt.Println("完整本地角色、交易、退款、权限、证据与生产隔离验收通过")
}
