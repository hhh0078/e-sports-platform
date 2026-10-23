// Package app 验证平台和俱乐部客服配置的合并保存、公开字段白名单、二维码用途/归属及图文审核。
package app

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"image"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

// TestSupportContactFormats 验证电话及微信号只接受受限字符，二维码必须是1:1合理尺寸。
func TestSupportContactFormats(t *testing.T) {
	for _, v := range []struct {
		phone, wechat string
		valid         bool
	}{{"+86 (176) 0708-0935", "support_58-it", true}, {"", "", true}, {"400-123-4567", "wechat", true}, {"123", "wechat", true}, {"+123", "wechat", true}, {strings.Repeat("1", 20), "wechat", true}, {"++", "wechat", false}, {"()", "wechat", false}, {"1", "wechat", false}, {"12", "wechat", false}, {strings.Repeat("1", 21), "wechat", false}, {"123+456", "wechat", false}, {"客服热线", "wechat", false}, {"17607080935\n", "wechat", false}, {"123", "<script>", false}, {strings.Repeat("1", 33), "wechat", false}, {"123", strings.Repeat("w", 65), false}} {
		if (validateSupportContact(v.phone, v.wechat) == nil) != v.valid {
			t.Fatalf("联系方式校验异常:%+v", v)
		}
	}
	for _, purpose := range []string{"support_qr", "club_contact_qr"} {
		if validateImageDimensions(purpose, 256, 256) != nil || validateImageDimensions(purpose, 256, 128) == nil || validateImageDimensions(purpose, 64, 64) == nil {
			t.Fatalf("客服二维码比例约束失效:%s", purpose)
		}
	}
}

// TestIntegrationPlatformSupportContactsMerge 验证两个独立平台配置页不会互相清空，非管理员与其它图片用途不能修改平台客服。
func TestIntegrationPlatformSupportContactsMerge(t *testing.T) {
	api := newTestAPI(t)
	var original Settings
	api.call("admin", "GET", "/admin/settings", nil, 200, &original)
	valid := communityPNG(t, 256, 256)
	api.uploadCommunityImage("player", "support_qr", "", valid, 403)
	api.uploadCommunityImage("admin", "support_qr", "shanhai", valid, 400)
	api.uploadCommunityImage("admin", "support_qr", "", communityPNG(t, 256, 128), 400)
	qr := api.uploadCommunityImage("admin", "support_qr", "", valid, 200)
	avatar := api.uploadCommunityImage("admin", "avatar", "", valid, 200)
	other, e := get[User](t.Context(), api.App.DB, "users", "outsider", false)
	if e != nil {
		t.Fatal(e)
	}
	other.Role = "admin"
	if e = put(t.Context(), api.App.DB, "users", other.ID, other); e != nil {
		t.Fatal(e)
	}
	foreign := api.uploadCommunityImage("outsider", "support_qr", "", valid, 200)
	api.call("admin", "PATCH", "/admin/settings", map[string]string{"support_qr_url": foreign}, 403, nil)

	api.call("player", "PATCH", "/admin/settings", map[string]string{"support_phone": "17607080935"}, 403, nil)
	api.call("admin", "PATCH", "/admin/settings", map[string]string{"support_qr_url": avatar}, 400, nil)
	var updated Settings
	api.call("admin", "PATCH", "/admin/settings", map[string]string{"support_qr_url": qr, "support_phone": "17607080935", "support_wechat": "support_58it"}, 200, &updated)
	if updated.SiteName != original.SiteName || updated.Announcement != original.Announcement {
		t.Fatal("客服页覆盖了原运营设置")
	}
	api.call("admin", "PATCH", "/admin/settings", map[string]string{"site_name": "集结测试平台", "announcement": "配置页独立保存公告"}, 200, nil)
	api.call("admin", "PATCH", "/admin/settings", map[string]string{"support_phone": "400-123-4567"}, 200, &updated)
	if updated.SupportQRURL != qr || updated.SupportWechat != "support_58it" || updated.SiteName != "集结测试平台" || updated.Announcement != "配置页独立保存公告" {
		t.Fatalf("部分字段未保留:%+v", updated)
	}
	var boot map[string]any
	api.call("", "GET", "/bootstrap", nil, 200, &boot)
	if boot["support_qr_url"] != qr || boot["support_phone"] != "400-123-4567" || boot["support_wechat"] != "support_58it" {
		t.Fatalf("公开客服信息缺失:%v", boot)
	}
	for _, key := range []string{"merchant_id", "app_secret", "api_v3_key", "merchant_private_key"} {
		if _, ok := boot[key]; ok {
			t.Fatal("公开客服引入商户秘密")
		}
	}
	api.call("admin", "PATCH", "/admin/settings", map[string]string{"support_phone": "<script>alert(1)</script>"}, 400, nil)
	api.call("admin", "PATCH", "/admin/settings", map[string]string{"support_phone": "++"}, 400, nil)
	api.call("admin", "PATCH", "/admin/settings", map[string]string{"support_wechat": "bad\nwechat"}, 400, nil)
	api.call("admin", "PATCH", "/admin/settings", map[string]string{"merchant_private_key": "MUST-NOT-ENTER-SETTINGS"}, 400, nil)
	api.call("admin", "PATCH", "/admin/settings", map[string]string{"support_qr_url": "", "support_phone": "", "support_wechat": ""}, 200, &updated)
	if updated.SupportQRURL != "" || updated.SupportPhone != "" || updated.SupportWechat != "" || updated.SiteName != "集结测试平台" {
		t.Fatal("清除客服信息时改变了其它配置")
	}
}

// TestIntegrationClubSupportContactOwnership 验证二维码属于编辑本人且匹配当前组织用途，普通成员不能覆盖组织客服资料。
func TestIntegrationClubSupportContactOwnership(t *testing.T) {
	api := newTestAPI(t)
	valid := communityPNG(t, 256, 256)
	api.uploadCommunityImage("worker", "club_contact_qr", "shanhai", valid, 403)
	qr := api.uploadCommunityImage("clubowner", "club_contact_qr", "shanhai", valid, 200)
	foreign := api.uploadCommunityImage("admin", "club_contact_qr", "shanhai", valid, 200)
	draft := api.uploadCommunityImage("outsider", "club_contact_qr", "", valid, 200)
	avatar := api.uploadCommunityImage("clubowner", "avatar", "", valid, 200)
	api.call("worker", "PATCH", "/clubs/shanhai", map[string]string{"contact_phone": "17607080935"}, 403, nil)
	api.call("clubowner", "PATCH", "/clubs/shanhai", map[string]string{"contact_phone": "()"}, 400, nil)
	api.call("clubowner", "PATCH", "/clubs/shanhai", map[string]string{"contact_qr_url": foreign}, 403, nil)
	api.call("clubowner", "PATCH", "/clubs/shanhai", map[string]string{"contact_qr_url": draft}, 403, nil)
	api.call("clubowner", "PATCH", "/clubs/shanhai", map[string]string{"contact_qr_url": avatar}, 400, nil)
	api.call("admin", "PATCH", "/clubs/wild", map[string]string{"contact_qr_url": foreign}, 403, nil)
	var saved Club
	api.call("clubowner", "PATCH", "/clubs/shanhai", map[string]string{"contact_qr_url": qr, "contact_phone": "17607080935", "contact_wechat": "club_support"}, 200, &saved)
	if saved.ContactQRURL != qr || saved.ContactPhone != "17607080935" || saved.ContactWechat != "club_support" {
		t.Fatalf("组织客服资料未保存:%+v", saved)
	}
	api.call("admin", "PATCH", "/admin/clubs/shanhai", map[string]any{"status": "active", "verified": true}, 200, nil)
	var page struct {
		Club Club `json:"club"`
	}
	api.call("", "GET", "/clubs/shanhai", nil, 200, &page)
	if page.Club.ContactQRURL != qr || page.Club.ContactWechat != "club_support" {
		t.Fatal("公开组织主页缺少客服字段")
	}
	api.call("admin", "PATCH", "/clubs/shanhai", map[string]string{"description": "另一个管理员修改简介，保留原负责人的客服二维码"}, 200, &saved)
	if saved.ContactQRURL != qr {
		t.Fatal("修改其它组织字段丢失客服二维码")
	}
	api.call("clubowner", "PATCH", "/clubs/shanhai", map[string]string{"contact_phone": "400-100-2000"}, 200, &saved)
	if saved.ContactQRURL != qr || saved.ContactWechat != "club_support" {
		t.Fatal("部分更新清空未提交客服字段")
	}
	api.call("clubowner", "PATCH", "/clubs/shanhai", map[string]string{"contact_qr_url": "", "contact_wechat": ""}, 200, &saved)
	if saved.ContactQRURL != "" || saved.ContactWechat != "" || saved.ContactPhone != "400-100-2000" {
		t.Fatal("组织客服清除不正确")
	}
}

// TestIntegrationSupportContactUGCModeration 验证客服电话/微信走文字审查，二维码上传及旧图引用走实际字节图像审查。
func TestIntegrationSupportContactUGCModeration(t *testing.T) {
	api := newTestAPI(t)
	valid := communityPNG(t, 256, 256)
	qr := api.uploadCommunityImage("admin", "support_qr", "", valid, 200)
	clubQR := api.uploadCommunityImage("clubowner", "club_contact_qr", "shanhai", valid, 200)
	var calls atomic.Int32
	var imageSeen atomic.Bool
	installMockAI(t, api, func(r *http.Request) (*http.Response, error) {
		calls.Add(1)
		raw, _ := io.ReadAll(r.Body)
		if strings.Contains(string(raw), "data:image/png;base64,") {
			imageSeen.Store(true)
		}
		return aiMockResponse(`{"decision":"reject","reason":"客服信息包含不合规内容"}`), nil
	})
	api.App.Config.DevMode = false
	for _, c := range []struct {
		user, path string
		body       map[string]string
	}{{"admin", "/admin/settings", map[string]string{"support_phone": "17607080935"}}, {"admin", "/admin/settings", map[string]string{"support_wechat": "support_account"}}, {"clubowner", "/clubs/shanhai", map[string]string{"contact_phone": "17607080935"}}, {"clubowner", "/clubs/shanhai", map[string]string{"contact_wechat": "club_account"}}, {"admin", "/admin/settings", map[string]string{"support_qr_url": qr}}, {"clubowner", "/clubs/shanhai", map[string]string{"contact_qr_url": clubQR}}} {
		api.call(c.user, "PATCH", c.path, c.body, 200, nil)
	}
	api.uploadCommunityImage("admin", "support_qr", "", valid, 200)
	api.uploadCommunityImage("clubowner", "club_contact_qr", "shanhai", valid, 200)
	drainReviewAI(t, api)
	if calls.Load() < 4 || !imageSeen.Load() {
		t.Fatal("客服文字或实际图片未进入合规审核")
	}
	var settings Settings
	api.call("admin", "GET", "/admin/settings", nil, 200, &settings)
	if settings.SupportPhone != "" || settings.SupportWechat != "" || settings.SupportQRURL != rejectedImageURL {
		t.Fatal("审核拒绝仍写入了平台客服配置")
	}
	club, _ := get[Club](t.Context(), api.App.DB, "clubs", "shanhai", false)
	if club.ContactQRURL != rejectedImageURL || club.ContactPhone != "" || club.ContactWechat != "" {
		t.Fatal("审核拒绝仍写入了组织客服配置")
	}
}

// TestSupportQRImageNormalization 验证透明客服二维码合成纯白底且码点逐像素不变，头像用途仍保留透明。
func TestSupportQRImageNormalization(t *testing.T) {
	raw, e := os.ReadFile("testdata/support-qr-transparent.png")
	if e != nil {
		t.Fatal(e)
	}
	original, _, e := image.Decode(bytes.NewReader(raw))
	if e != nil {
		t.Fatal(e)
	}
	for _, purpose := range []string{"support_qr", "club_contact_qr"} {
		normalized, mime, cfg, e := normalizePublicImage(raw, purpose)
		if e != nil || mime != "image/png" || cfg.Width != original.Bounds().Dx() || cfg.Height != original.Bounds().Dy() {
			t.Fatalf("二维码规范化改变尺寸或格式:%s %+v %v", mime, cfg, e)
		}
		decoded, _, e := image.Decode(bytes.NewReader(normalized))
		if e != nil {
			t.Fatal(e)
		}
		for y := 0; y < cfg.Height; y++ {
			for x := 0; x < cfg.Width; x++ {
				_, _, _, originalAlpha := original.At(x, y).RGBA()
				red, green, blue, alpha := decoded.At(x, y).RGBA()
				expected := uint32(65535)
				if originalAlpha == 65535 {
					expected = 0
				}
				if red != expected || green != expected || blue != expected || alpha != 65535 {
					t.Fatalf("二维码码点或静区发生损失:%s %d,%d", purpose, x, y)
				}
			}
		}
		if purpose == "support_qr" {
			if target := os.Getenv("SUPPORT_QR_ACCEPTANCE_OUTPUT"); target != "" {
				if e = os.WriteFile(target, normalized, 0600); e != nil {
					t.Fatal(e)
				}
			}
		}
	}
	avatar, _, _, e := normalizePublicImage(raw, "avatar")
	if e != nil {
		t.Fatal(e)
	}
	decoded, _, e := image.Decode(bytes.NewReader(avatar))
	if e != nil {
		t.Fatal(e)
	}
	_, _, _, alpha := decoded.At(0, 0).RGBA()
	if alpha != 0 {
		t.Fatal("客服二维码规范化影响了普通头像透明度")
	}
}

// TestIntegrationSupportQRStoredBytes 验证审核、存储、公开预览及SHA256均使用同一份白底无损图片字节。
func TestIntegrationSupportQRStoredBytes(t *testing.T) {
	api := newTestAPI(t)
	var auditMu sync.Mutex
	var auditedBytes []byte
	installMockAI(t, api, func(r *http.Request) (*http.Response, error) {
		var payload struct {
			Messages []struct {
				Content json.RawMessage `json:"content"`
			} `json:"messages"`
		}
		if e := json.NewDecoder(r.Body).Decode(&payload); e != nil {
			t.Error(e)
		}
		for _, message := range payload.Messages {
			var parts []struct {
				Type     string `json:"type"`
				ImageURL struct {
					URL string `json:"url"`
				} `json:"image_url"`
			}
			if json.Unmarshal(message.Content, &parts) != nil {
				continue
			}
			for _, part := range parts {
				if part.Type == "image_url" {
					data, e := base64.StdEncoding.DecodeString(strings.TrimPrefix(part.ImageURL.URL, "data:image/png;base64,"))
					if e != nil {
						t.Error(e)
					}
					auditMu.Lock()
					auditedBytes = append([]byte{}, data...)
					auditMu.Unlock()
				}
			}
		}
		return aiMockResponse(`{"decision":"allow","reason":""}`), nil
	})
	api.App.Config.DevMode = false
	raw, e := os.ReadFile("testdata/support-qr-transparent.png")
	if e != nil {
		t.Fatal(e)
	}
	for _, v := range []struct{ user, purpose, club string }{{"admin", "support_qr", ""}, {"clubowner", "club_contact_qr", "shanhai"}} {
		url := api.uploadCommunityImage(v.user, v.purpose, v.club, raw, 200)
		drainReviewAI(t, api)
		match := publicImagePath.FindStringSubmatch(url)
		stored, e := get[PublicImage](t.Context(), api.App.DB, "evidence", match[1], false)
		if e != nil {
			t.Fatal(e)
		}
		auditMu.Lock()
		sameAuditedBytes := bytes.Equal(auditedBytes, stored.Data)
		auditMu.Unlock()
		if !sameAuditedBytes || stored.Moderation != "ai" {
			t.Fatal("模型审核字节与最终存储的二维码不一致")
		}
		sum := sha256.Sum256(stored.Data)
		if stored.SHA256 != hex.EncodeToString(sum[:]) || stored.MIME != "image/png" {
			t.Fatal("二维码存储摘要或格式不对应实际字节")
		}
		decoded, _, e := image.Decode(bytes.NewReader(stored.Data))
		if e != nil || !publicImageOpaque(decoded) {
			t.Fatal("实际存储的客服二维码仍含透明像素")
		}
		red, green, blue, alpha := decoded.At(0, 0).RGBA()
		if red != 65535 || green != 65535 || blue != 65535 || alpha != 65535 {
			t.Fatal("微信预览图未包含白色静区")
		}
		response, e := api.Server.Client().Get(api.Server.URL + url)
		if e != nil {
			t.Fatal(e)
		}
		body, e := io.ReadAll(response.Body)
		response.Body.Close()
		if e != nil || response.StatusCode != 200 || !bytes.Equal(body, stored.Data) {
			t.Fatal("原生预览读取字节与已审核存储字节不一致")
		}
	}
}
