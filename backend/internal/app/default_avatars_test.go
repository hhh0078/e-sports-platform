// Package app 验证动物头像的白名单、微信分配幂等以及仅迁移已核验旧默认图的边界。
package app

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"jijie/internal/assets"
)

// TestDefaultAvatarTrustBoundary 验证内置图仅能用于头像，外部URL、目录路径和未知图片不享有审核豁免。
func TestDefaultAvatarTrustBoundary(t *testing.T) {
	for _, path := range assets.DefaultAvatarPaths() {
		if !trustedDefaultAvatar(path, "avatar") || trustedDefaultAvatar(path, "profile_cover") || trustedDefaultAvatar(path, "club_cover") {
			t.Fatalf("内置素材用途检查错误：%s", path)
		}
		if err := validateOwnedImage(t.Context(), nil, path, "avatar", "worker", "", User{}); err != nil {
			t.Fatalf("正常默认头像无法保存：%v", err)
		}
		if err := (&App{}).moderateOwnedImage(httptest.NewRequest("POST", "/", nil), User{}, path, "avatar", "worker", ""); err != nil {
			t.Fatalf("已核验内置头像不应发给外部模型：%v", err)
		}
	}
	for _, path := range []string{"/static/avatars/unknown.png", "/static/avatars/cat_face.png?bypass=1", "https://example.com/static/avatars/cat_face.png", "/static/avatars/../cat_face.png", "/static/avatars/LICENSE"} {
		if trustedDefaultAvatar(path, "avatar") {
			t.Fatalf("越权路径获得信任：%s", path)
		}
		if err := validateOwnedImage(t.Context(), nil, path, "avatar", "worker", "", User{}); err == nil {
			t.Fatalf("越权路径被保存：%s", path)
		}
	}
	for i := 0; i < 60; i++ {
		path, err := randomDefaultAvatar()
		if err != nil || !assets.IsDefaultAvatar(path) {
			t.Fatalf("随机分配越界：%s %v", path, err)
		}
	}
	if defaultClubAvatar("club-1") != defaultClubAvatar("club-1") || !assets.IsDefaultAvatar(defaultClubAvatar("club-1")) {
		t.Fatal("俱乐部默认头像不稳定或不属于清单")
	}
}

// TestIntegrationDefaultAvatarWechatRegistration 验证首次登录分配头像、再次登录保留用户修改后的头像。
func TestIntegrationDefaultAvatarWechatRegistration(t *testing.T) {
	api := newTestAPI(t)
	mockWechat(t, api, func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{"openid": "avatar-registration-user"})
	})
	var prepared struct {
		Ticket     string       `json:"ticket"`
		Agreements agreementSet `json:"agreements"`
	}
	api.call("", "POST", "/auth/wechat/prepare", map[string]string{"code": "avatar-first"}, 200, &prepared)
	var login struct {
		Token string `json:"token"`
		User  User   `json:"user"`
	}
	api.call("", "POST", "/auth/wechat", loginConsent(prepared.Ticket, prepared.Agreements), 200, &login)
	if !assets.IsDefaultAvatar(login.User.Avatar) {
		t.Fatal("新注册未持久化默认动物头像")
	}
	api.Tokens["new-avatar-user"] = login.Token
	changed := assets.DefaultAvatarPaths()[0]
	if changed == login.User.Avatar {
		changed = assets.DefaultAvatarPaths()[1]
	}
	api.call("new-avatar-user", "PATCH", "/me/profile", map[string]string{"avatar": changed}, 200, nil)
	api.call("", "POST", "/auth/wechat/prepare", map[string]string{"code": "avatar-again"}, 200, &prepared)
	api.call("", "POST", "/auth/wechat", loginConsent(prepared.Ticket, prepared.Agreements), 200, &login)
	if login.User.Avatar != changed {
		t.Fatal("重复微信登录覆盖了用户已有头像")
	}
}

// TestIntegrationDefaultAvatarMigration 验证迁移只补空值和准确旧几何图，保留自定义内容、演示标记及所有未知字段。
func TestIntegrationDefaultAvatarMigration(t *testing.T) {
	api := newTestAPI(t)
	if _, err := api.App.DB.Exec(t.Context(), "DELETE FROM settings WHERE id='default-avatars-fluent-v1'"); err != nil {
		t.Fatal(err)
	}
	for _, item := range []struct {
		index   int
		altered bool
	}{{0, false}, {1, true}} {
		raw, err := legacyShowcaseAvatar(item.index)
		if err != nil {
			t.Fatal(err)
		}
		if item.altered {
			raw, err = showcaseAvatar(item.index)
			if err != nil {
				t.Fatal(err)
			}
		}
		sum := sha256.Sum256(raw)
		id := showcaseID("user", item.index)
		image := PublicImage{Evidence: Evidence{ID: showcaseID("avatar", item.index), OwnerID: id, Public: true, MIME: "image/png", Data: raw, SHA256: hex.EncodeToString(sum[:])}, Purpose: "avatar", Width: 256, Height: 256}
		imageRaw, err := json.Marshal(image)
		if err != nil {
			t.Fatal(err)
		}
		var markedImage map[string]any
		if err = json.Unmarshal(imageRaw, &markedImage); err != nil {
			t.Fatal(err)
		}
		markedImage["is_demo"], markedImage["demo_version"] = true, 1
		if err := put(t.Context(), api.App.DB, "evidence", image.ID, markedImage); err != nil {
			t.Fatal(err)
		}
		if err := put(t.Context(), api.App.DB, "users", id, map[string]any{"id": id, "avatar": "/api/media/" + image.ID, "is_demo": true, "demo_version": 1, "custom_field": "keep"}); err != nil {
			t.Fatal(err)
		}
	}
	for _, record := range []struct {
		table, id string
		value     map[string]any
	}{
		{"users", "empty-avatar", map[string]any{"id": "empty-avatar", "avatar": "", "custom_field": "keep"}},
		{"users", "custom-avatar", map[string]any{"id": "custom-avatar", "avatar": "/api/media/0123456789abcdef0123456789abcdef", "custom_field": "keep"}},
		{"clubs", "empty-club-avatar", map[string]any{"id": "empty-club-avatar", "avatar_url": "", "custom_field": "keep"}},
		{"clubs", "custom-club-avatar", map[string]any{"id": "custom-club-avatar", "avatar_url": "/api/media/abcdef0123456789abcdef0123456789", "custom_field": "keep"}},
	} {
		if err := put(t.Context(), api.App.DB, record.table, record.id, record.value); err != nil {
			t.Fatal(err)
		}
	}
	for attempt := 0; attempt < 2; attempt++ {
		tx, err := api.App.DB.Begin(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		if err = migrateDefaultAvatars(t.Context(), tx); err != nil {
			tx.Rollback(t.Context())
			t.Fatal(err)
		}
		if err = tx.Commit(t.Context()); err != nil {
			t.Fatal(err)
		}
	}
	for _, scenario := range []struct{ table, id, field, want string }{
		{"users", showcaseID("user", 0), "avatar", stableDefaultAvatar("user", showcaseID("user", 0))},
		{"users", showcaseID("user", 1), "avatar", "/api/media/" + showcaseID("avatar", 1)},
		{"users", "empty-avatar", "avatar", stableDefaultAvatar("user", "empty-avatar")},
		{"users", "custom-avatar", "avatar", "/api/media/0123456789abcdef0123456789abcdef"},
		{"clubs", "empty-club-avatar", "avatar_url", defaultClubAvatar("empty-club-avatar")},
		{"clubs", "custom-club-avatar", "avatar_url", "/api/media/abcdef0123456789abcdef0123456789"},
	} {
		value, err := get[map[string]any](t.Context(), api.App.DB, scenario.table, scenario.id, false)
		if err != nil || value[scenario.field] != scenario.want || value["custom_field"] != "keep" {
			t.Fatalf("迁移覆盖自定义或遗漏空值：%s/%s %+v %v", scenario.table, scenario.id, value, err)
		}
		if scenario.id == showcaseID("user", 0) && (value["is_demo"] != true || value["demo_version"] != float64(1)) {
			t.Fatal("头像迁移丢失演示隔离标记")
		}
	}
}
