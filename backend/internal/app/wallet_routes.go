// Package app 注册钱包、管理员转账审核和官方通知路由，所有资金接口均经过本人或平台管理员身份校验。
package app

import "net/http"

// registerWalletRoutes 集中挂载钱包API，避免页面或通用管理资源绕过资金业务状态机。
func (a *App) registerWalletRoutes(mux *http.ServeMux) {
	routes := map[string]func(http.ResponseWriter, *http.Request) (any, error){
		"GET /api/wallet": a.walletSummary, "GET /api/wallet/entries": a.walletEntries,
		"GET /api/wallet/topups": a.walletTopups, "POST /api/wallet/topups": a.createWalletTopup, "POST /api/wallet/topups/{id}/sync": a.syncWalletTopup, "POST /api/wallet/topups/{id}/pay": a.resumeWalletTopup,
		"GET /api/wallet/withdrawals": a.walletWithdrawals, "POST /api/wallet/withdrawals": a.createWalletWithdrawal, "POST /api/wallet/withdrawals/{id}/sync": a.syncWalletWithdrawal,
		"GET /api/wallet/deposit-refunds": a.walletDepositRefunds, "POST /api/wallet/deposit-refunds": a.createWalletDepositRefund, "POST /api/wallet/deposit-refunds/{id}/sync": a.syncWalletDepositRefund,
		"GET /api/admin/wallet/withdrawals": a.adminWalletWithdrawals, "POST /api/admin/wallet/withdrawals/{id}/execute": a.executeWalletWithdrawal, "POST /api/admin/wallet/withdrawals/{id}/reject": a.rejectWalletWithdrawal,
		"GET /api/admin/wallet-settings": a.adminWalletSettings, "PATCH /api/admin/wallet-settings": a.saveWalletSettings,
	}
	for path, fn := range routes {
		mux.HandleFunc(path, a.wrap(fn))
	}
	mux.HandleFunc("POST /api/payments/wechat/transfer-notify", a.transferCallback)
}
