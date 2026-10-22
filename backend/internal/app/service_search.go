// Package app 在数据库内完成公开服务的可见性过滤、全文字面搜索、稳定排序与分页。
package app

import (
	"net/http"
	"strconv"
	"strings"
)

// serviceQuery 先剔除不可售服务再排序分页，避免只对首屏排序或因隐藏商品导致空页。
func serviceQuery(r *http.Request, ownerID string) (string, []any, error) {
	orders := map[string]string{
		"recommended": "(data->>'created_at')::timestamptz DESC, id DESC",
		"price":       "(data->>'price_cents')::bigint ASC, id ASC",
		"completed":   "COALESCE((data->>'completed_count')::integer,0) DESC, id ASC",
	}
	sortName := r.URL.Query().Get("sort")
	if sortName == "" {
		sortName = "recommended"
	}
	order, ok := orders[sortName]
	if !ok {
		return "", nil, fail(400, "服务排序仅支持 recommended、price、completed")
	}
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	if !length(query, 0, 100) {
		return "", nil, fail(400, "搜索关键词最多 100 字")
	}
	limit, offset := pageLimit(r)
	args := []any{limit, offset}
	conditions := []string{}
	if ownerID != "" {
		args = append(args, ownerID)
		conditions = append(conditions, "data->>'provider_id'=$3")
	} else {
		conditions = append(conditions, "data->>'status'='active'", "EXISTS(SELECT 1 FROM games g WHERE g.id=services.data->>'game_id' AND g.data->>'enabled'='true')", "EXISTS(SELECT 1 FROM users u WHERE u.id=services.data->>'provider_id' AND u.data->>'status'='active')")
	}
	for _, key := range []string{"game_id", "category"} {
		if value := r.URL.Query().Get(key); value != "" {
			args = append(args, value)
			conditions = append(conditions, "data->>'"+key+"'=$"+strconv.Itoa(len(args)))
		}
	}
	if query != "" {
		args = append(args, query)
		param := "$" + strconv.Itoa(len(args))
		conditions = append(conditions, "(strpos(lower(concat_ws(' ',data->>'title',data->>'description',data->>'category',data->>'game_id',data->>'tags')),lower("+param+"))>0 OR EXISTS(SELECT 1 FROM games g WHERE g.id=services.data->>'game_id' AND strpos(lower(concat_ws(' ',g.data->>'name',g.data->>'short_name')),lower("+param+"))>0))")
	}
	return "WHERE " + strings.Join(conditions, " AND ") + " ORDER BY " + order + " LIMIT $1 OFFSET $2", args, nil
}
