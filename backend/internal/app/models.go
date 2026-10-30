// Package app 提供开机电竞平台的身份、游戏、服务、俱乐部和交易领域模型。
package app

import "encoding/json"

type User struct {
	// IsMinor 默认为未成年或尚未核验，仅成年人实名核验成功后才可置为假。
	IsMinor       bool   `json:"is_minor"`
	ProfileCover  string `json:"profile_cover"`
	PhoneVerified bool   `json:"phone_verified"`
	PhoneMasked   string `json:"phone_masked"`
	ID            string `json:"id"`
	Nickname      string `json:"nickname"`
	Avatar        string `json:"avatar"`
	Bio           string `json:"bio"`
	Role          string `json:"role"`
	// PlatformAdminRole 区分平台根管理员和受托副管理员，普通用户为空。
	PlatformAdminRole string `json:"platform_admin_role"`
	Status            string `json:"status"`
	ProviderEnabled   bool   `json:"provider_enabled"`
	CreatedAt         string `json:"created_at"`
}

// UnmarshalJSON 兼容历史关闭或缺失的接单字段，读取时始终默认开启；冻结与手机号状态保持原值。
func (u *User) UnmarshalJSON(data []byte) error {
	type storedUser User
	value := storedUser{IsMinor: true}
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	*u = User(value)
	u.ProviderEnabled = true
	if u.Role == "admin" {
		if u.PlatformAdminRole == "" || u.PlatformAdminRole == "root" {
			u.PlatformAdminRole = "root"
		} else {
			u.PlatformAdminRole = "deputy"
		}
	} else if u.Role != "admin" {
		u.PlatformAdminRole = ""
	}
	return nil
}

type Game struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	ShortName    string   `json:"short_name"`
	Subtitle     string   `json:"subtitle"`
	PrimaryColor string   `json:"primary_color"`
	BgColor      string   `json:"bg_color"`
	SurfaceColor string   `json:"surface_color"`
	TextColor    string   `json:"text_color"`
	IconURL      string   `json:"icon_url"`
	CoverURL     string   `json:"cover_url"`
	Covers       []string `json:"covers"`
	Categories   []string `json:"categories"`
	Enabled      bool     `json:"enabled"`
	SortOrder    int      `json:"sort_order"`
}
type CoverLabel struct {
	ID         string `json:"id"`
	Text       string `json:"text"`
	Color      string `json:"color"`
	Background string `json:"background"`
	Position   string `json:"position"`
	FontSize   int    `json:"font_size"`
}
type Service struct {
	ProviderClubDepositCents int64        `json:"provider_club_deposit_cents"`
	ProviderDepositCents     int64        `json:"provider_deposit_cents"`
	ID                       string       `json:"id"`
	GameID                   string       `json:"game_id"`
	ProviderID               string       `json:"provider_id"`
	ProviderName             string       `json:"provider_name"`
	ProviderAvatar           string       `json:"provider_avatar"`
	ClubID                   string       `json:"club_id"`
	ClubName                 string       `json:"club_name"`
	ClubVerified             bool         `json:"club_verified"`
	Title                    string       `json:"title"`
	Description              string       `json:"description"`
	Category                 string       `json:"category"`
	PriceCents               int64        `json:"price_cents"`
	Unit                     string       `json:"unit"`
	CoverURL                 string       `json:"cover_url"`
	Labels                   []CoverLabel `json:"labels"`
	Tags                     []string     `json:"tags"`
	Status                   string       `json:"status"`
	Rating                   float64      `json:"rating"`
	CompletedCount           int          `json:"completed_count"`
	CreatedAt                string       `json:"created_at"`
	Reviews                  []Review     `json:"reviews,omitempty"`
}
type Club struct {
	// ContactQRURL 为公开客服二维码，仅接受当前编辑者上传的组织客服用途图片。
	ContactQRURL string `json:"contact_qr_url"`
	// ContactPhone 为俱乐部公开客服电话，不参与登录手机号验证。
	ContactPhone string `json:"contact_phone"`
	// ContactWechat 为俱乐部公开客服微信号。
	ContactWechat                 string           `json:"contact_wechat"`
	CreationDepositVersion        int              `json:"creation_deposit_version"`
	CreationDepositRequiredCents  int64            `json:"creation_deposit_required_cents"`
	AvatarURL                     string           `json:"avatar_url"`
	CommissionRateBPS             int64            `json:"commission_rate_bps"`
	JoinNote                      string           `json:"join_note"`
	PlatformDepositCents          int64            `json:"platform_deposit_cents"`
	PlatformDepositRefundingCents int64            `json:"platform_deposit_refunding_cents"`
	PlatformDepositRequiredCents  int64            `json:"platform_deposit_required_cents"`
	PlatformDepositShortfallCents int64            `json:"platform_deposit_shortfall_cents"`
	DepositReady                  bool             `json:"deposit_ready"`
	CoverURL                      string           `json:"cover_url"`
	DetailBlocks                  []CommunityBlock `json:"detail_blocks"`
	DepositCents                  int64            `json:"deposit_cents"`
	Assessment                    string           `json:"assessment"`
	ID                            string           `json:"id"`
	Name                          string           `json:"name"`
	Description                   string           `json:"description"`
	OwnerID                       string           `json:"owner_id"`
	OwnerName                     string           `json:"owner_name"`
	GameIDs                       []string         `json:"game_ids"`
	Status                        string           `json:"status"`
	Verified                      bool             `json:"verified"`
	MemberCount                   int              `json:"member_count"`
	CreatedAt                     string           `json:"created_at"`
	MembershipStatus              string           `json:"membership_status,omitempty"`
	ManagementRole                string           `json:"management_role,omitempty"`
	CanManageAdmins               bool             `json:"can_manage_admins,omitempty"`
}
type ClubMember struct {
	AppliedCommissionRateBPS int64  `json:"applied_commission_rate_bps"`
	AppliedJoinNote          string `json:"applied_join_note"`
	ApplicationNote          string `json:"application_note,omitempty"`
	RequirementsAccepted     bool   `json:"requirements_accepted,omitempty"`
	AppliedDepositCents      int64  `json:"applied_deposit_cents,omitempty"`
	AppliedAssessment        string `json:"applied_assessment,omitempty"`
	AssessmentPassed         bool   `json:"assessment_passed,omitempty"`
	ReviewNote               string `json:"review_note,omitempty"`
	UserID                   string `json:"user_id"`
	Nickname                 string `json:"nickname"`
	ClubID                   string `json:"club_id"`
	Status                   string `json:"status"`
	CreatedAt                string `json:"created_at"`
}
type Order struct {
	HasVirtualPayment               bool            `json:"has_virtual_payment"`
	AdjustmentVersion               int             `json:"adjustment_version,omitempty"`
	OriginalAmountCents             int64           `json:"original_amount_cents,omitempty"`
	PaidTotalCents                  int64           `json:"paid_total_cents,omitempty"`
	PriceRefundedCents              int64           `json:"price_refunded_cents,omitempty"`
	ActiveAdjustmentID              string          `json:"active_adjustment_id,omitempty"`
	FinancialReconciliationRequired bool            `json:"financial_reconciliation_required,omitempty"`
	PaymentMerchantID               string          `json:"payment_merchant_id,omitempty"`
	PaymentAppID                    string          `json:"payment_app_id,omitempty"`
	Settlement                      OrderSettlement `json:"settlement"`
	PaidAt                          string          `json:"paid_at,omitempty"`
	RefundedAt                      string          `json:"refunded_at,omitempty"`
	PaymentChannel                  string          `json:"payment_channel"`
	BalanceRechargeCents            int64           `json:"balance_recharge_cents"`
	BalanceIncomeCents              int64           `json:"balance_income_cents"`
	WalletAccounted                 bool            `json:"wallet_accounted"`
	WalletRequestID                 string          `json:"wallet_request_id,omitempty"`
	RefundFinal                     bool            `json:"refund_final"`
	ID                              string          `json:"id"`
	OrderNo                         string          `json:"order_no"`
	GameID                          string          `json:"game_id"`
	GameName                        string          `json:"game_name"`
	ServiceID                       string          `json:"service_id"`
	BuyerID                         string          `json:"buyer_id"`
	BuyerName                       string          `json:"buyer_name"`
	ProviderID                      string          `json:"provider_id"`
	ProviderName                    string          `json:"provider_name"`
	ClubID                          string          `json:"club_id"`
	Title                           string          `json:"title"`
	Requirements                    string          `json:"requirements"`
	Quantity                        int             `json:"quantity"`
	AmountCents                     int64           `json:"amount_cents"`
	Status                          string          `json:"status"`
	PaymentStatus                   string          `json:"payment_status"`
	DeliveryNote                    string          `json:"delivery_note"`
	DisputeReason                   string          `json:"dispute_reason"`
	CreatedAt                       string          `json:"created_at"`
	UpdatedAt                       string          `json:"updated_at"`
	RefundStatus                    string          `json:"refund_status"`
	RefundReason                    string          `json:"refund_reason"`
	RefundReply                     string          `json:"refund_reply"`
	CompletedAt                     string          `json:"completed_at"`
	PaymentTransactionID            string          `json:"payment_transaction_id,omitempty"`
	RefundID                        string          `json:"refund_id,omitempty"`
	PayOpenID                       string          `json:"pay_open_id,omitempty"`
	ServiceDescription              string          `json:"service_description"`
	Unit                            string          `json:"unit"`
	UnitPriceCents                  int64           `json:"unit_price_cents"`
}
type Message struct {
	ID         string `json:"id"`
	OrderID    string `json:"order_id"`
	SenderID   string `json:"sender_id"`
	SenderName string `json:"sender_name"`
	Content    string `json:"content"`
	CreatedAt  string `json:"created_at"`
	SenderRole string `json:"sender_role"`
	ImageURL   string `json:"image_url,omitempty"`
	Kind       string `json:"kind"`
}
type Review struct {
	ID        string `json:"id"`
	OrderID   string `json:"order_id"`
	Rating    int    `json:"rating"`
	Content   string `json:"content"`
	BuyerName string `json:"buyer_name"`
	CreatedAt string `json:"created_at"`
}
type Settings struct {
	// SupportQRURL 为平台管理员配置的公开客服二维码。
	SupportQRURL string `json:"support_qr_url"`
	// SupportPhone 为公开客服电话，不包含商户配置。
	SupportPhone string `json:"support_phone"`
	// SupportWechat 为公开客服微信号，在线咨询仍由微信官方客服承载。
	SupportWechat string `json:"support_wechat"`
	SiteName      string `json:"site_name"`
	Announcement  string `json:"announcement"`
}
type Audit struct {
	ID        string `json:"id"`
	ActorID   string `json:"actor_id"`
	Action    string `json:"action"`
	TargetID  string `json:"target_id"`
	Note      string `json:"note"`
	CreatedAt string `json:"created_at"`
}

// PublicJob 仅包含允许在匿名接单大厅展示的字段，不携带交易参与人及私密约定。
type PublicJob struct {
	ID             string `json:"id"`
	GameID         string `json:"game_id"`
	GameName       string `json:"game_name"`
	Title          string `json:"title"`
	Quantity       int    `json:"quantity"`
	AmountCents    int64  `json:"amount_cents"`
	Status         string `json:"status"`
	PaymentStatus  string `json:"payment_status"`
	Requirements   string `json:"requirements"`
	CreatedAt      string `json:"created_at"`
	Unit           string `json:"unit"`
	UnitPriceCents int64  `json:"unit_price_cents"`
}
