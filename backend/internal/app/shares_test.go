// Package app 验证分享脱敏、实时状态、永久映射及真实微信图片协议，所有交易仅在隔离测试中模拟。
package app

import (
	"bytes"
	"encoding/json"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

// shareImageFixture 生成测试用有效图片字节，不作为应用中的真实微信小程序码。
func shareImageFixture(t *testing.T, width, height int, format string) []byte {
	t.Helper()
	var output bytes.Buffer
	var err error
	if format == "jpeg" {
		err = jpeg.Encode(&output, image.NewRGBA(image.Rect(0, 0, width, height)), nil)
	} else {
		err = png.Encode(&output, image.NewRGBA(image.Rect(0, 0, width, height)))
	}
	if err != nil {
		t.Fatal(err)
	}
	return output.Bytes()
}

// TestShareDescription 验证Unicode摘要截断保留完整字符且不会改变原文。
func TestShareDescription(t *testing.T) {
	if shareDescription("  队友同行  ") != "队友同行" || shareDescription("") != "" {
		t.Fatal("短摘要去空白不正确")
	}
	if got := shareDescription(strings.Repeat("队🎮", 80)); len([]rune(got)) != 121 || !strings.HasSuffix(got, "…") {
		t.Fatalf("中文和表情必须按字符截断: %q", got)
	}
}

// TestShareCodeImageValidation 验证真实图像边界、JPEG转PNG、伪造头部和错误JSON拒绝。
func TestShareCodeImageValidation(t *testing.T) {
	for _, format := range []string{"png", "jpeg"} {
		encoded, err := normalizeShareCode(shareImageFixture(t, 430, 430, format))
		if err != nil {
			t.Fatal(err)
		}
		config, decodedFormat, err := image.DecodeConfig(bytes.NewReader(encoded))
		if err != nil || decodedFormat != "png" || config.Width != 430 || config.Height != 430 {
			t.Fatalf("官方图像必须规范为完整PNG: %s %+v %v", decodedFormat, config, err)
		}
	}
	valid := shareImageFixture(t, 430, 430, "png")
	for _, raw := range [][]byte{nil, []byte(`{"errcode":41030,"errmsg":"秘密不可出现在响应"}`), []byte("<html>失败</html>"), valid[:33], shareImageFixture(t, 279, 279, "png"), shareImageFixture(t, 430, 320, "png"), shareImageFixture(t, 1281, 1281, "png"), make([]byte, shareCodeMaxBytes+1)} {
		if _, err := normalizeShareCode(raw); err == nil {
			t.Fatal("错误JSON、残缺或越界图片不得成为可分享的码")
		}
	}
}

// createShareFixture 通过正常登录接口生成分享，返回服务端实际确认的公开元数据。
func createShareFixture(api *testAPI, kind, target string) ShareMetadata {
	api.t.Helper()
	var share ShareMetadata
	api.call("player", "POST", "/shares", map[string]string{"kind": kind, "target_id": target, "game_id": "delta"}, 200, &share)
	return share
}

// paidShareJob 创建只有标题和预算可公开的已付款需求，完整约定用哨兵验证不会泄露。
func paidShareJob(api *testAPI) Order {
	api.t.Helper()
	var order Order
	api.call("player", "POST", "/requests", map[string]any{"game_id": "delta", "title": "周末游戏任务合作", "requirements": "私密验收哨兵 ONLY_ORDER_PARTICIPANTS", "amount_cents": 2300}, 200, &order)
	api.call("player", "POST", "/orders/"+order.ID+"/pay", map[string]any{}, 200, nil)
	return order
}

// TestIntegrationShareTargetsAndPrivacy 验证五类目标、分享人真实性、私密订单与公共需求的权限隔离。
func TestIntegrationShareTargetsAndPrivacy(t *testing.T) {
	api := newTestAPI(t)
	job := paidShareJob(api)
	for _, item := range []struct{ kind, target, path string }{{"home", "", "pages/main/index"}, {"service", "delta-service-1", "pages/services/detail"}, {"request", job.ID, "pages/requests/detail"}, {"player", "worker", "pages/community/players/detail"}, {"club", "shanhai", "pages/community/clubs/detail"}} {
		share := createShareFixture(api, item.kind, item.target)
		if !shareIDPattern.MatchString(share.ID) || share.Path != item.path || share.Kind != item.kind || share.TargetID != item.target || share.GameID != "delta" || share.EnvVersion != "release" || share.Sharer.ID != "player" || share.Sharer.Nickname != "云间旅人" || share.Sharer.AvatarURL == "" || !strings.HasPrefix(share.CoverURL, "/static/covers/") {
			t.Fatalf("分享元数据或身份不正确: %+v", share)
		}
		query, err := url.ParseQuery(share.Query)
		if err != nil || query.Get("share_id") != share.ID || query.Get("id") != item.target || query.Get("tab") == "manage" {
			t.Fatalf("分享只允许公开路由参数: %s", share.Query)
		}
		if item.kind == "home" && (query.Get("tab") != "player" || query.Get("game_id") != "delta") {
			t.Fatal("首页必须稳定指向公开玩家页并保留游戏")
		}
		if item.kind == "service" && share.AmountCents != 1900 || item.kind == "request" && share.AmountCents != 2300 {
			t.Fatal("海报金额未读取服务端实价")
		}
		code, raw, err := api.request("", "GET", "/shares/"+share.ID, nil)
		if err != nil || code != 200 {
			t.Fatalf("分享应可匿名解析: %d %s %v", code, raw, err)
		}
		for _, private := range []string{"ONLY_ORDER_PARTICIPANTS", "buyer_id", "payment_transaction_id", "phone_number", "openid", "sharer_id", "access_token", "contact_phone"} {
			if strings.Contains(string(raw), private) {
				t.Fatalf("分享泄露私密字段: %s", private)
			}
		}
	}
	code, raw, err := api.request("", "GET", "/jobs/"+job.ID, nil)
	if err != nil || code != 200 || strings.Contains(string(raw), "ONLY_ORDER_PARTICIPANTS") || strings.Contains(string(raw), "buyer_id") || !strings.Contains(string(raw), "接单后查看完整需求") {
		t.Fatalf("公共需求不能成为私密订单后门: %d %s %v", code, raw, err)
	}
	api.call("", "GET", "/orders/"+job.ID, nil, 401, nil)
	api.call("outsider", "GET", "/orders/"+job.ID, nil, 403, nil)
	api.call("", "POST", "/shares", map[string]string{"kind": "home"}, 401, nil)
	api.call("player", "POST", "/shares", map[string]string{"kind": "service", "target_id": "delta-service-1", "sharer_id": "admin"}, 400, nil)
	api.call("player", "POST", "/shares", map[string]string{"kind": "job", "target_id": job.ID}, 400, nil)
	api.call("player", "POST", "/shares", map[string]string{"kind": "home", "target_id": "worker"}, 400, nil)
	api.call("player", "POST", "/shares", map[string]string{"kind": "service", "target_id": ""}, 400, nil)
	api.call("player", "POST", "/shares", map[string]string{"kind": "service", "target_id": "delta-service-1", "game_id": "peace"}, 400, nil)
	api.call("player", "POST", "/shares", map[string]string{"kind": "home", "game_id": "missing"}, 404, nil)
	api.call("", "GET", "/shares/wx_long_user_id_is_not_a_scene", nil, 404, nil)
	api.call("", "GET", "/jobs/delta-service-1", nil, 404, nil)
}

// TestIntegrationShareStateIsCurrent 验证更新资料实时生效，暂停、账号冻结、领取和退款中的目标无法再分享。
func TestIntegrationShareStateIsCurrent(t *testing.T) {
	api := newTestAPI(t)
	share := createShareFixture(api, "service", "delta-service-1")
	user, err := get[User](t.Context(), api.App.DB, "users", "player", false)
	if err != nil {
		t.Fatal(err)
	}
	user.Nickname = "新分享人昵称"
	if err = put(t.Context(), api.App.DB, "users", user.ID, user); err != nil {
		t.Fatal(err)
	}
	var fresh ShareMetadata
	api.call("", "GET", "/shares/"+share.ID, nil, 200, &fresh)
	if fresh.Sharer.Nickname != user.Nickname {
		t.Fatal("永久映射不能把海报资料冻结成旧快照")
	}
	api.call("worker", "POST", "/services/delta-service-1/pause", map[string]any{}, 200, nil)
	api.call("", "GET", "/shares/"+share.ID, nil, 404, nil)
	api.call("admin", "POST", "/shares", map[string]string{"kind": "service", "target_id": "delta-service-1"}, 404, nil)
	job := paidShareJob(api)
	jobShare := createShareFixture(api, "request", job.ID)
	api.action("worker", job.ID, "claim", "", 200)
	api.call("", "GET", "/jobs/"+job.ID, nil, 404, nil)
	api.call("", "GET", "/shares/"+jobShare.ID, nil, 404, nil)
	api.call("", "GET", "/shares/"+jobShare.ID+"/code", nil, 404, nil)
	clubShare := createShareFixture(api, "club", "shanhai")
	club, _ := get[Club](t.Context(), api.App.DB, "clubs", "shanhai", false)
	club.Status = "pending"
	if err = put(t.Context(), api.App.DB, "clubs", club.ID, club); err != nil {
		t.Fatal(err)
	}
	api.call("", "GET", "/shares/"+clubShare.ID, nil, 404, nil)
	profileShare := createShareFixture(api, "player", "worker")
	user.Status = "frozen"
	if err = put(t.Context(), api.App.DB, "users", user.ID, user); err != nil {
		t.Fatal(err)
	}
	api.call("", "GET", "/shares/"+profileShare.ID, nil, 404, nil)
	api.call("player", "POST", "/shares", map[string]string{"kind": "home"}, 403, nil)
}

// TestIntegrationShareImagesAreHosted 验证不读取任意外部图片或私密证据，未托管封面明确失败。
func TestIntegrationShareImagesAreHosted(t *testing.T) {
	api := newTestAPI(t)
	private := Evidence{ID: newID(), OwnerID: "worker", OrderID: "private-order", Public: false, Data: shareImageFixture(t, 430, 430, "png"), MIME: "image/png"}
	if err := put(t.Context(), api.App.DB, "evidence", private.ID, private); err != nil {
		t.Fatal(err)
	}
	user, _ := get[User](t.Context(), api.App.DB, "users", "worker", false)
	user.ProfileCover, user.Avatar = "/api/media/"+private.ID, "https://127.0.0.1/private"
	if err := put(t.Context(), api.App.DB, "users", user.ID, user); err != nil {
		t.Fatal(err)
	}
	share := createShareFixture(api, "player", user.ID)
	if share.CoverURL == user.ProfileCover || share.Sharer.AvatarURL == user.Avatar {
		t.Fatal("分享不可展示私密图片或任意URL")
	}
	game, _ := get[Game](t.Context(), api.App.DB, "games", "delta", false)
	game.CoverURL = "https://127.0.0.1/internal-secret"
	if err := put(t.Context(), api.App.DB, "games", game.ID, game); err != nil {
		t.Fatal(err)
	}
	api.call("player", "POST", "/shares", map[string]string{"kind": "home", "game_id": "delta"}, 409, nil)
}

// TestIntegrationShareCreateRateLimit 验证生成分享记录按真实账号限流，不能无限增加永久映射。
func TestIntegrationShareCreateRateLimit(t *testing.T) {
	api := newTestAPI(t)
	for range 20 {
		createShareFixture(api, "home", "")
	}
	api.call("player", "POST", "/shares", map[string]string{"kind": "home", "game_id": "delta"}, 429, nil)
}

// shareCodeResponse 读取二进制分享码及响应头，保持失败JSON可被调用方完整检查。
func shareCodeResponse(api *testAPI, id string) (int, http.Header, []byte) {
	api.t.Helper()
	res, err := api.Server.Client().Get(api.Server.URL + "/api/shares/" + id + "/code")
	if err != nil {
		api.t.Fatal(err)
	}
	defer res.Body.Close()
	raw, err := io.ReadAll(res.Body)
	if err != nil {
		api.t.Fatal(err)
	}
	return res.StatusCode, res.Header, raw
}

// assertShareError 验证图片入口的错误仍为稳定JSON，且不会泄露上游敏感内容。
func assertShareError(t *testing.T, status int, header http.Header, raw []byte, expected int) {
	t.Helper()
	if status != expected || !strings.Contains(header.Get("Content-Type"), "application/json") || !json.Valid(raw) {
		t.Fatalf("错误码图片响应不正确: %d %v %s", status, header, raw)
	}
	for _, private := range []string{"test-secret-never-public", "secret-token", "upstream-secret", "api.weixin.qq.com", "access_token"} {
		if strings.Contains(string(raw), private) {
			t.Fatalf("上游秘密泄露: %s", raw)
		}
	}
}
