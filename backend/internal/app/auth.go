// Package app 负责开发身份、微信身份兑换以及每次请求的冻结校验。
package app

import (
	"context"
	"github.com/redis/go-redis/v9"
	"net"
	"net/http"
	"strings"
	"time"
)

// actor 从 Redis 会话取用户，并实时校验账号状态。
func (a *App) actor(r *http.Request) (User, error) {
	token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	if token == "" || token == r.Header.Get("Authorization") {
		return User{}, fail(401, "请先登录")
	}
	id, err := a.Redis.Get(r.Context(), a.redisKey("session:"+token)).Result()
	if err == redis.Nil {
		return User{}, fail(401, "登录已过期，请重新登录")
	}
	if err != nil {
		return User{}, fail(503, "登录服务暂时不可用")
	}
	u, err := get[User](r.Context(), a.DB, "users", id, false)
	if err != nil {
		return User{}, fail(401, "账号不存在")
	}
	if u.Status != "active" {
		return User{}, fail(403, "账号已被停用")
	}
	return u, nil
}

// admin 强制校验平台管理员身份。
func (a *App) admin(r *http.Request) (User, error) {
	u, err := a.actor(r)
	if err != nil {
		return u, err
	}
	if u.Role != "admin" {
		return u, fail(403, "仅管理员可操作")
	}
	return u, nil
}

// session 创建三十天有效的随机会话，不接受客户端角色。
func (a *App) session(ctx context.Context, u User) (any, error) {
	if u.Status != "active" {
		return nil, fail(403, "账号已被停用")
	}
	token := newID() + newID()
	if err := a.Redis.Set(ctx, a.redisKey("session:"+token), u.ID, 30*24*time.Hour).Err(); err != nil {
		return nil, fail(503, "登录服务暂时不可用")
	}
	return map[string]any{"token": token, "user": u}, nil
}

// loginRate 通过 Redis 固定窗口限制身份入口滥用。
func (a *App) loginRate(r *http.Request) error {
	ip, _, _ := net.SplitHostPort(r.RemoteAddr)
	key := a.redisKey("login:" + ip + ":" + time.Now().UTC().Format("200601021504"))
	n, err := a.Redis.Incr(r.Context(), key).Result()
	if err != nil {
		return fail(503, "登录服务暂时不可用")
	}
	if n == 1 {
		a.Redis.Expire(r.Context(), key, 2*time.Minute)
	}
	if n > 120 {
		return fail(429, "登录请求过于频繁，请稍后再试")
	}
	return nil
}

// accounts 仅向开发环境暴露固定验收账号。
func (a *App) accounts(w http.ResponseWriter, r *http.Request) (any, error) {
	if !a.Config.DevMode {
		return nil, fail(404, "接口不存在")
	}
	return list[User](r.Context(), a.DB, "users", "WHERE id IN ('player','worker','clubowner','admin','outsider') ORDER BY id")
}

// devLogin 在显式开发模式下选用种子账号。
func (a *App) devLogin(w http.ResponseWriter, r *http.Request) (any, error) {
	if !a.Config.DevMode {
		return nil, fail(404, "接口不存在")
	}
	if err := a.loginRate(r); err != nil {
		return nil, err
	}
	var in struct {
		UserID string `json:"user_id"`
		consentInput
	}
	if err := decode(r, &in); err != nil {
		return nil, err
	}
	if !contains([]string{"player", "worker", "clubowner", "admin", "outsider"}, in.UserID) {
		return nil, fail(400, "请选择测试账号")
	}
	tx, err := a.DB.Begin(r.Context())
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(r.Context())
	explicit := in.Agreed || in.UserAgreementVersion != "" || in.PrivacyPolicyVersion != ""
	if explicit {
		if err = validateConsent(r.Context(), tx, in.consentInput); err != nil {
			return nil, err
		}
	}
	u, err := get[User](r.Context(), tx, "users", in.UserID, true)
	if err != nil {
		return nil, err
	}
	if explicit {
		if err = mergeIdentity(r.Context(), tx, u.ID, consentFields(in.consentInput)); err != nil {
			return nil, err
		}
	}
	if err = tx.Commit(r.Context()); err != nil {
		return nil, err
	}
	return a.session(r.Context(), u)
}

// logout 主动使当前会话失效。
func (a *App) logout(w http.ResponseWriter, r *http.Request) (any, error) {
	token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	if token != "" {
		if err := a.Redis.Del(r.Context(), a.redisKey("session:"+token)).Err(); err != nil {
			return nil, fail(503, "退出失败，请稍后重试")
		}
	}
	return map[string]bool{"ok": true}, nil
}

// me 返回当前用户安全资料。
func (a *App) me(w http.ResponseWriter, r *http.Request) (any, error) { return a.actor(r) }

// updateMe 仅允许个人修改展示资料，绝不信任客户端角色。
func (a *App) updateMe(w http.ResponseWriter, r *http.Request) (any, error) {
	u, err := a.actor(r)
	if err != nil {
		return nil, err
	}
	var in struct {
		Nickname *string `json:"nickname"`
		Bio      *string `json:"bio"`
		Provider *bool   `json:"provider_enabled"`
	}
	if err = decode(r, &in); err != nil {
		return nil, err
	}
	if in.Provider != nil && !*in.Provider {
		return nil, fail(400, "接单身份默认开启，不支持关闭")
	}
	if err = a.moderateProfileText(r, u, in.Nickname, in.Bio); err != nil {
		return nil, err
	}
	tx, err := a.DB.Begin(r.Context())
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(r.Context())
	u, err = get[User](r.Context(), tx, "users", u.ID, true)
	if err != nil {
		return nil, err
	}
	if in.Nickname != nil {
		if !length(*in.Nickname, 2, 24) {
			return nil, fail(400, "昵称需为 2 至 24 个字")
		}
		u.Nickname = strings.TrimSpace(*in.Nickname)
	}
	if in.Bio != nil {
		if !length(*in.Bio, 0, 300) {
			return nil, fail(400, "简介最多 300 字")
		}
		u.Bio = strings.TrimSpace(*in.Bio)
	}
	// 兼容旧客户端提交的接单字段，任何值都不能关闭默认接单身份。
	u.ProviderEnabled = true
	if err = put(r.Context(), tx, "users", u.ID, u); err != nil {
		return nil, err
	}
	return u, tx.Commit(r.Context())
}

// stats 从最终订单状态实时聚合，退款自然扣除收入。
func (a *App) stats(w http.ResponseWriter, r *http.Request) (any, error) {
	u, err := a.actor(r)
	if err != nil {
		return nil, err
	}
	orders, err := list[Order](r.Context(), a.DB, "orders", "WHERE data->>'buyer_id'=$1 OR data->>'provider_id'=$1", u.ID)
	if err != nil {
		return nil, err
	}
	var servicesCount int64
	if err = a.DB.QueryRow(r.Context(), "SELECT count(*) FROM services WHERE data->>'provider_id'=$1", u.ID).Scan(&servicesCount); err != nil {
		return nil, err
	}
	out := map[string]int64{"buying_count": 0, "working_count": 0, "completed_count": 0, "earnings_cents": 0, "provider_working_count": 0, "provider_completed_count": 0, "services_count": servicesCount, "buyer_pending_payment_count": 0, "buyer_working_count": 0, "buyer_delivered_count": 0, "buyer_refund_count": 0, "buyer_order_count": 0, "completed_order_count": 0}
	for _, o := range orders {
		if o.BuyerID == u.ID {
			out["buyer_order_count"]++
			if o.Status == "pending_payment" {
				out["buyer_pending_payment_count"]++
			}
			if contains(workingOrderStatuses, o.Status) {
				out["buyer_working_count"]++
			}
			if o.Status == "delivered" {
				out["buyer_delivered_count"]++
			}
			if contains([]string{"requested", "escalated", "processing"}, o.RefundStatus) || o.Status == "refunded" {
				out["buyer_refund_count"]++
			}
		}
		if o.BuyerID == u.ID && !contains([]string{"completed", "cancelled", "refunded"}, o.Status) {
			out["buying_count"]++
		}
		if o.ProviderID == u.ID && !contains([]string{"completed", "cancelled", "refunded"}, o.Status) {
			out["working_count"]++
		}
		// 新计数严格对应接单视角筛选，保留旧跨身份计数供现有页面兼容。
		if o.ProviderID == u.ID && contains(workingOrderStatuses, o.Status) {
			out["provider_working_count"]++
		}
		if o.Status == "completed" {
			out["completed_order_count"]++
			if o.ProviderID == u.ID {
				out["provider_completed_count"]++
			}
		}
		if o.Status == "completed" && !contains([]string{"requested", "escalated", "processing"}, o.RefundStatus) {
			out["completed_count"]++
			if o.ProviderID == u.ID {
				out["earnings_cents"] += settlementForOrder(o).ProviderIncomeCents
			}
		}
	}
	return out, nil
}

// getWechatOpenID 从私密身份表读取微信标识，不向用户资料接口暴露。
func (a *App) getWechatOpenID(ctx context.Context, userID string) (string, error) {
	v, err := get[map[string]string](ctx, a.DB, "identities", userID, false)
	if err != nil || v["openid"] == "" {
		return "", fail(503, "请先通过微信登录后使用微信支付")
	}
	return v["openid"], nil
}

// redisKey 隔离部署环境及测试命名空间，避免共享 Redis 的会话和限流串扰。
func (a *App) redisKey(key string) string { return a.Config.RedisPrefix + key }
