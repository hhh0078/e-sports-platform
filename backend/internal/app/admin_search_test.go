// Package app 验证后台待办状态在数据库中先筛选再分页，非法状态不能拼接SQL或泄露全部列表。
package app

import (
	"fmt"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"testing"
)

// TestAdminQueryStatusBinding 验证各资源允许状态枚举、搜索组合占位符、全部状态与非法输入。
func TestAdminQueryStatusBinding(t *testing.T) {
	allowed := map[string][]string{"services": {"pending", "active", "paused", "rejected"}, "clubs": {"pending", "active", "rejected"}, "users": {"active", "frozen"}}
	for resource, statuses := range allowed {
		for _, status := range statuses {
			t.Run(resource+"/"+status, func(t *testing.T) {
				for _, search := range []string{"", "参数化搜索"} {
					query := url.Values{"status": {status}, "limit": {"2"}, "offset": {"3"}}
					expected := []any{2, 3, status}
					placeholder := "$3"
					if search != "" {
						query.Set("q", search)
						expected = []any{2, 3, search, status}
						placeholder = "$4"
					}
					sql, args, err := adminQuery(httptest.NewRequest("GET", "/admin/"+resource+"?"+query.Encode(), nil), resource)
					if err != nil || !reflect.DeepEqual(args, expected) || !strings.Contains(sql, "data->>'status'="+placeholder) || !strings.HasSuffix(sql, "LIMIT $1 OFFSET $2") {
						t.Fatalf("未绑定搜索/状态/分页:%q %#v %v", sql, args, err)
					}
				}
			})
		}
		for _, status := range []string{"", "all"} {
			sql, args, err := adminQuery(httptest.NewRequest("GET", "/admin/"+resource+"?status="+status, nil), resource)
			if err != nil || len(args) != 2 || strings.Contains(sql, "data->>'status'=") {
				t.Fatal("全部列表误加状态约束")
			}
		}
		for _, status := range []string{"unknown", "paid", "pending' OR 1=1 --"} {
			_, _, err := adminQuery(httptest.NewRequest("GET", "/admin/"+resource+"?status="+url.QueryEscape(status), nil), resource)
			if err == nil {
				t.Fatal("无效状态未拒绝", resource, status)
			}
		}
	}
}

// TestIntegrationAdminStatusBeforePagination 验证待办能命中超过一百条更新记录后的目标，且组合关键词与分页均由SQL完成。
func TestIntegrationAdminStatusBeforePagination(t *testing.T) {
	api := newTestAPI(t)
	for i := 0; i < 103; i++ {
		id := fmt.Sprintf("admin-status-%03d", i)
		stamp := "2030-01-01T00:00:00Z"
		user := User{ID: id, Nickname: "分页待办检索", Status: "active", CreatedAt: stamp}
		service := Service{ID: id, Title: "分页待办检索", ProviderID: "worker", GameID: "delta", Status: "active", CreatedAt: stamp}
		club := Club{ID: id, Name: "分页待办检索", OwnerID: "clubowner", Status: "active", CreatedAt: stamp}
		if i >= 101 {
			stamp = "2020-01-01T00:00:00Z"
			user.Status = "frozen"
			user.CreatedAt = stamp
			service.Status = "pending"
			service.CreatedAt = stamp
			club.Status = "pending"
			club.CreatedAt = stamp
		}
		if err := put(t.Context(), api.App.DB, "users", id, user); err != nil {
			t.Fatal(err)
		}
		if err := put(t.Context(), api.App.DB, "services", id, service); err != nil {
			t.Fatal(err)
		}
		if err := put(t.Context(), api.App.DB, "clubs", id, club); err != nil {
			t.Fatal(err)
		}
	}
	for resource, status := range map[string]string{"users": "frozen", "services": "pending", "clubs": "pending"} {
		for _, actor := range []string{"", "player", "worker", "clubowner"} {
			code := 403
			if actor == "" {
				code = 401
			}
			api.call(actor, "GET", "/admin/"+resource+"?status="+status, nil, code, nil)
		}
		var first, second []map[string]any
		route := "/admin/" + resource + "?status=" + status + "&q=" + url.QueryEscape("分页待办检索") + "&limit=1"
		api.call("admin", "GET", route, nil, 200, &first)
		api.call("admin", "GET", route+"&offset=1", nil, 200, &second)
		if len(first) != 1 || len(second) != 1 || first[0]["status"] != status || second[0]["status"] != status || first[0]["id"] == second[0]["id"] {
			t.Fatalf("状态或分页错误:%s %#v %#v", resource, first, second)
		}
		var none []map[string]any
		api.call("admin", "GET", route+"&offset=2", nil, 200, &none)
		if len(none) != 0 {
			t.Fatal("分页越界应为空")
		}
		api.call("admin", "GET", "/admin/"+resource+"?status="+url.QueryEscape("pending' OR 1=1 --"), nil, 400, nil)
	}
}
