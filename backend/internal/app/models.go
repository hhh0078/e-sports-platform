// Package app 提供集结平台的身份、游戏、服务、俱乐部和交易领域模型。
package app

type User struct {
	ProfileCover    string `json:"profile_cover"`
	PhoneVerified   bool   `json:"phone_verified"`
	PhoneMasked     string `json:"phone_masked"`
	ID              string `json:"id"`
	Nickname        string `json:"nickname"`
	Avatar          string `json:"avatar"`
	Bio             string `json:"bio"`
	Role            string `json:"role"`
	Status          string `json:"status"`
	ProviderEnabled bool   `json:"provider_enabled"`
	CreatedAt       string `json:"created_at"`
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
	ID             string       `json:"id"`
	GameID         string       `json:"game_id"`
	ProviderID     string       `json:"provider_id"`
	ProviderName   string       `json:"provider_name"`
	ProviderAvatar string       `json:"provider_avatar"`
	ClubID         string       `json:"club_id"`
	ClubName       string       `json:"club_name"`
	ClubVerified   bool         `json:"club_verified"`
	Title          string       `json:"title"`
	Description    string       `json:"description"`
	Category       string       `json:"category"`
	PriceCents     int64        `json:"price_cents"`
	Unit           string       `json:"unit"`
	CoverURL       string       `json:"cover_url"`
	Labels         []CoverLabel `json:"labels"`
	Tags           []string     `json:"tags"`
	Status         string       `json:"status"`
	Rating         float64      `json:"rating"`
	CompletedCount int          `json:"completed_count"`
	CreatedAt      string       `json:"created_at"`
	Reviews        []Review     `json:"reviews,omitempty"`
}
type Club struct {
	CoverURL         string           `json:"cover_url"`
	DetailBlocks     []CommunityBlock `json:"detail_blocks"`
	DepositCents     int64            `json:"deposit_cents"`
	Assessment       string           `json:"assessment"`
	ID               string           `json:"id"`
	Name             string           `json:"name"`
	Description      string           `json:"description"`
	OwnerID          string           `json:"owner_id"`
	OwnerName        string           `json:"owner_name"`
	GameIDs          []string         `json:"game_ids"`
	Status           string           `json:"status"`
	Verified         bool             `json:"verified"`
	MemberCount      int              `json:"member_count"`
	CreatedAt        string           `json:"created_at"`
	MembershipStatus string           `json:"membership_status,omitempty"`
}
type ClubMember struct {
	ApplicationNote      string `json:"application_note,omitempty"`
	RequirementsAccepted bool   `json:"requirements_accepted,omitempty"`
	AppliedDepositCents  int64  `json:"applied_deposit_cents,omitempty"`
	AppliedAssessment    string `json:"applied_assessment,omitempty"`
	AssessmentPassed     bool   `json:"assessment_passed,omitempty"`
	ReviewNote           string `json:"review_note,omitempty"`
	UserID               string `json:"user_id"`
	Nickname             string `json:"nickname"`
	ClubID               string `json:"club_id"`
	Status               string `json:"status"`
	CreatedAt            string `json:"created_at"`
}
type Order struct {
	ID                   string `json:"id"`
	OrderNo              string `json:"order_no"`
	GameID               string `json:"game_id"`
	GameName             string `json:"game_name"`
	ServiceID            string `json:"service_id"`
	BuyerID              string `json:"buyer_id"`
	BuyerName            string `json:"buyer_name"`
	ProviderID           string `json:"provider_id"`
	ProviderName         string `json:"provider_name"`
	ClubID               string `json:"club_id"`
	Title                string `json:"title"`
	Requirements         string `json:"requirements"`
	Quantity             int    `json:"quantity"`
	AmountCents          int64  `json:"amount_cents"`
	Status               string `json:"status"`
	PaymentStatus        string `json:"payment_status"`
	DeliveryNote         string `json:"delivery_note"`
	DisputeReason        string `json:"dispute_reason"`
	CreatedAt            string `json:"created_at"`
	UpdatedAt            string `json:"updated_at"`
	RefundStatus         string `json:"refund_status"`
	RefundReason         string `json:"refund_reason"`
	RefundReply          string `json:"refund_reply"`
	CompletedAt          string `json:"completed_at"`
	PaymentTransactionID string `json:"payment_transaction_id,omitempty"`
	RefundID             string `json:"refund_id,omitempty"`
	PayOpenID            string `json:"pay_open_id,omitempty"`
	ServiceDescription   string `json:"service_description"`
	Unit                 string `json:"unit"`
	UnitPriceCents       int64  `json:"unit_price_cents"`
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
	SiteName     string `json:"site_name"`
	Announcement string `json:"announcement"`
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
