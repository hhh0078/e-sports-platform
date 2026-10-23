// Package app 只读查询当前小程序的微信订阅模板，供管理员按真实模板字段配置业务提醒。
package app

import (
	"context"
	"net/http"
	"net/url"
	"regexp"
	"strings"
)

// SubscriptionTemplateField 描述官方模板中的实际字段，不包含示例联系人或身份凭证。
type SubscriptionTemplateField struct {
	Key   string `json:"key"`
	Label string `json:"label"`
	Type  string `json:"type"`
}

// SubscriptionWechatTemplate 是当前AppID可使用的官方模板，只向平台管理员返回。
type SubscriptionWechatTemplate struct {
	TemplateID string                      `json:"template_id"`
	Title      string                      `json:"title"`
	Type       int                         `json:"type"`
	Content    string                      `json:"content"`
	Fields     []SubscriptionTemplateField `json:"fields"`
}

var subscriptionTemplatePlaceholder = regexp.MustCompile(`\{\{([a-z_]+)([0-9]+)\.DATA\}\}`)

// subscriptionTemplateTitleAllowed 将六类提醒限制到用途匹配的官方标题，不能用退款申请模板发送接单或入驻消息。
func subscriptionTemplateTitleAllowed(scene, title string) bool {
	allowed := map[string][]string{
		"provider_order":  {"交易提醒", "新订单通知", "订单付款通知"},
		"provider_refund": {"退款申请通知", "退款申请提醒"},
		"buyer_accepted":  {"接单成功提醒", "接单成功通知", "订单接单通知"},
		"club_member":     {"新成员邀请通知", "新成员申请通知", "入会申请通知"},
		"club_dispute":    {"订单异常通知", "订单争议通知"},
		"admin_club":      {"商家入驻申请通知", "俱乐部申请通知", "新俱乐部申请通知"},
	}
	for _, candidate := range allowed[scene] {
		if strings.TrimSpace(title) == candidate {
			return true
		}
	}
	return false
}

// subscriptionTemplateFields 提取官方字段和显示名称，按出现顺序去重并限制数量。
func subscriptionTemplateFields(content string) []SubscriptionTemplateField {
	fields := []SubscriptionTemplateField{}
	seen := map[string]bool{}
	for _, line := range strings.Split(content, "\n") {
		match := subscriptionTemplatePlaceholder.FindStringSubmatchIndex(line)
		if match == nil {
			continue
		}
		kind := line[match[2]:match[3]]
		key := kind + line[match[4]:match[5]]
		if seen[key] || len(fields) >= 32 {
			continue
		}
		seen[key] = true
		label := strings.TrimSpace(strings.TrimRight(strings.TrimSpace(line[:match[0]]), ":："))
		if label == "" || len([]rune(label)) > 80 {
			label = key
		}
		fields = append(fields, SubscriptionTemplateField{Key: key, Label: label, Type: kind})
	}
	return fields
}

// fetchSubscriptionTemplates 仅向固定微信HTTPS地址查询模板，失效访问凭证最多刷新一次。
func (a *App) fetchSubscriptionTemplates(ctx context.Context) ([]SubscriptionWechatTemplate, error) {
	if err := a.wechatConfigured(); err != nil {
		return nil, fail(503, "请先配置当前小程序的服务端微信凭证，再读取官方模板")
	}
	for attempt := 0; attempt < 2; attempt++ {
		token, err := a.wechatAccessToken(ctx, attempt > 0)
		if err != nil {
			return nil, fail(503, "无法取得微信访问凭证，请检查小程序服务端配置")
		}
		var result struct {
			ErrCode int `json:"errcode"`
			Data    []struct {
				TemplateID string `json:"priTmplId"`
				Title      string `json:"title"`
				Type       int    `json:"type"`
				Content    string `json:"content"`
			} `json:"data"`
		}
		address := "https://api.weixin.qq.com/wxaapi/newtmpl/gettemplate?" + url.Values{"access_token": {token}}.Encode()
		if err = a.wechatJSON(ctx, http.MethodGet, address, nil, &result); err != nil {
			return nil, fail(502, "微信模板列表暂时不可用，请稍后重试")
		}
		if attempt == 0 && (result.ErrCode == 40001 || result.ErrCode == 40014 || result.ErrCode == 42001) {
			continue
		}
		if result.ErrCode != 0 {
			return nil, fail(502, "微信未允许读取订阅模板，请核对小程序权限、IP白名单及后台模板配置")
		}
		if len(result.Data) > 100 {
			return nil, fail(502, "微信模板列表数量异常，请稍后重试")
		}
		items := []SubscriptionWechatTemplate{}
		for _, item := range result.Data {
			if !length(item.TemplateID, 1, 128) || !length(item.Title, 1, 100) || len(item.Content) > 12000 {
				return nil, fail(502, "微信模板字段格式异常，请稍后重试")
			}
			items = append(items, SubscriptionWechatTemplate{TemplateID: item.TemplateID, Title: strings.TrimSpace(item.Title), Type: item.Type, Content: item.Content, Fields: subscriptionTemplateFields(item.Content)})
		}
		return items, nil
	}
	return nil, fail(502, "微信访问凭证已失效，请稍后重试")
}

// subscriptionTemplates 鉴权后返回当前账号的实际模板，读取行为不授权、不发送订阅消息。
func (a *App) subscriptionTemplates(w http.ResponseWriter, r *http.Request) (any, error) {
	if _, err := a.admin(r); err != nil {
		return nil, err
	}
	items, err := a.fetchSubscriptionTemplates(r.Context())
	if err != nil {
		return nil, err
	}
	return map[string]any{"items": items}, nil
}

// validateOfficialSubscriptionSettings 在正式启用前核对当前AppID的实际一次性模板和必填字段，禁用草稿不依赖微信网络。
func (a *App) validateOfficialSubscriptionSettings(ctx context.Context, settings SubscriptionSettings) error {
	if !settings.Enabled || a.Config.DevMode {
		return nil
	}
	items, err := a.fetchSubscriptionTemplates(ctx)
	if err != nil {
		return err
	}
	byID := map[string]SubscriptionWechatTemplate{}
	for _, item := range items {
		byID[item.TemplateID] = item
	}
	for _, scene := range settings.Scenes {
		if !scene.Enabled {
			continue
		}
		template, ok := byID[scene.TemplateID]
		if !ok {
			return fail(400, "启用前请从当前小程序的官方模板列表选择有效模板")
		}
		if template.Type != 2 {
			return fail(400, "本功能仅支持普通一次性订阅模板，不能混用长期或新版服务动态模板")
		}
		if !subscriptionTemplateTitleAllowed(scene.Scene, template.Title) {
			return fail(400, "微信模板用途与提醒场景不匹配，请选择对应接单、退款、成员或俱乐部申请模板")
		}
		if len(template.Fields) == 0 || len(scene.Fields) != len(template.Fields) {
			return fail(400, "模板字段需与官方必填字段完整一致，请重新选择模板并配置来源")
		}
		for _, field := range template.Fields {
			if _, ok := scene.Fields[field.Key]; !ok {
				return fail(400, "模板字段与官方配置不一致，请重新读取模板")
			}
		}
	}
	return nil
}
