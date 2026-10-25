// Package app 为管理列表生成白名单搜索条件，先在数据库搜索再分页且参数全部绑定。
package app

import (
	"fmt"
	"net/http"
	"strings"
)

// adminQuery 按不同资源的业务字段搜索，避免只在已加载的一百条记录内过滤。
func adminQuery(r *http.Request, resource string) (string, []any, error) {
	fields := map[string][]string{
		"users":    {"id", "nickname", "bio", "status", "role"},
		"services": {"id", "game_id", "title", "description", "category", "provider_id", "provider_name", "club_id", "club_name", "status"},
		"clubs":    {"id", "name", "description", "owner_id", "owner_name", "status"},
		"orders":   {"id", "order_no", "title", "game_id", "game_name", "buyer_id", "buyer_name", "provider_id", "provider_name", "club_id", "status", "refund_status", "requirements"},
		"audit":    {"id", "actor_id", "action", "target_id", "note"},
		"games":    {"id", "name", "short_name", "subtitle"},
	}
	keys, ok := fields[resource]
	if !ok {
		return "", nil, fail(404, "管理资源不存在")
	}
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if !length(q, 0, 100) {
		return "", nil, fail(400, "搜索关键词最多 100 字")
	}
	limit, offset := pageLimit(r)
	args := []any{limit, offset}
	where := ""
	if q != "" {
		parts := make([]string, 0, len(keys))
		for _, field := range keys {
			parts = append(parts, "data->>'"+field+"'")
		}
		where = "WHERE (strpos(lower(concat_ws(' '," + strings.Join(parts, ",") + ")),lower($3))>0"
		args = append(args, q)
		currentNames := map[string]string{"services": "EXISTS(SELECT 1 FROM users u WHERE u.id=services.data->>'provider_id' AND strpos(lower(u.data->>'nickname'),lower($3))>0) OR EXISTS(SELECT 1 FROM clubs c WHERE c.id=services.data->>'club_id' AND strpos(lower(c.data->>'name'),lower($3))>0)", "clubs": "EXISTS(SELECT 1 FROM users u WHERE u.id=clubs.data->>'owner_id' AND strpos(lower(u.data->>'nickname'),lower($3))>0)", "orders": "EXISTS(SELECT 1 FROM users u WHERE (u.id=orders.data->>'buyer_id' OR u.id=orders.data->>'provider_id') AND strpos(lower(u.data->>'nickname'),lower($3))>0)", "audit": "EXISTS(SELECT 1 FROM users u WHERE u.id=audit.data->>'actor_id' AND strpos(lower(u.data->>'nickname'),lower($3))>0)"}
		if extra := currentNames[resource]; extra != "" {
			where += " OR " + extra
		}
		if resource == "users" {
			where += " OR EXISTS(SELECT 1 FROM identities phone_identity WHERE phone_identity.id=users.id AND strpos(phone_identity.data->>'phone_number',$3)>0)"
		}
		where += ") "
	}
	statuses := map[string][]string{"services": {"pending", "active", "paused", "rejected"}, "clubs": {"pending", "active", "rejected"}, "users": {"active", "frozen"}}
	if allowed, filtered := statuses[resource]; filtered {
		status := strings.TrimSpace(r.URL.Query().Get("status"))
		if status != "" && status != "all" {
			if !contains(allowed, status) {
				return "", nil, fail(400, "管理列表状态筛选无效")
			}
			if where == "" {
				where = "WHERE "
			} else {
				where += "AND "
			}
			args = append(args, status)
			where += fmt.Sprintf("data->>'status'=$%d ", len(args))
		}
	}
	if resource == "orders" && r.URL.Query().Get("refund") == "1" {
		if where == "" {
			where = "WHERE "
		} else {
			where += "AND "
		}
		where += "(data->>'refund_status' IN ('requested','escalated','processing') OR data->>'status'='disputed') "
	}
	order := "data->>'created_at' DESC, id DESC"
	if resource == "games" {
		order = "(data->>'sort_order')::integer ASC, id ASC"
	}
	return where + "ORDER BY " + order + " LIMIT $1 OFFSET $2", args, nil
}
