// Package app 将明确授权的演示服务复制为真实账号的独立服务，保留原演示交易、标记与资金事实。
package app

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
)

const showcaseConversionMarker = "showcase-services-to-live-v1"
const showcaseConversionCount = 130

// ServiceConversionItem 保存一条原服务内容摘要和完整的新服务，供执行前逐项审阅。
type ServiceConversionItem struct {
	SourceID   string  `json:"source_id"`
	SourceHash string  `json:"source_hash"`
	Service    Service `json:"service"`
}

// ServiceConversionPlan 是绑定真实接单人、源内容和生成时间的不可变转换预览。
type ServiceConversionPlan struct {
	Version   int                     `json:"version"`
	OwnerID   string                  `json:"owner_id"`
	OwnerHash string                  `json:"owner_hash"`
	CreatedAt string                  `json:"created_at"`
	Items     []ServiceConversionItem `json:"items"`
	Digest    string                  `json:"digest"`
}

// serviceConversionState 只保存一次性迁移映射，不改订单、评价或资金归属。
type serviceConversionState struct {
	Digest    string            `json:"digest"`
	OwnerID   string            `json:"owner_id"`
	AppliedAt string            `json:"applied_at"`
	Mapping   map[string]string `json:"mapping"`
}

// conversionHash 对稳定JSON求摘要，仅用于检测预览之后的内容漂移。
func conversionHash(value any) (string, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	return reviewHash(string(raw)), nil
}

// conversionOwner 校验真实管理员及微信手机号，仅在进程内部读取私密身份。
func conversionOwner(ctx context.Context, q querier, id string, lock bool) (User, error) {
	u, err := get[User](ctx, q, "users", id, lock)
	if err != nil {
		return u, err
	}
	if strings.HasPrefix(id, "demo_") || u.Status != "active" || u.Role != "admin" {
		return u, fmt.Errorf("转换接单人必须是有效的真实平台管理员")
	}
	i, err := get[map[string]string](ctx, q, "identities", id, lock)
	if err != nil || i["openid"] == "" || !phoneDigits.MatchString(i["phone_number"]) || i["phone_verified_at"] == "" || i["phone_source"] != "wechat" {
		return u, fmt.Errorf("转换接单人缺少真实微信或微信手机号验证")
	}
	if !length(u.Bio, 2, 300) {
		return u, fmt.Errorf("转换接单人需要有效的个人简介")
	}
	return u, nil
}

// hasShowcaseWording 检查公开服务是否仍含来源演示文案，不替换历史订单或评价。
func hasShowcaseWording(value string) bool {
	for _, word := range []string{"测试", "示例", "演示", "源码展示", "不可付款"} {
		if strings.Contains(value, word) {
			return true
		}
	}
	return false
}

// convertedShowcaseService 仅清理约定的展示内容，真实成交和评分均从零开始。
func convertedShowcaseService(source Service, owner User, stamp string, game Game) (Service, error) {
	s := source
	s.ID = reviewHash(showcaseConversionMarker + "\n" + owner.ID + "\n" + source.ID)[:32]
	s.ProviderID, s.ProviderName, s.ProviderAvatar = owner.ID, owner.Nickname, owner.Avatar
	s.ClubID, s.ClubName, s.ClubVerified = "", "", false
	s.ProviderDepositCents, s.ProviderClubDepositCents = 0, 0
	s.CompletedCount, s.Rating, s.Reviews = 0, 0, nil
	s.Status, s.CreatedAt = "active", stamp
	s.Title = strings.TrimSuffix(s.Title, "·演示")
	s.Description = strings.ReplaceAll(s.Description, "此为源码展示服务，不可付款下单。", "")
	s.Description = strings.ReplaceAll(s.Description, "所有图片文字均为可编辑展示示例。", "")
	s.Labels, s.Tags = []CoverLabel{}, []string{}
	for _, label := range source.Labels {
		if !hasShowcaseWording(label.Text) {
			label.ID = fmt.Sprintf("label-%d", len(s.Labels)+1)
			s.Labels = append(s.Labels, label)
		}
	}
	for _, tag := range source.Tags {
		if !hasShowcaseWording(tag) {
			s.Tags = append(s.Tags, tag)
		}
	}
	if hasShowcaseWording(s.Title + s.Description + s.ProviderName + s.ClubName) {
		return s, fmt.Errorf("服务 %s 清理后仍含展示文案，拒绝自动转换", source.ID)
	}
	if err := validateService(&s, game); err != nil {
		return s, err
	}
	return s, nil
}

// conversionDigest 排除摘要自身，绑定待执行预览的全部公开内容。
func conversionDigest(plan ServiceConversionPlan) (string, error) {
	plan.Digest = ""
	return conversionHash(plan)
}

// PlanShowcaseServices 在只读事务中生成精确转换清单，不初始化种子、不创建会话或交易。
func (a *App) PlanShowcaseServices(ctx context.Context, ownerID string) (ServiceConversionPlan, error) {
	plan := ServiceConversionPlan{Version: 1, OwnerID: ownerID, CreatedAt: now(), Items: []ServiceConversionItem{}}
	tx, err := a.DB.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return plan, err
	}
	defer tx.Rollback(ctx)
	owner, err := conversionOwner(ctx, tx, ownerID, false)
	if err != nil {
		return plan, err
	}
	plan.OwnerHash, err = conversionHash(owner)
	if err != nil {
		return plan, err
	}
	if _, err = get[serviceConversionState](ctx, tx, "settings", showcaseConversionMarker, false); err == nil {
		return plan, fmt.Errorf("服务已转换，请使用 verify 核对，不重复生成")
	} else if !missing(err) {
		return plan, err
	}
	sources, err := list[map[string]any](ctx, tx, "services", "WHERE left(id,5)='demo_' ORDER BY id")
	if err != nil {
		return plan, err
	}
	if len(sources) != showcaseConversionCount {
		return plan, fmt.Errorf("预期130条原演示服务，实际%d条，拒绝扩大转换范围", len(sources))
	}
	for _, raw := range sources {
		id := reviewString(raw, "id")
		if raw["is_demo"] != true || raw["demo_version"] != float64(1) || reviewString(raw, "status") != "active" {
			return plan, fmt.Errorf("原服务 %s 标记或状态不符合转换前提", id)
		}
		source, err := get[Service](ctx, tx, "services", id, false)
		if err != nil {
			return plan, err
		}
		game, err := get[Game](ctx, tx, "games", source.GameID, false)
		if err != nil || !game.Enabled {
			return plan, fmt.Errorf("原服务 %s 的游戏不可用", id)
		}
		service, err := convertedShowcaseService(source, owner, plan.CreatedAt, game)
		if err != nil {
			return plan, err
		}
		hash, err := conversionHash(raw)
		if err != nil {
			return plan, err
		}
		plan.Items = append(plan.Items, ServiceConversionItem{SourceID: id, SourceHash: hash, Service: service})
	}
	plan.Digest, err = conversionDigest(plan)
	return plan, err
}

// ApplyShowcaseServices 原子校验预览、发布独立真实服务和下架原服务，任何漂移均整笔回滚。
func (a *App) ApplyShowcaseServices(ctx context.Context, plan ServiceConversionPlan) (map[string]int, error) {
	digest, err := conversionDigest(plan)
	if err != nil || digest != plan.Digest || plan.Version != 1 || len(plan.Items) != showcaseConversionCount {
		return nil, fmt.Errorf("转换预览摘要、版本或数量不正确")
	}
	tx, err := a.DB.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, "SELECT pg_advisory_xact_lock(8173040)"); err != nil {
		return nil, err
	}
	state, err := get[serviceConversionState](ctx, tx, "settings", showcaseConversionMarker, true)
	if err == nil {
		if state.Digest != plan.Digest || state.OwnerID != plan.OwnerID {
			return nil, fmt.Errorf("已有其他转换记录，拒绝覆盖")
		}
		return map[string]int{"created": 0, "paused": 0, "already_applied": len(state.Mapping)}, nil
	} else if !missing(err) {
		return nil, err
	}
	owner, err := conversionOwner(ctx, tx, plan.OwnerID, true)
	if err != nil {
		return nil, err
	}
	ownerHash, err := conversionHash(owner)
	if err != nil || ownerHash != plan.OwnerHash {
		return nil, fmt.Errorf("接单人资料已变动，请重新生成转换预览")
	}
	state = serviceConversionState{Digest: plan.Digest, OwnerID: owner.ID, AppliedAt: now(), Mapping: map[string]string{}}
	review := &reviewRequest{App: a, ActorID: owner.ID}
	reviewCtx := context.WithValue(ctx, reviewContextKey{}, review)
	for _, item := range plan.Items {
		if state.Mapping[item.SourceID] != "" || !strings.HasPrefix(item.SourceID, "demo_") {
			return nil, fmt.Errorf("预览含重复或非演示源服务")
		}
		raw, err := get[map[string]any](ctx, tx, "services", item.SourceID, true)
		if err != nil {
			return nil, err
		}
		sourceHash, err := conversionHash(raw)
		if err != nil || sourceHash != item.SourceHash || raw["is_demo"] != true || raw["demo_version"] != float64(1) {
			return nil, fmt.Errorf("原服务 %s 已变动，转换未执行", item.SourceID)
		}
		source, err := get[Service](ctx, tx, "services", item.SourceID, false)
		if err != nil {
			return nil, err
		}
		game, err := get[Game](ctx, tx, "games", source.GameID, true)
		if err != nil || !game.Enabled {
			return nil, fmt.Errorf("原服务 %s 的游戏已停用", item.SourceID)
		}
		expected, err := convertedShowcaseService(source, owner, plan.CreatedAt, game)
		if err != nil {
			return nil, err
		}
		want, _ := conversionHash(expected)
		got, _ := conversionHash(item.Service)
		if want != got {
			return nil, fmt.Errorf("服务 %s 的预览内容不符合转换规则", item.SourceID)
		}
		if _, err = get[Service](ctx, tx, "services", expected.ID, true); !missing(err) {
			return nil, fmt.Errorf("新服务编号已存在或读取失败，拒绝覆盖")
		}
		if err = put(reviewCtx, tx, "services", expected.ID, expected); err != nil {
			return nil, err
		}
		raw["status"] = "paused"
		if err = put(ctx, tx, "services", item.SourceID, raw); err != nil {
			return nil, err
		}
		if err = audit(ctx, tx, owner.ID, "service.convert_showcase", expected.ID, "经管理员授权承接；原服务="+item.SourceID+"；原成交与评价未迁移"); err != nil {
			return nil, err
		}
		state.Mapping[item.SourceID] = expected.ID
	}
	if err = put(ctx, tx, "settings", showcaseConversionMarker, state); err != nil {
		return nil, err
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	return map[string]int{"created": len(state.Mapping), "paused": len(state.Mapping), "pending_review_fields": len(review.TaskIDs)}, nil
}

// VerifyShowcaseServices 只读核对真实服务归属、下架状态及零虚构成交，不创建订单或付款。
func (a *App) VerifyShowcaseServices(ctx context.Context) (map[string]int, error) {
	tx, err := a.DB.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	state, err := get[serviceConversionState](ctx, tx, "settings", showcaseConversionMarker, false)
	if err != nil {
		return nil, err
	}
	if _, err = conversionOwner(ctx, tx, state.OwnerID, false); err != nil {
		return nil, err
	}
	counts := map[string]int{"mapped": len(state.Mapping), "old_paused": 0, "live_ready": 0}
	for oldID, newID := range state.Mapping {
		old, err := get[map[string]any](ctx, tx, "services", oldID, false)
		if err != nil || reviewString(old, "status") != "paused" || old["is_demo"] != true {
			return nil, fmt.Errorf("原演示服务下架或标记异常")
		}
		counts["old_paused"]++
		s, err := get[Service](ctx, tx, "services", newID, false)
		if err != nil || s.ProviderID != state.OwnerID || s.ClubID != "" || s.ClubVerified || s.CompletedCount != 0 || s.Rating != 0 || s.Status != "active" {
			return nil, fmt.Errorf("真实服务归属、状态或初始成交记录异常")
		}
		if err = requireLiveOrder(Order{ID: "preflight", BuyerID: "preflight-buyer", ProviderID: s.ProviderID, ServiceID: s.ID}); err != nil {
			return nil, err
		}
		g, err := get[Game](ctx, tx, "games", s.GameID, false)
		if err != nil || !g.Enabled {
			return nil, fmt.Errorf("真实服务对应游戏不可用")
		}
		if err = validateService(&s, g); err != nil {
			return nil, err
		}
		counts["live_ready"]++
	}
	return counts, nil
}
