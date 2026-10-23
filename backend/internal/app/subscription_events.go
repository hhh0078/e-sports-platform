// Package app 登记用户确认的六类微信订阅事件，禁止将退款申请模板用于到账或其它业务。
package app

// SubscriptionEventOption 表示场景唯一允许的业务事件编码和中文含义。
type SubscriptionEventOption struct {
	Code  string `json:"code"`
	Label string `json:"label"`
}

// subscriptionEventCatalog 将六个订阅场景固定到真实事件及接收角色，不开放任意模板广播。
var subscriptionEventCatalog = map[string][]SubscriptionEventOption{
	"provider_order":  {{"order_paid", "打手收到已付款新订单"}},
	"provider_refund": {{"refund_requested", "打手收到买家退款申请"}},
	"buyer_accepted":  {{"order_accepted", "买家订单已被接单"}},
	"club_member":     {{"member_applied", "负责人收到新的入会申请"}},
	"club_dispute":    {{"club_dispute", "负责人旗下订单申请平台介入"}},
	"admin_club":      {{"admin_club", "平台收到已满足押金条件的俱乐部申请"}},
}

// subscriptionEventsForScene 返回选项副本，外部读取不会改写服务端的固定映射。
func subscriptionEventsForScene(scene string) []SubscriptionEventOption {
	return append([]SubscriptionEventOption{}, subscriptionEventCatalog[scene]...)
}

// validSubscriptionEvent 校验场景唯一精确事件，未知或跨场景事件不得发送微信模板。
func validSubscriptionEvent(scene, event string) bool {
	for _, option := range subscriptionEventCatalog[scene] {
		if option.Code == event {
			return true
		}
	}
	return false
}
