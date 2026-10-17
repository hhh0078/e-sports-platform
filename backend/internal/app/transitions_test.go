// Package app 测试交易状态机的权限、付款约束、售后期限和主题边界。
package app

import (
	"testing"
	"time"
)

// TestOrderTransitions 验证交付必须由打手执行且只能由买家确认。
func TestOrderTransitions(t *testing.T) {
	buyer := User{ID: "b"}
	provider := User{ID: "p", ProviderEnabled: true}
	o := Order{BuyerID: "b", ProviderID: "p", Status: "open", PaymentStatus: "paid", RefundStatus: "none"}
	if applyAction(&o, buyer, "accept", "") == nil {
		t.Fatal("买家不能冒充打手")
	}
	for _, action := range []string{"accept", "start", "deliver"} {
		if err := applyAction(&o, provider, action, "已经完成双方约定服务"); err != nil {
			t.Fatal(err)
		}
	}
	if applyAction(&o, provider, "confirm", "") == nil {
		t.Fatal("卖家不能自己确认")
	}
	if err := applyAction(&o, buyer, "request_refund", "进度不符合约定"); err != nil {
		t.Fatal(err)
	}
	if applyAction(&o, buyer, "confirm", "") == nil {
		t.Fatal("退款处理中必须暂停结算")
	}
	if err := applyAction(&o, provider, "reject_refund", "已经提交完整交付证据"); err != nil {
		t.Fatal(err)
	}
	if err := applyAction(&o, buyer, "escalate", ""); err != nil {
		t.Fatal(err)
	}
	if o.Status != "delivered" || o.RefundStatus != "escalated" {
		t.Fatal("退款不能丢失原履约状态")
	}
	if err := applyAction(&o, buyer, "withdraw_refund", ""); err != nil {
		t.Fatal(err)
	}
	if err := applyAction(&o, buyer, "confirm", ""); err != nil {
		t.Fatal(err)
	}
	if o.CompletedAt == "" {
		t.Fatal("完成时间不可缺失")
	}
}

// TestAfterSaleWindow 验证七天售后窗口和自接单保护。
func TestAfterSaleWindow(t *testing.T) {
	u := User{ID: "b", ProviderEnabled: true}
	o := Order{BuyerID: "b", Status: "completed", PaymentStatus: "paid", CompletedAt: time.Now().Add(-8 * 24 * time.Hour).Format(time.RFC3339Nano)}
	if applyAction(&o, u, "request_refund", "申请售后") == nil {
		t.Fatal("超期售后应拒绝")
	}
	o.Status = "open"
	o.ProviderID = ""
	if applyAction(&o, u, "claim", "") == nil {
		t.Fatal("不能抢自己需求")
	}
	o.BuyerID = "other"
	o.PaymentStatus = "unpaid"
	if applyAction(&o, u, "claim", "") == nil {
		t.Fatal("未付款需求不能抢单")
	}
}

// TestServiceValidation 验证封面不能来自其他游戏或注入任意 CSS。
func TestServiceValidation(t *testing.T) {
	g := Game{Covers: []string{"/static/covers/delta-1.jpg"}, Categories: []string{"陪练"}}
	s := Service{Title: "认真陪练", Description: "认真沟通并完成约定的陪练服务", PriceCents: 100, Unit: "小时", Category: "陪练", CoverURL: g.Covers[0], Labels: []CoverLabel{{Text: "耐心陪练", Color: "#FFFFFF", Background: "transparent"}}}
	if err := validateService(&s, g); err != nil {
		t.Fatal(err)
	}
	if s.Labels[0].Position != "bottom-left" {
		t.Fatal("默认位置应为左下角")
	}
	s.Labels[0].Color = "red;position:fixed"
	if validateService(&s, g) == nil {
		t.Fatal("非法样式必须拒绝")
	}
	s.Labels = nil
	s.CoverURL = "/static/covers/honor-1.jpg"
	if validateService(&s, g) == nil {
		t.Fatal("跨游戏封面必须拒绝")
	}
	s.CoverURL = g.Covers[0]
	s.PriceCents = -1
	if validateService(&s, g) == nil {
		t.Fatal("负价格必须拒绝")
	}
}

// TestProductionGuard 验证生产配置无法启动开发身份入口。
func TestProductionGuard(t *testing.T) {
	if _, err := New(t.Context(), Config{Env: "production", DevMode: true}); err == nil {
		t.Fatal("生产不可开启开发模式")
	}
}
