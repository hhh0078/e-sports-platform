// Package app 验证旧付款意图的公开渠道投影及私密支付身份删除，不改变原始交易记录。
package app

import "testing"

// TestCommercePendingChannelProjection 保留普通微信原意图和虚拟渠道，禁止泄露或回写私密身份。
func TestCommercePendingChannelProjection(t *testing.T) {
	original := Order{PaymentStatus: "unpaid", PayOpenID: "private-old-payer", PaymentAppID: "private-app", PaymentMerchantID: "private-merchant"}
	projected := publicValue(original).(Order)
	if projected.PaymentChannel != "wechat" || projected.PayOpenID != "" || projected.PaymentAppID != "" || projected.PaymentMerchantID != "" {
		t.Fatal("旧单渠道投影不准确或暴露了私密身份")
	}
	if original.PaymentChannel != "" || original.PayOpenID == "" {
		t.Fatal("公开投影改变了原交易记录")
	}
	original.PaymentChannel = "virtual"
	if publicValue(original).(Order).PaymentChannel != "virtual" {
		t.Fatal("虚拟支付原渠道被覆盖")
	}
	if publicValue(Order{}).(Order).PaymentChannel != "" {
		t.Fatal("未发起付款的订单被误标为普通支付")
	}
}
