// Package app 为六类微信模板读取当前安全展示字段，不复制私密手机号、退款原文或支付凭证到通知。
package app

import (
	"context"
	"net/url"
	"strings"
	"time"
)

// subscriptionBusinessTime 将真实业务时间格式化为上海时间；缺失或异常时间保持为空，不编造下单时间。
func subscriptionBusinessTime(value string) string {
	stamp, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return ""
	}
	return stamp.In(time.FixedZone("Asia/Shanghai", 8*3600)).Format("2006-01-02 15:04:05")
}

// subscriptionDisplayText 读取投影后的名称，审核失败或删除内容使用中性说明，不暴露原始被拒文字。
func subscriptionDisplayText(value any, field, fallback string) string {
	object, _ := value.(map[string]any)
	text := strings.TrimSpace(reviewString(object, field))
	if text == "" || strings.Contains(text, rejectedContentText) {
		return fallback
	}
	return strings.Join(strings.Fields(text), " ")
}

// populateSubscriptionValues 在真实发送前重读公开对象并应用审核投影，事件金额仍使用入队时核验的快照。
func (a *App) populateSubscriptionValues(ctx context.Context, q querier, delivery *subscriptionStoredDelivery) error {
	if delivery.Values == nil {
		delivery.Values = map[string]string{}
	}
	target, err := url.Parse(delivery.ActionPath)
	if err != nil {
		return nil
	}
	id, actorID, createdAt := target.Query().Get("id"), "", ""
	objects := map[string]any{}
	if delivery.Scene == "club_member" || delivery.Scene == "admin_club" {
		club, err := get[Club](ctx, q, "clubs", id, false)
		if err != nil {
			if missing(err) {
				return nil
			}
			return err
		}
		objects["club"] = map[string]any{"id": club.ID, "name": club.Name, "game_ids": club.GameIDs}
		actorID, createdAt = club.OwnerID, club.CreatedAt
		if delivery.Scene == "club_member" {
			actorID = target.Query().Get("member_id")
			member, err := get[ClubMember](ctx, q, "members", id+":"+actorID, false)
			if err != nil && !missing(err) {
				return err
			}
			createdAt = member.CreatedAt
		}
	} else {
		order, err := get[Order](ctx, q, "orders", id, false)
		if err != nil {
			if missing(err) {
				return nil
			}
			return err
		}
		objects["order"] = map[string]any{"id": order.ID, "order_no": order.OrderNo, "title": order.Title, "service_id": order.ServiceID, "buyer_id": order.BuyerID, "provider_id": order.ProviderID, "club_id": order.ClubID}
		actorID, createdAt = order.BuyerID, order.CreatedAt
		if delivery.Scene == "buyer_accepted" || delivery.Scene == "club_dispute" {
			actorID = order.ProviderID
		}
		service, serviceErr := get[Service](ctx, q, "services", order.ServiceID, false)
		if serviceErr != nil && !missing(serviceErr) {
			return serviceErr
		}
		if serviceErr == nil {
			objects["service"] = map[string]any{"id": service.ID, "title": service.Title, "price_cents": service.PriceCents}
		}
		delivery.Values["order_no"] = order.OrderNo
	}
	if actorID != "" {
		actor, actorErr := get[User](ctx, q, "users", actorID, false)
		if actorErr != nil && !missing(actorErr) {
			return actorErr
		}
		if actorErr == nil {
			objects["actor"] = map[string]any{"id": actor.ID, "nickname": actor.Nickname}
		}
	}
	projected, err := a.projectReviewedValue(ctx, objects)
	if err != nil {
		return err
	}
	visible, _ := projected.(map[string]any)
	delivery.Values["actor_name"] = subscriptionDisplayText(visible["actor"], "nickname", "平台玩家")
	delivery.Values["club_name"] = subscriptionDisplayText(visible["club"], "name", "电竞俱乐部")
	serviceName := subscriptionDisplayText(visible["order"], "title", "游戏服务")
	if _, exists := visible["service"]; exists {
		serviceName = subscriptionDisplayText(visible["service"], "title", "游戏服务")
	}
	delivery.Values["service_name"] = serviceName
	delivery.Values["created_time"] = subscriptionBusinessTime(createdAt)
	delivery.Values["time"] = subscriptionBusinessTime(delivery.CreatedAt)
	delivery.Values["status"] = map[string]string{"provider_order": "待接单", "provider_refund": "退款申请", "buyer_accepted": "已接单", "club_member": "待审批", "club_dispute": "待介入", "admin_club": "待认证"}[delivery.Scene]
	delivery.Values["target_name"] = serviceName
	if delivery.Scene == "club_member" || delivery.Scene == "admin_club" {
		delivery.Values["target_name"] = delivery.Values["club_name"]
	}
	return nil
}
