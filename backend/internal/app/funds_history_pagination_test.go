// Package app 验证资金历史只读分页、组织筛选和本人归属，不调用任何资金写入接口。
package app

import (
	"fmt"
	"testing"
)

// TestIntegrationClubWithdrawalHistoryPagination 验证100条以后的组织历史、同时间稳定排序及负责人权限。
func TestIntegrationClubWithdrawalHistoryPagination(t *testing.T) {
	api := newTestAPI(t)
	stamp := "2026-10-24T01:00:00Z"
	for i := 0; i < 115; i++ {
		id := fmt.Sprintf("history-withdraw-%03d", i)
		v := WalletWithdrawal{ID: id, UserID: "clubowner", ClubID: "shanhai", AccountID: clubWalletID("shanhai"), AccountType: "club", RequestID: id, OutBillNo: id, AmountCents: 1, Status: "success", CreatedAt: stamp, UpdatedAt: stamp}
		if err := putRaw(t.Context(), api.App.DB, "wallet_withdrawals", id, v); err != nil {
			t.Fatal(err)
		}
	}
	seen := map[string]bool{}
	for page, expected := range []int{30, 30, 30, 25, 0} {
		var items []WalletWithdrawal
		api.call("clubowner", "GET", fmt.Sprintf("/clubs/shanhai/withdrawals?limit=30&offset=%d", page*30), nil, 200, &items)
		if len(items) != expected {
			t.Fatalf("第%d页数量错误: %d != %d", page, len(items), expected)
		}
		for index, item := range items {
			want := fmt.Sprintf("history-withdraw-%03d", 114-page*30-index)
			if item.ID != want || seen[item.ID] || item.ClubID != "shanhai" {
				t.Fatalf("分页重复、乱序或越权: %+v，预期%s", item, want)
			}
			seen[item.ID] = true
		}
	}
	if len(seen) != 115 {
		t.Fatalf("历史被100条上限截断: %d", len(seen))
	}
	for _, user := range []string{"worker", "outsider"} {
		api.call(user, "GET", "/clubs/shanhai/withdrawals?limit=30&offset=100", nil, 403, nil)
	}
	var managed []map[string]any
	api.call("admin", "GET", "/clubs/shanhai/withdrawals?limit=30&offset=100", nil, 200, &managed)
	if len(managed) != 15 {
		t.Fatal("平台管理员未能只读查看组织资金历史")
	}
	for _, row := range managed {
		if row["confirmation"] != nil {
			t.Fatal("平台读取不得得到负责人收款确认参数")
		}
	}
	api.call("", "GET", "/clubs/shanhai/withdrawals?limit=30", nil, 401, nil)
}

// TestIntegrationOwnClubFundingHistoryFilters 验证本人组织用途先筛选再分页，普通成员不能读取他人的资金历史。
func TestIntegrationOwnClubFundingHistoryFilters(t *testing.T) {
	api := newTestAPI(t)
	stamp := "2026-10-24T01:00:00Z"
	for i := 0; i < 75; i++ {
		id := fmt.Sprintf("history-topup-%03d", i)
		club, purpose, scope := "shanhai", "club_platform", "club_platform"
		if i >= 35 {
			club = "another-club"
		}
		if i >= 65 {
			club, purpose, scope = "shanhai", "member_club", "member_club"
		}
		topup := WalletTopup{ID: id, UserID: "clubowner", RequestID: id, OrderNo: id, ClubID: club, Purpose: purpose, Status: "paid", AmountCents: 1, CreatedAt: stamp, UpdatedAt: stamp}
		refundID := fmt.Sprintf("history-refund-%03d", i)
		refund := WalletDepositRefund{ID: refundID, UserID: "clubowner", RequestID: refundID, ClubID: club, Scope: scope, Status: "success", AmountCents: 1, CreatedAt: stamp, UpdatedAt: stamp}
		if err := putRaw(t.Context(), api.App.DB, "wallet_topups", id, topup); err != nil {
			t.Fatal(err)
		}
		if err := putRaw(t.Context(), api.App.DB, "wallet_deposit_refunds", refundID, refund); err != nil {
			t.Fatal(err)
		}
	}
	foreign := WalletTopup{ID: "history-foreign", UserID: "worker", RequestID: "history-foreign", OrderNo: "history-foreign", ClubID: "shanhai", Purpose: "club_platform", CreatedAt: stamp, UpdatedAt: stamp}
	if err := putRaw(t.Context(), api.App.DB, "wallet_topups", foreign.ID, foreign); err != nil {
		t.Fatal(err)
	}
	for page, expected := range []int{30, 5, 0} {
		var topups []WalletTopup
		var refunds []WalletDepositRefund
		api.call("clubowner", "GET", fmt.Sprintf("/wallet/topups?club_id=shanhai&purpose=club_platform&limit=30&offset=%d", page*30), nil, 200, &topups)
		api.call("clubowner", "GET", fmt.Sprintf("/wallet/deposit-refunds?club_id=shanhai&scope=club_platform&limit=30&offset=%d", page*30), nil, 200, &refunds)
		if len(topups) != expected || len(refunds) != expected {
			t.Fatalf("先筛选再分页失败，第%d页付款%d，退款%d，预期%d", page, len(topups), len(refunds), expected)
		}
		for index, item := range topups {
			if item.ID != fmt.Sprintf("history-topup-%03d", 34-page*30-index) || item.UserID != "clubowner" || item.ClubID != "shanhai" || item.Purpose != "club_platform" {
				t.Fatalf("付款筛选越界: %+v", item)
			}
		}
		for index, item := range refunds {
			if item.ID != fmt.Sprintf("history-refund-%03d", 34-page*30-index) || item.UserID != "clubowner" || item.ClubID != "shanhai" || item.Scope != "club_platform" {
				t.Fatalf("退款筛选越界: %+v", item)
			}
		}
	}
	api.call("outsider", "GET", "/wallet/topups?club_id=shanhai&purpose=club_platform", nil, 403, nil)
	legacy := WalletDepositRefund{ID: "history-legacy-personal", UserID: "clubowner", RequestID: "history-legacy-personal", CreatedAt: stamp, UpdatedAt: stamp}
	if err := putRaw(t.Context(), api.App.DB, "wallet_deposit_refunds", legacy.ID, legacy); err != nil {
		t.Fatal(err)
	}
	var personal []WalletDepositRefund
	api.call("clubowner", "GET", "/wallet/deposit-refunds?scope=personal", nil, 200, &personal)
	if len(personal) != 1 || personal[0].ID != legacy.ID {
		t.Fatal("旧个人退款scope兼容失效")
	}
	api.call("clubowner", "GET", "/wallet/topups?purpose=forged", nil, 400, nil)
	api.call("clubowner", "GET", "/wallet/deposit-refunds?scope=forged", nil, 400, nil)
}
