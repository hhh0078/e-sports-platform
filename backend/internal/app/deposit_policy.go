// Package app 按当前个人及俱乐部平台最低押金判断认证与补缴差额，不移动已有资金或改变原支付来源。
package app

import "context"

// depositShortfall 返回达到当前最低押金还需补缴的整数分，足额或未配置门槛时为零。
func depositShortfall(amount, required int64) int64 {
	if amount >= required {
		return 0
	}
	return required - amount
}

// qualifiedPlatformDeposit 只返回已实缴且达到当前门槛的金额；不足门槛的原押金继续保留在本人专户。
func qualifiedPlatformDeposit(wallet Wallet, required int64) int64 {
	if wallet.DepositCents > 0 && wallet.DepositCents >= required {
		return wallet.DepositCents
	}
	return 0
}

// validateNewDepositTopup 仅校验新付款意图累计后是否足额，不拒绝旧意图恢复或已发生的真实付款入账。
func validateNewDepositTopup(ctx context.Context, q querier, purpose, clubID string, wallet Wallet, amount int64) error {
	if purpose != "deposit" && purpose != "club_platform" {
		return nil
	}
	settings, err := financialSettings(ctx, q)
	if err != nil {
		return err
	}
	required := settings.PersonalPlatformDepositCents
	if purpose == "club_platform" {
		club, e := get[Club](ctx, q, "clubs", clubID, false)
		if e != nil {
			return e
		}
		required = clubRequiredDeposit(club, settings)
	}
	total, err := checkedWalletAmount(wallet.DepositCents, amount)
	if err != nil {
		return err
	}
	if total < required {
		return fail(400, "本次缴纳后仍未达到平台最低押金，请按页面显示的差额补缴")
	}
	return nil
}
