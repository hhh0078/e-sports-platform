// Package app 提供 HTTP 路由、错误处理、请求限流和配置校验。
package app

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"io"
	"jijie/internal/assets"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type Config struct {
	DatabaseURL, RedisAddr, RedisPassword, RedisPrefix, Addr, WechatAppID, WechatSecret, Env string
	WechatMessageToken, WechatMessageAESKey                                                  string
	DevMode                                                                                  bool
}
type App struct {
	publicCacheEnabled       atomic.Bool
	publicCacheReady         atomic.Bool
	publicCacheDrainMu       sync.Mutex
	publicCacheWorkerMu      sync.Mutex
	publicCacheCancel        context.CancelFunc
	publicCacheWG            sync.WaitGroup
	subscriptionWorkerMu     sync.Mutex
	subscriptionWorkerCancel context.CancelFunc
	subscriptionWorkerWG     sync.WaitGroup
	reviewWorkerMu           sync.Mutex
	reviewWorkerCancel       context.CancelFunc
	reviewWorkerWG           sync.WaitGroup
	paymentConfigMu          sync.RWMutex
	paymentConfig            *paymentConfigState
	DB                       *pgxpool.Pool
	Redis                    *redis.Client
	Config                   Config
	HTTPClient               *http.Client
	AIHTTPClient             *http.Client
	EvidenceKey              []byte
}
type apiError struct {
	Status  int
	Message string
	Code    string
}

// Error 返回可以安全展示的中文错误。
func (e *apiError) Error() string { return e.Message }

// fail 构造业务错误。
func fail(status int, message string) error { return &apiError{Status: status, Message: message} }

// env 读取环境变量并应用本地默认值。
func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// LoadConfig 加载配置，开发能力必须显式开启。
func LoadConfig() Config {
	return Config{DatabaseURL: env("DATABASE_URL", "postgres://postgres:postgres@127.0.0.1:5432/jijie_dev?sslmode=disable"), RedisAddr: env("REDIS_ADDR", "127.0.0.1:56379"), RedisPassword: os.Getenv("REDIS_PASSWORD"), RedisPrefix: env("REDIS_PREFIX", "jijie:"), Addr: env("ADDR", "127.0.0.1:8090"), WechatAppID: os.Getenv("WECHAT_APP_ID"), WechatSecret: os.Getenv("WECHAT_APP_SECRET"), WechatMessageToken: os.Getenv("WECHAT_MESSAGE_TOKEN"), WechatMessageAESKey: os.Getenv("WECHAT_MESSAGE_AES_KEY"), Env: env("APP_ENV", "development"), DevMode: os.Getenv("DEV_MODE") == "true"}
}

// New 校验生产边界、连接存储并进行幂等初始化。
func New(ctx context.Context, cfg Config) (*App, error) {
	if cfg.RedisPrefix == "" {
		cfg.RedisPrefix = "jijie:"
	}
	if cfg.Env == "production" && cfg.DevMode {
		return nil, errors.New("生产环境禁止 DEV_MODE")
	}
	db, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return nil, errors.New("数据库连接配置无效，请检查 DATABASE_URL 的格式与连接选项")
	}
	r := redis.NewClient(&redis.Options{Addr: cfg.RedisAddr, Password: cfg.RedisPassword, DialTimeout: 2 * time.Second, ReadTimeout: 2 * time.Second, WriteTimeout: 2 * time.Second})
	a := &App{DB: db, Redis: r, Config: cfg, HTTPClient: &http.Client{Timeout: 10 * time.Second}, EvidenceKey: []byte(newID() + newID())}
	if err = db.Ping(ctx); err != nil {
		a.Close()
		return nil, errors.New("数据库连接失败，请检查地址、认证与数据库状态")
	}
	if err = r.Ping(ctx).Err(); err != nil {
		a.Close()
		return nil, err
	}
	if err = migrate(ctx, db); err != nil {
		a.Close()
		return nil, err
	}
	if err = a.seed(ctx); err != nil {
		a.Close()
		return nil, err
	}
	if !cfg.DevMode {
		if err = guardProductionWalletData(ctx, db); err != nil {
			a.Close()
			return nil, err
		}
	}
	if err = a.loadPaymentSettings(ctx); err != nil {
		a.Close()
		return nil, err
	}
	return a, nil
}

// Close 释放数据库与会话连接。
func (a *App) Close() {
	a.stopPublicCacheWorker()
	a.stopSubscriptionWorker()
	a.stopModerationWorker()
	a.DB.Close()
	a.Redis.Close()
}

// decode 限制请求体大小并拒绝未知字段与多份 JSON。
func decode(r *http.Request, out any) error {
	return decodeWithLimit(r, out, 64*1024)
}

// decodeWithLimit 复用统一JSON安全规则，在有界传输预算内解码并严格拒绝未知字段及尾部JSON。
// limit 为服务端固定字节预算；正文实际长度仍由对应领域校验器独立核验。
func decodeWithLimit(r *http.Request, out any, limit int64) error {
	reader := &io.LimitedReader{R: r.Body, N: limit + 1}
	d := json.NewDecoder(reader)
	d.DisallowUnknownFields()
	if err := d.Decode(out); err != nil {
		return fail(400, "请求内容格式不正确")
	}
	if d.Decode(&struct{}{}) != io.EOF {
		return fail(400, "请求仅允许一个 JSON 对象")
	}
	if reader.N <= 0 {
		return fail(400, "请求内容超过接口允许大小")
	}
	return nil
}

// jsonTransportLimit 仅给三个精确文章写入路由预留JSON最坏转义预算，其它接口保持64KiB。
func jsonTransportLimit(r *http.Request) int64 {
	switch r.Pattern {
	case "POST /api/admin/articles", "PATCH /api/admin/articles/{id}", "POST /api/admin/articles/preview":
		return articleJSONTransportMaxBytes
	default:
		return 64 * 1024
	}
}

// respond 写入统一响应。
func respond(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(value)
}

// wrap 统一捕获异常并隐藏内部错误。
func (a *App) wrap(fn func(http.ResponseWriter, *http.Request) (any, error)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				log.Printf("请求异常: %v", rec)
				respond(w, 500, map[string]any{"error": "服务器暂时不可用"})
			}
		}()
		w.Header().Set("X-Content-Type-Options", "nosniff")
		if origin := r.Header.Get("Origin"); strings.HasPrefix(origin, "http://localhost:") || strings.HasPrefix(origin, "http://127.0.0.1:") {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type,Authorization")
			w.Header().Set("Access-Control-Allow-Methods", "GET,POST,PATCH,DELETE,OPTIONS")
		}
		if r.Method == "OPTIONS" {
			w.WriteHeader(204)
			return
		}
		if !strings.HasSuffix(r.URL.Path, "/evidence") && r.URL.Path != "/api/admin/covers" && r.URL.Path != "/api/media/images" {
			r.Body = http.MaxBytesReader(w, r.Body, jsonTransportLimit(r))
		}
		if r.Method != http.MethodGet && r.Method != http.MethodHead && r.Method != http.MethodOptions {
			for _, id := range []string{r.PathValue("id"), r.PathValue("user_id"), r.URL.Query().Get("club_id")} {
				if strings.HasPrefix(id, "demo_") {
					respond(w, 400, map[string]any{"error": "演示数据仅用于展示，不可修改或交易"})
					return
				}
			}
		}
		r = a.reviewContext(r)
		out, err := fn(w, r)
		if err == nil && publicCacheWriteRequest(r) {
			a.flushPublicCache(r.Context())
		}
		if err != nil {
			var ae *apiError
			if !errors.As(err, &ae) {
				if missing(err) {
					ae = &apiError{Status: 404, Message: "内容不存在"}
				} else {
					log.Printf("%s %s: %v", r.Method, r.URL.Path, err)
					ae = &apiError{Status: 500, Message: "服务器暂时不可用，请稍后重试"}
				}
			}
			payload := map[string]any{"error": ae.Message}
			if ae.Code != "" {
				payload["code"] = ae.Code
			}
			respond(w, ae.Status, payload)
			return
		}
		safe, projectionErr := a.projectReviewedValue(r.Context(), publicValue(out))
		if projectionErr != nil {
			respond(w, 503, map[string]any{"error": "内容显示暂不可用，请稍后重试"})
			return
		}
		payload := map[string]any{"data": safe}
		if meta := reviewResponseMeta(r.Context()); meta != nil {
			payload["moderation"] = meta
		}
		respond(w, 200, payload)
	}
}

// Handler 注册平台全部 API，并只允许明确列出的接口。
func (a *App) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.Handle("GET /static/covers/", assets.Handler())
	mux.Handle("GET /static/avatars/", assets.AvatarHandler())
	mux.Handle("GET /static/operations/", assets.OperationsHandler())
	a.registerIdentityRoutes(mux)
	a.registerAdminCentersRoutes(mux)
	a.registerPublicContentRoutes(mux)
	a.registerPaymentSettingsRoutes(mux)
	a.registerWalletRoutes(mux)
	a.registerAIRoutes(mux)
	a.registerModerationRoutes(mux)
	a.registerInboxRoutes(mux)
	a.registerSubscriptionRoutes(mux)
	a.registerAdjustmentRoutes(mux)
	a.registerShareRoutes(mux)
	routes := map[string]func(http.ResponseWriter, *http.Request) (any, error){
		"GET /api/players/{id}": a.playerProfile, "PATCH /api/me/profile": a.saveProfile, "POST /api/media/images": a.uploadImage,
		"GET /health": a.health, "GET /api/health": a.health, "GET /api/bootstrap": a.bootstrap, "GET /api/dev/accounts": a.accounts, "POST /api/auth/dev": a.devLogin, "POST /api/auth/wechat": a.wechatLogin, "POST /api/auth/logout": a.logout, "GET /api/me": a.me, "PATCH /api/me": a.updateMe, "GET /api/me/stats": a.stats,
		"GET /api/games": a.games, "GET /api/services": a.services, "GET /api/services/{id}": a.service, "POST /api/services": a.saveService, "PATCH /api/services/{id}": a.saveService, "POST /api/services/{id}/pause": a.pauseService,
		"POST /api/orders": a.createOrder, "POST /api/requests": a.createRequest, "GET /api/orders": a.orders, "GET /api/jobs": a.jobs, "GET /api/orders/{id}": a.order, "POST /api/orders/{id}/pay": a.pay, "POST /api/orders/{id}/sync-payment": a.syncPayment, "POST /api/orders/{id}/actions": a.action, "POST /api/orders/{id}/evidence": a.uploadEvidence, "GET /api/orders/{id}/evidence-link": a.evidenceLink, "GET /api/orders/{id}/messages": a.messages, "POST /api/orders/{id}/messages": a.message, "POST /api/orders/{id}/review": a.review, "GET /api/orders/{id}/reviews": a.reviews,
		"GET /api/clubs": a.clubs, "POST /api/clubs": a.saveClub, "GET /api/clubs/{id}": a.club, "PATCH /api/clubs/{id}": a.saveClub, "POST /api/clubs/{id}/join": a.joinClub, "POST /api/clubs/{id}/leave": a.leaveClub, "POST /api/clubs/{id}/members/{user_id}": a.moderateMember, "GET /api/clubs/{id}/orders": a.clubOrders,
		"POST /api/admin/covers": a.uploadCover, "GET /api/admin/overview": a.overview, "GET /api/admin/analytics": a.analytics, "GET /api/admin/{resource}": a.adminList, "PATCH /api/admin/users/{id}": a.moderateUser, "PATCH /api/admin/services/{id}": a.moderateService, "PATCH /api/admin/clubs/{id}": a.moderateClub, "POST /api/admin/games": a.saveGame, "PATCH /api/admin/games/{id}": a.saveGame, "POST /api/admin/orders/{id}/resolve": a.resolve, "POST /api/admin/orders/{id}/retry-refund": a.retryRefund, "PATCH /api/admin/settings": a.saveSettings,
	}
	for path, fn := range routes {
		mux.HandleFunc(path, a.wrap(fn))
	}
	mux.HandleFunc("POST /api/payments/wechat/notify", a.paymentCallback)
	mux.HandleFunc("POST /api/payments/wechat/refund-notify", a.refundCallback)
	mux.HandleFunc("GET /api/evidence/{id}", a.viewEvidence)
	mux.HandleFunc("GET /api/media/{id}", a.viewCover)
	mux.HandleFunc("OPTIONS /", a.wrap(func(w http.ResponseWriter, r *http.Request) (any, error) { return nil, nil }))
	return mux
}

// health 实时检查数据库与 Redis，不将故障报告为健康。
func (a *App) health(w http.ResponseWriter, r *http.Request) (any, error) {
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()
	dbErr := a.DB.Ping(ctx)
	rErr := a.Redis.Ping(ctx).Err()
	if dbErr != nil || rErr != nil {
		return nil, fail(503, "数据库或会话服务不可用")
	}
	return map[string]any{"status": "ok", "database": "ok", "redis": "ok", "dev_mode": a.Config.DevMode}, nil
}

// publicValue 移除业务响应中的支付私密标识，数据库仍保留回调校验资料。
func publicValue(value any) any {
	switch v := value.(type) {
	case WalletTopup:
		v.MerchantID = ""
		v.AppID = ""
		v.PayOpenID = ""
		v.TransactionID = ""
		return v
	case []WalletTopup:
		out := make([]WalletTopup, len(v))
		for i, item := range v {
			out[i] = publicValue(item).(WalletTopup)
		}
		return out
	case WalletWithdrawal:
		v.MerchantID = ""
		v.AppID = ""
		v.PayOpenID = ""
		v.EncryptedName = ""
		v.NameHash = ""
		v.PackageInfo = ""
		v.TransferConfig = WalletSettings{}
		return v
	case []WalletWithdrawal:
		out := make([]WalletWithdrawal, len(v))
		for i, item := range v {
			out[i] = publicValue(item).(WalletWithdrawal)
		}
		return out
	case Order:
		v.PaymentMerchantID = ""
		v.PaymentAppID = ""
		v.WalletRequestID = ""
		v.PayOpenID = ""
		v.PaymentTransactionID = ""
		v.RefundID = ""
		return v
	case []Order:
		out := make([]Order, len(v))
		for i, o := range v {
			out[i] = publicValue(o).(Order)
		}
		return out
	case map[string]any:
		out := map[string]any{}
		for k, item := range v {
			out[k] = publicValue(item)
		}
		return out
	default:
		return value
	}
}
