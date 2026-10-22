// Package app 验证开机电竞默认品牌升级、协议历史保护及管理员自定义持久化边界。
package app

import (
	"encoding/base64"
	"reflect"
	"strings"
	"testing"
)

// TestDefaultBrandAgreements 验证更名使用新协议版本，旧默认正文仍可完整追溯。
func TestDefaultBrandAgreements(t *testing.T) {
	legacy, current := legacyDefaultAgreements(), defaultAgreements()
	for _, pair := range [][2]agreementDocument{{legacy.UserAgreement, current.UserAgreement}, {legacy.PrivacyPolicy, current.PrivacyPolicy}} {
		if pair[0].Version != "2026-10-22.2" || pair[1].Version != "2026-10-22.3" || pair[0].Version == pair[1].Version {
			t.Fatal("品牌正文变更没有使用独立新版本")
		}
		if !strings.Contains(pair[0].Title, "集结") || !strings.Contains(pair[1].Title, "开机电竞") || strings.Contains(pair[1].Content, "集结") {
			t.Fatal("新品牌或旧版本原文不完整")
		}
		if pair[1].Content != strings.ReplaceAll(pair[0].Content, "集结", "开机电竞") {
			t.Fatal("更名修改了品牌之外的协议内容")
		}
	}
}

// TestIntegrationDefaultBrandMigration 验证真实重启升级默认配置，同时保留旧同意、聊天与订单快照。
func TestIntegrationDefaultBrandMigration(t *testing.T) {
	api := newTestAPI(t)
	key := base64.StdEncoding.EncodeToString([]byte(strings.Repeat("6", 32)))
	t.Setenv("CONFIG_ENCRYPTION_KEY", key)
	ctx := t.Context()
	legacy := legacyDefaultAgreements()
	order := api.newOrder()
	if _, err := api.App.DB.Exec(ctx, "DELETE FROM settings WHERE id IN ('default-brand-kaiji-v1','agreements') OR id LIKE 'agreement:%'"); err != nil {
		t.Fatal(err)
	}
	if err := put(ctx, api.App.DB, "settings", "platform", Settings{SiteName: "集结", Announcement: "找队友，也成为队友 · 下单前请与打手确认服务范围"}); err != nil {
		t.Fatal(err)
	}
	u, err := get[User](ctx, api.App.DB, "users", "admin", false)
	if err != nil {
		t.Fatal(err)
	}
	u.Nickname = "集结管理员"
	if err = put(ctx, api.App.DB, "users", u.ID, u); err != nil {
		t.Fatal(err)
	}
	wallet := WalletSettings{TransferRemark: "集结服务收入提现", TransferSceneID: "keep-approved-scene"}
	if err = put(ctx, api.App.DB, "settings", "wallet", wallet); err != nil {
		t.Fatal(err)
	}
	ai := defaultAISettings()
	ai.APIKey, ai.Model = "preserve-encrypted-ai-secret", "preserve-custom-model"
	ai.Prompt = strings.Replace(defaultModerationPrompt, "开机电竞", "集结", 1)
	encrypted, err := sealAISettings(ai)
	if err != nil {
		t.Fatal(err)
	}
	if err = put(ctx, api.App.DB, "settings", "ai-private", encrypted); err != nil {
		t.Fatal(err)
	}
	identity := map[string]string{"user_agreement_version": legacy.UserAgreement.Version, "privacy_policy_version": legacy.PrivacyPolicy.Version, "consented_at": "2026-10-21T00:00:00Z", "openid": "keep-private-identity"}
	if err = put(ctx, api.App.DB, "identities", "player", identity); err != nil {
		t.Fatal(err)
	}
	order.Title = "集结历史服务快照"
	if err = put(ctx, api.App.DB, "orders", order.ID, order); err != nil {
		t.Fatal(err)
	}
	message := Message{ID: "legacy-brand-message", OrderID: order.ID, SenderID: "system", SenderName: "集结平台", SenderRole: "system", Kind: "system", Content: "旧交易证据应保留原文", CreatedAt: now()}
	if err = put(ctx, api.App.DB, "messages", message.ID, message); err != nil {
		t.Fatal(err)
	}
	// AI配置不可解密时仍应允许默认品牌升级，实际审核读取继续按原规则明确失败。
	t.Setenv("CONFIG_ENCRYPTION_KEY", "")
	after, err := New(ctx, api.App.Config)
	if err != nil {
		t.Fatal(err)
	}
	defer after.Close()
	t.Setenv("CONFIG_ENCRYPTION_KEY", key)
	settings, err := get[Settings](ctx, after.DB, "settings", "platform", false)
	if err != nil || settings.SiteName != "开机电竞" {
		t.Fatalf("旧默认站点未升级：%+v %v", settings, err)
	}
	u, err = get[User](ctx, after.DB, "users", "admin", false)
	if err != nil || u.Nickname != "开机电竞管理员" {
		t.Fatalf("旧体验管理员未升级：%+v %v", u, err)
	}
	savedAI, err := after.readAISettings(ctx, after.DB)
	if err != nil || savedAI.Prompt != defaultModerationPrompt || savedAI.APIKey != ai.APIKey || savedAI.Model != ai.Model {
		t.Fatal("默认提示词升级破坏了密钥或自定义模型")
	}
	savedWallet, err := after.walletSettings(ctx)
	if err != nil || savedWallet.TransferRemark != "开机电竞服务收入提现" || savedWallet.TransferSceneID != wallet.TransferSceneID {
		t.Fatalf("提现默认备注升级破坏了场景配置：%+v %v", savedWallet, err)
	}
	current, err := readAgreements(ctx, after.DB)
	if err != nil || !sameAgreement(current.UserAgreement, defaultAgreements().UserAgreement) || !sameAgreement(current.PrivacyPolicy, defaultAgreements().PrivacyPolicy) {
		t.Fatal("未持久化的旧默认协议没有按新版本发布")
	}
	for kind, doc := range map[string]agreementDocument{"user": legacy.UserAgreement, "privacy": legacy.PrivacyPolicy} {
		history, err := get[agreementDocument](ctx, after.DB, "settings", agreementHistoryID(kind, doc.Version), false)
		if err != nil || !reflect.DeepEqual(history, doc) {
			t.Fatalf("%s旧默认协议历史被覆盖或丢失：%v", kind, err)
		}
	}
	savedIdentity, err := get[map[string]string](ctx, after.DB, "identities", "player", false)
	if err != nil || !reflect.DeepEqual(savedIdentity, identity) {
		t.Fatal("旧同意证据或微信身份被改写")
	}
	savedOrder, err := get[Order](ctx, after.DB, "orders", order.ID, false)
	if err != nil || savedOrder.Title != order.Title {
		t.Fatal("历史订单快照被更名改写")
	}
	savedMessage, err := get[Message](ctx, after.DB, "messages", message.ID, false)
	if err != nil || !reflect.DeepEqual(savedMessage, message) {
		t.Fatal("历史聊天证据被改写")
	}
	api.call("", "POST", "/auth/dev", map[string]any{"user_id": "player", "agreed": true, "user_agreement_version": legacy.UserAgreement.Version, "privacy_policy_version": legacy.PrivacyPolicy.Version}, 409, nil)
	// 迁移完成后管理员主动恢复的旧字样也属于自定义，重复启动不得再次覆盖。
	settings.SiteName = "集结"
	if err = put(ctx, after.DB, "settings", "platform", settings); err != nil {
		t.Fatal(err)
	}
	if err = after.seed(ctx); err != nil {
		t.Fatal(err)
	}
	settings, err = get[Settings](ctx, after.DB, "settings", "platform", false)
	if err != nil || settings.SiteName != "集结" {
		t.Fatal("重复启动再次执行品牌迁移")
	}
}

// TestIntegrationBrandMigrationPreservesCustom 验证管理员配置、定制条款和已占用协议版本不被迁移覆盖。
func TestIntegrationBrandMigrationPreservesCustom(t *testing.T) {
	for _, name := range []string{"custom", "published-default", "partial-default"} {
		t.Run(name, func(t *testing.T) {
			api := newTestAPI(t)
			t.Setenv("CONFIG_ENCRYPTION_KEY", base64.StdEncoding.EncodeToString([]byte(strings.Repeat("7", 32))))
			ctx := t.Context()
			if _, err := api.App.DB.Exec(ctx, "DELETE FROM settings WHERE id='default-brand-kaiji-v1' OR id LIKE 'agreement:%'"); err != nil {
				t.Fatal(err)
			}
			site := Settings{SiteName: "运营自定义站点", Announcement: "运营自定义公告"}
			if name == "published-default" {
				site = Settings{SiteName: "集结", Announcement: "找队友，也成为队友 · 下单前请与打手确认服务范围"}
				if err := audit(ctx, api.App.DB, "admin", "settings.save", "platform", "运营者已确认保存"); err != nil {
					t.Fatal(err)
				}
			}
			if err := put(ctx, api.App.DB, "settings", "platform", site); err != nil {
				t.Fatal(err)
			}
			terms := legacyDefaultAgreements()
			terms.UserAgreement.Title = "运营者定制服务协议"
			if err := put(ctx, api.App.DB, "settings", "agreements", terms); err != nil {
				t.Fatal(err)
			}
			collision := defaultAgreements().PrivacyPolicy
			collision.Title = "运营者已发布的同号自定义政策"
			if name != "partial-default" {
				if err := put(ctx, api.App.DB, "settings", agreementHistoryID("privacy", collision.Version), collision); err != nil {
					t.Fatal(err)
				}
			}
			ai := defaultAISettings()
			ai.Prompt = "运营者定制集结审核提示词，必须完整保留，不能根据包含品牌字样替换。"
			encrypted, err := sealAISettings(ai)
			if err != nil {
				t.Fatal(err)
			}
			if err = put(ctx, api.App.DB, "settings", "ai-private", encrypted); err != nil {
				t.Fatal(err)
			}
			wallet := WalletSettings{TransferRemark: "自定义集结收入"}
			if err = put(ctx, api.App.DB, "settings", "wallet", wallet); err != nil {
				t.Fatal(err)
			}
			if err = api.App.seed(ctx); err != nil {
				t.Fatal(err)
			}
			savedSite, err := get[Settings](ctx, api.App.DB, "settings", "platform", false)
			if err != nil || !reflect.DeepEqual(savedSite, site) {
				t.Fatal("运营自定义或主动保存的旧名称被改写")
			}
			savedTerms, err := readAgreements(ctx, api.App.DB)
			if name == "partial-default" {
				terms.PrivacyPolicy = defaultAgreements().PrivacyPolicy
			}
			if err != nil || !reflect.DeepEqual(savedTerms, terms) {
				t.Fatal("自定义条款或已被占用的新版本被覆盖")
			}
			if name != "partial-default" {
				savedHistory, err := get[agreementDocument](ctx, api.App.DB, "settings", agreementHistoryID("privacy", collision.Version), false)
				if err != nil || !reflect.DeepEqual(savedHistory, collision) {
					t.Fatal("不可变协议历史被覆盖")
				}
			}
			savedAI, err := api.App.readAISettings(ctx, api.App.DB)
			if err != nil || !reflect.DeepEqual(savedAI, ai) {
				t.Fatal("自定义AI提示词被机械替换")
			}
			savedWallet, err := api.App.walletSettings(ctx)
			if err != nil || !reflect.DeepEqual(savedWallet, wallet) {
				t.Fatal("自定义提现备注被机械替换")
			}
		})
	}
}
