// Package app 隔离公开演示记录，确保示例订单及关联示例资源永不进入真实交易和退款通道。
package app

import "strings"

// requireLiveOrder 拒绝任何关联演示资源的订单变更，不受开发模式或管理员身份影响。
func requireLiveOrder(o Order) error {
	for _, id := range []string{o.ID, o.BuyerID, o.ProviderID, o.ServiceID, o.ClubID} {
		if strings.HasPrefix(id, "demo_") {
			return fail(409, "演示订单仅供浏览，不支持接单、支付、退款或其他交易操作")
		}
	}
	return nil
}
