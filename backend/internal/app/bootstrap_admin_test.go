// Package app 验证首次管理员只来自私密配置指定的官方微信身份，且一次性授予不会形成后门。
package app

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

// TestIntegrationBootstrapWechatAdmin 验证访客不能抢占管理员，精确匹配可授予且降权后不会自动重授。
func TestIntegrationBootstrapWechatAdmin(t *testing.T) {
	t.Setenv("BOOTSTRAP_ADMIN_OPENID", "private-owner-openid")
	api := newTestAPI(t)
	mockWechat(t, api, func(w http.ResponseWriter, r *http.Request) {
		openid := "other-user-openid"
		switch r.URL.Query().Get("js_code") {
		case "owner":
			openid = "private-owner-openid"
		case "case-mismatch":
			openid = "Private-owner-openid"
		}
		json.NewEncoder(w).Encode(map[string]string{"openid": openid})
	})
	// login 获取官方短票据并主动同意当前协议，模拟完整正式登录路径。
	login := func(code string) User {
		t.Helper()
		var prepared struct {
			Ticket     string       `json:"ticket"`
			Agreements agreementSet `json:"agreements"`
		}
		api.call("", "POST", "/auth/wechat/prepare", map[string]string{"code": code}, 200, &prepared)
		var result struct {
			User  User   `json:"user"`
			Token string `json:"token"`
		}
		api.call("", "POST", "/auth/wechat", loginConsent(prepared.Ticket, prepared.Agreements), 200, &result)
		api.Tokens[code] = result.Token
		return result.User
	}
	if login("other").Role != "user" || login("case-mismatch").Role != "user" {
		t.Fatal("非精确匹配身份获得管理员")
	}
	owner := login("owner")
	if owner.Role != "admin" {
		t.Fatal("指定身份未获首次管理员")
	}
	api.call("owner", "GET", "/admin/overview", nil, 200, nil)
	code, raw, err := api.request("owner", "GET", "/admin/audit", nil)
	if err != nil || code != 200 || strings.Contains(string(raw), "private-owner-openid") {
		t.Fatal("管理员审计泄漏openid")
	}
	marker, err := get[map[string]string](t.Context(), api.App.DB, "settings", "bootstrap-admin", false)
	if err != nil || marker["user_id"] != owner.ID || marker["assigned_at"] == "" {
		t.Fatal("首次授予缺少不可重放记录")
	}
	owner.Role = "user"
	if err = put(t.Context(), api.App.DB, "users", owner.ID, owner); err != nil {
		t.Fatal(err)
	}
	if login("owner").Role != "user" {
		t.Fatal("已经降权的用户被旧环境配置重新提权")
	}
	api.call("owner", "GET", "/admin/overview", nil, 403, nil)
	var count int
	if err = api.App.DB.QueryRow(t.Context(), "SELECT count(*) FROM audit WHERE data->>'action'='admin.bootstrap'").Scan(&count); err != nil || count != 1 {
		t.Fatal("管理员初始化未严格执行一次")
	}
}
