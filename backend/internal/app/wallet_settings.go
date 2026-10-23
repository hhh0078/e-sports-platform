// Package app 提供管理员钱包配置与提现资质校验，服务收入转账场景必须由真实商户自行开通。
package app

import (
	"context"
	"net/http"
	"net/url"
	"strings"
)

// WalletSceneReport 保存微信所批准转账场景要求的报备字段，不预设现金营销场景。
type WalletSceneReport struct {
	InfoType    string `json:"info_type"`
	InfoContent string `json:"info_content"`
}

// WalletSettings 保存不含支付密钥的钱包设置，支付签名与实名加密复用已加密的商户配置。
type WalletSettings struct {
	WithdrawalEnabled        bool                `json:"withdrawal_enabled"`
	TransferSceneID          string              `json:"transfer_scene_id"`
	TransferSceneReportInfos []WalletSceneReport `json:"transfer_scene_report_infos"`
	TransferNotifyURL        string              `json:"transfer_notify_url"`
	TransferRemark           string              `json:"transfer_remark"`
	UserRecvPerception       string              `json:"user_recv_perception"`
}

// walletSettings 读取已保存的钱包配置，默认关闭真实提现，开发通道仍可完整测试资金冻结。
func (a *App) walletSettings(ctx context.Context) (WalletSettings, error) {
	v, err := get[WalletSettings](ctx, a.DB, "settings", "wallet", false)
	if missing(err) {
		return WalletSettings{TransferSceneReportInfos: []WalletSceneReport{}, TransferRemark: "集结服务收入提现"}, nil
	}
	return v, err
}

// validateWalletSettings 校验官方商家转账配置格式，不把保存草稿当作商户场景已获审批。
func validateWalletSettings(v WalletSettings) error {
	if len(v.TransferSceneID) > 36 || strings.ContainsAny(v.TransferSceneID, "\r\n\t /?&") {
		return fail(400, "转账场景编号格式无效")
	}
	if !length(v.TransferRemark, 0, 32) || !length(v.UserRecvPerception, 0, 10) || len(v.TransferSceneReportInfos) > 10 {
		return fail(400, "转账说明最多32字，收款感知最多10字，报备字段最多10项")
	}
	if v.TransferNotifyURL != "" {
		address, e := url.Parse(v.TransferNotifyURL)
		if e != nil || !validNotifyURL(v.TransferNotifyURL) || address.RawQuery != "" {
			return fail(400, "商家转账通知地址必须是无查询参数的HTTPS地址")
		}
	}
	for _, info := range v.TransferSceneReportInfos {
		if !length(info.InfoType, 1, 15) || !length(info.InfoContent, 1, 32) {
			return fail(400, "转账场景报备字段名称须为1至15字，内容须为1至32字")
		}
	}
	return nil
}

// readyWalletTransfer 对真实提现要求完整商户配置和已显式启用的转账场景，不生成伪成功。
func (a *App) readyWalletTransfer(ctx context.Context) (WalletSettings, wxPayConfig, error) {
	settings, err := a.walletSettings(ctx)
	if err != nil {
		return settings, wxPayConfig{}, err
	}
	if !settings.WithdrawalEnabled || settings.TransferSceneID == "" || settings.TransferNotifyURL == "" || settings.TransferRemark == "" || len(settings.TransferSceneReportInfos) == 0 {
		return settings, wxPayConfig{}, fail(503, "平台尚未配置获批的服务收入转账场景，暂不能微信提现")
	}
	c, err := a.loadWXPay()
	return settings, c, err
}

// adminWalletSettings 仅向管理员展示转账配置和当前就绪状态。
func (a *App) adminWalletSettings(w http.ResponseWriter, r *http.Request) (any, error) {
	if _, err := a.admin(r); err != nil {
		return nil, err
	}
	v, err := a.walletSettings(r.Context())
	if err != nil {
		return nil, err
	}
	_, _, readyErr := a.readyWalletTransfer(r.Context())
	return map[string]any{"settings": v, "transfer_ready": readyErr == nil, "configuration_notice": "需商户平台开通服务收入适用的商家转账场景并填写报备字段；保存配置不代表微信审核通过"}, nil
}

// saveWalletSettings 审核配置草稿后保存并写入管理审计，不记录密钥或用户实名。
func (a *App) saveWalletSettings(w http.ResponseWriter, r *http.Request) (any, error) {
	u, err := a.admin(r)
	if err != nil {
		return nil, err
	}
	var in WalletSettings
	if err = decode(r, &in); err != nil {
		return nil, err
	}
	if err = validateWalletSettings(in); err != nil {
		return nil, err
	}
	tx, err := a.DB.Begin(r.Context())
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(r.Context())
	if err = put(r.Context(), tx, "settings", "wallet", in); err != nil {
		return nil, err
	}
	if err = audit(r.Context(), tx, u.ID, "wallet.settings.update", "wallet", "更新钱包转账场景配置"); err != nil {
		return nil, err
	}
	if err = tx.Commit(r.Context()); err != nil {
		return nil, err
	}
	return a.adminWalletSettings(w, r)
}
