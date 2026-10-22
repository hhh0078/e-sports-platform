// Package app 定义人民币整数分钱包、不可变账单、充值、提现及押金原路退还记录。
package app

// Wallet 按用途隔离充值款、可提现收入、待结算收入、提现冻结与押金，不以展示总额作为可用资金。
type Wallet struct {
	UserID                string `json:"user_id"`
	RechargeCents         int64  `json:"recharge_cents"`
	AvailableCents        int64  `json:"available_cents"`
	PendingCents          int64  `json:"pending_cents"`
	WithdrawalFrozenCents int64  `json:"withdrawal_frozen_cents"`
	DepositCents          int64  `json:"deposit_cents"`
	DepositRefundingCents int64  `json:"deposit_refunding_cents"`
	UpdatedAt             string `json:"updated_at"`
}

// WalletEntry 保存各资金分区的实际变动；业务唯一编号保证重放不会重复记账。
type WalletEntry struct {
	ID                    string `json:"id"`
	UserID                string `json:"user_id"`
	Kind                  string `json:"kind"`
	ReferenceID           string `json:"reference_id"`
	AmountCents           int64  `json:"amount_cents"`
	RechargeDelta         int64  `json:"recharge_delta"`
	AvailableDelta        int64  `json:"available_delta"`
	PendingDelta          int64  `json:"pending_delta"`
	WithdrawalFrozenDelta int64  `json:"withdrawal_frozen_delta"`
	DepositDelta          int64  `json:"deposit_delta"`
	DepositRefundingDelta int64  `json:"deposit_refunding_delta"`
	Note                  string `json:"note"`
	CreatedAt             string `json:"created_at"`
}

// WalletEscrow 逐单记录实付来源与托管状态，退款严格还原原支出，历史无账本订单不会凭空产生收入。
type WalletEscrow struct {
	OrderID      string `json:"order_id"`
	BuyerID      string `json:"buyer_id"`
	ProviderID   string `json:"provider_id"`
	AmountCents  int64  `json:"amount_cents"`
	Channel      string `json:"channel"`
	RechargeUsed int64  `json:"recharge_used_cents"`
	IncomeUsed   int64  `json:"income_used_cents"`
	Status       string `json:"status"`
	CreatedAt    string `json:"created_at"`
	ReleasedAt   string `json:"released_at,omitempty"`
}

// WalletTopup 保存充值付款意图和微信核验结果；私密支付标识只在服务端使用。
type WalletTopup struct {
	MerchantID    string `json:"merchant_id,omitempty"`
	AppID         string `json:"app_id,omitempty"`
	ID            string `json:"id"`
	UserID        string `json:"user_id"`
	RequestID     string `json:"request_id"`
	Purpose       string `json:"purpose"`
	AmountCents   int64  `json:"amount_cents"`
	Status        string `json:"status"`
	OrderNo       string `json:"order_no"`
	PayOpenID     string `json:"pay_open_id,omitempty"`
	TransactionID string `json:"transaction_id,omitempty"`
	RefundedCents int64  `json:"refunded_cents"`
	ReservedCents int64  `json:"reserved_cents"`
	CreatedAt     string `json:"created_at"`
	UpdatedAt     string `json:"updated_at"`
}

// WalletWithdrawal 先冻结服务收入，再由已核验的官方转账终态扣除或解冻；充值余额不能提现。
type WalletWithdrawal struct {
	MerchantID     string         `json:"merchant_id,omitempty"`
	AppID          string         `json:"app_id,omitempty"`
	ID             string         `json:"id"`
	UserID         string         `json:"user_id"`
	RequestID      string         `json:"request_id"`
	AmountCents    int64          `json:"amount_cents"`
	Status         string         `json:"status"`
	OutBillNo      string         `json:"out_bill_no"`
	TransferBillNo string         `json:"transfer_bill_no,omitempty"`
	PayOpenID      string         `json:"pay_open_id,omitempty"`
	EncryptedName  string         `json:"encrypted_name,omitempty"`
	NameHash       string         `json:"name_hash,omitempty"`
	TransferConfig WalletSettings `json:"transfer_config"`
	PackageInfo    string         `json:"package_info,omitempty"`
	FailureReason  string         `json:"failure_reason,omitempty"`
	CreatedAt      string         `json:"created_at"`
	UpdatedAt      string         `json:"updated_at"`
}

// DepositRefundPart 对每一笔押金充值分别建立稳定退款号，支持部分金额与多笔原路退款。
type DepositRefundPart struct {
	TopupID     string `json:"topup_id"`
	RefundNo    string `json:"refund_no"`
	AmountCents int64  `json:"amount_cents"`
	Status      string `json:"status"`
}

// WalletDepositRefund 在事务内移除有效押金，再等待各笔微信退款；未知外部结果保持冻结。
type WalletDepositRefund struct {
	ID          string              `json:"id"`
	UserID      string              `json:"user_id"`
	RequestID   string              `json:"request_id"`
	AmountCents int64               `json:"amount_cents"`
	Status      string              `json:"status"`
	Parts       []DepositRefundPart `json:"parts"`
	CreatedAt   string              `json:"created_at"`
	UpdatedAt   string              `json:"updated_at"`
}
