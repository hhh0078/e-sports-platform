// Package app 根据六类已确认领域变化生成持久通知，业务事务内不访问微信或其他外部服务。
package app

import (
	"context"
	"encoding/json"
	"net/url"
	"strings"
)

// domainNoticeTable 限定可产生六类业务通知的领域表，普通聊天和资金记录不触发新提醒。
func domainNoticeTable(table string) bool {
	return table == "orders" || table == "members" || table == "clubs"
}

// noticeVersions 解码受信任的前后领域记录；不存在的旧记录使用零值。
func noticeVersions[T any](previous, current []byte) (T, T, error) {
	var before, after T
	if len(previous) > 0 {
		if err := json.Unmarshal(previous, &before); err != nil {
			return before, after, err
		}
	}
	err := json.Unmarshal(current, &after)
	return before, after, err
}

// captureDomainNotices 只比较白名单结构化状态，种子使用独立插入函数且历史数据不回扫。
func captureDomainNotices(ctx context.Context, q querier, table, id string, previous, current []byte) error {
	switch table {
	case "orders":
		before, after, err := noticeVersions[Order](previous, current)
		if err != nil {
			return err
		}
		return noticeOrderChange(ctx, q, before, after, len(previous) > 0)
	case "members":
		before, after, err := noticeVersions[ClubMember](previous, current)
		if err != nil {
			return err
		}
		return noticeMemberChange(ctx, q, before, after, len(previous) > 0)
	case "clubs":
		before, after, err := noticeVersions[Club](previous, current)
		if err != nil {
			return err
		}
		return noticeClubChange(ctx, q, before, after, len(previous) > 0)
	}
	return nil
}

// domainNotice 为六类可信事件设置固定摘要，不发送待审图文、聊天证据、手机号或支付秘密。
func domainNotice(id, scene, event, title, body, path string, users ...string) DomainNotice {
	return DomainNotice{ID: id, Scene: scene, Event: event, UserIDs: users, Title: title, Body: body, ActionPath: path, ActionLabel: "查看详情", Values: map[string]string{"title": title, "summary": title, "status": title}}
}

// noticeOrderPath 返回经过转义的订单详情地址，不接受客户端路由。
func noticeOrderPath(id string) string { return "/pages/orders/detail?id=" + url.QueryEscape(id) }

// noticeClubOwner 取得组织当前负责人，已删除组织或演示组织不产生伪造接收人。
func noticeClubOwner(ctx context.Context, q querier, clubID string) (string, error) {
	if clubID == "" || strings.HasPrefix(clubID, "demo_") {
		return "", nil
	}
	club, err := get[Club](ctx, q, "clubs", clubID, false)
	if missing(err) {
		return "", nil
	}
	return club.OwnerID, err
}
