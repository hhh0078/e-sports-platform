// Package app 统一屏蔽被拒绝的交易图文展示，同时保留数据库原始证据和资金幂等事实。
package app

import (
	"context"
	"encoding/json"
	"strings"
)

// reviewedObjectTarget 从服务端输出形状识别白名单领域对象，不接受客户端提供的表名。
func reviewedObjectTarget(v map[string]any) (string, string) {
	if target := reviewString(v, "target_id"); target != "" {
		if table := map[string]string{"service": "services", "request": "orders", "player": "users", "club": "clubs"}[reviewString(v, "kind")]; table != "" {
			return table, target
		}
	}
	id := reviewString(v, "id")
	if id == "" {
		if reviewString(v, "user_id") != "" && reviewString(v, "club_id") != "" {
			return "members", reviewString(v, "club_id") + ":" + reviewString(v, "user_id")
		}
		return "", ""
	}
	if _, ok := v["order_no"]; ok {
		return "orders", id
	}
	if _, ok := v["amount_cents"]; ok {
		if _, hasRequirements := v["requirements"]; hasRequirements {
			return "orders", id
		}
	}
	if _, ok := v["sender_role"]; ok {
		return "messages", id
	}
	if _, ok := v["rating"]; ok {
		if _, exists := v["order_id"]; exists {
			return "reviews", id
		}
	}
	if _, ok := v["proposer_id"]; ok {
		return "order_adjustments", id
	}
	if _, ok := v["price_cents"]; ok {
		return "services", id
	}
	if _, ok := v["game_ids"]; ok {
		return "clubs", id
	}
	if _, ok := v["nickname"]; ok {
		return "users", id
	}
	return "", ""
}

// walkReviewObjects 访问JSON响应中的领域对象，并收集快照引用的源记录ID。
func walkReviewObjects(value any, visit func(map[string]any, string, string), ids map[string]bool) {
	switch v := value.(type) {
	case map[string]any:
		table, id := reviewedObjectTarget(v)
		if id != "" {
			ids[id] = true
			visit(v, table, id)
		}
		for _, key := range []string{"order_id", "game_id", "proposer_id", "provider_id", "buyer_id", "owner_id", "sender_id", "service_id", "club_id", "user_id"} {
			if linked := reviewString(v, key); linked != "" {
				ids[linked] = true
			}
		}
		for _, item := range v {
			walkReviewObjects(item, visit, ids)
		}
	case []any:
		for _, item := range v {
			walkReviewObjects(item, visit, ids)
		}
	}
}

// projectReviewedValue 用拒绝记录生成安全响应投影，原订单、聊天及调价原因不被物理覆盖。
func (a *App) projectReviewedValue(ctx context.Context, value any) (any, error) {
	state, _ := ctx.Value(reviewContextKey{}).(*reviewRequest)
	if state != nil && state.RawAdminReview {
		return value, nil
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	var out any
	if err = json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	ids := map[string]bool{}
	orderIDs := map[string]bool{}
	walkReviewObjects(out, func(object map[string]any, table, id string) {
		if table == "orders" {
			orderIDs[id] = true
		}
		if order := reviewString(object, "order_id"); order != "" {
			orderIDs[order] = true
		}
	}, ids)
	// 只读取当前响应涉及的订单来源标识；原始支付事实、实名和身份秘密均不参与投影查询。
	if len(orderIDs) > 0 {
		orders := []string{}
		for id := range orderIDs {
			orders = append(orders, id)
		}
		rows, e := a.DB.Query(ctx, `SELECT COALESCE(data->>'buyer_id',''),COALESCE(data->>'provider_id',''),COALESCE(data->>'service_id',''),COALESCE(data->>'club_id',''),COALESCE(data->>'game_id','') FROM orders WHERE id=ANY($1)`, orders)
		if e != nil {
			return nil, e
		}
		for rows.Next() {
			var buyer, provider, service, club, game string
			if e = rows.Scan(&buyer, &provider, &service, &club, &game); e != nil {
				rows.Close()
				return nil, e
			}
			for _, id := range []string{buyer, provider, service, club, game} {
				if id != "" {
					ids[id] = true
				}
			}
		}
		e = rows.Err()
		rows.Close()
		if e != nil {
			return nil, e
		}
	}

	if len(ids) == 0 {
		return out, nil
	}
	keys := []string{}
	for key := range ids {
		keys = append(keys, key)
	}
	targets, err := list[moderationTarget](ctx, a.DB, "moderation_targets", "WHERE data->>'target_id'=ANY($1) AND data->>'status'='rejected'", keys)
	if err != nil {
		return nil, err
	}
	byTarget := map[string][]moderationTarget{}
	for _, target := range targets {
		key := target.TargetType + ":" + target.TargetID
		byTarget[key] = append(byTarget[key], target)
	}
	walkReviewObjects(out, func(object map[string]any, table, id string) {
		for _, target := range byTarget[table+":"+id] {
			setReviewPath(object, target.Path, target.Replacement)
		}
	}, map[string]bool{})
	// 不可变订单昵称、商品快照及系统事件可能复制已拒绝的源文字，显示时同步屏蔽旧片段。
	tasks, err := list[ModerationTask](ctx, a.DB, "moderation_tasks", `WHERE (data->>'target_id'=ANY($1) OR data->>'order_id'=ANY($1)) AND data->>'status'='rejected' AND data->>'kind'='text' AND NOT EXISTS (SELECT 1 FROM moderation_targets mt WHERE mt.data->>'target_type'=moderation_tasks.data->>'target_type' AND mt.data->>'target_id'=moderation_tasks.data->>'target_id' AND mt.data->>'field_code'=moderation_tasks.data->>'field_code' AND mt.data->>'value_hash'=moderation_tasks.data->>'value_hash' AND mt.data->>'status'='allowed' AND (mt.data->>'version')::bigint>(moderation_tasks.data->>'content_version')::bigint)`, keys)
	if err != nil {
		return nil, err
	}
	blocked := []string{}
	for _, task := range tasks {
		if task.Preview != "" && task.Preview != rejectedContentText {
			blocked = append(blocked, task.Preview)
		}
	}
	// 服务分类来自游戏枚举；只定向屏蔽该游戏已拒分类，不对可变服务正文做历史全文替换。
	deniedCategories := map[string]map[string]bool{}
	for _, task := range tasks {
		if task.TargetType == "games" && strings.HasPrefix(task.FieldCode, "categories.") {
			if deniedCategories[task.TargetID] == nil {
				deniedCategories[task.TargetID] = map[string]bool{}
			}
			deniedCategories[task.TargetID][task.Preview] = true
		}
	}
	walkReviewObjects(out, func(object map[string]any, table, id string) {
		if table == "services" && deniedCategories[reviewString(object, "game_id")][reviewString(object, "category")] {
			object["category"] = rejectedContentText
		}
	}, map[string]bool{})
	maskReviewSnapshots(out, blocked)
	return out, nil
}

// maskReviewSnapshots 仅替换可见文本字段中的拒绝片段，绝不改变ID、签名、状态或金额。
func maskReviewSnapshots(value any, blocked []string) {
	fields := map[string]bool{"nickname": true, "provider_name": true, "proposer_name": true, "buyer_name": true, "sender_name": true, "owner_name": true, "club_name": true, "game_name": true, "unit": true, "title": true, "description": true, "service_description": true, "requirements": true, "content": true, "delivery_note": true, "refund_reason": true, "refund_reply": true, "dispute_reason": true, "reason": true, "applied_assessment": true, "applied_join_note": true}
	switch v := value.(type) {
	case map[string]any:
		table, _ := reviewedObjectTarget(v)
		mutable := table == "users" || table == "services" || table == "clubs"
		for key, item := range v {
			if text, ok := item.(string); ok && fields[key] && !mutable {
				for _, bad := range blocked {
					text = strings.ReplaceAll(text, bad, rejectedContentText)
				}
				v[key] = text
			} else {
				maskReviewSnapshots(item, blocked)
			}
		}
	case []any:
		for _, item := range v {
			maskReviewSnapshots(item, blocked)
		}
	}
}
