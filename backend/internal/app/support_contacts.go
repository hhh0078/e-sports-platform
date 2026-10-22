// Package app 校验平台与俱乐部公开客服联系方式；在线客服由微信原生能力承载，不建立业务聊天室。
package app

import (
	"context"
	"net/http"
	"regexp"
	"strings"
)

var supportPhonePattern = regexp.MustCompile(`^[0-9+() -]*$`)
var supportDialPhonePattern = regexp.MustCompile(`^\+?[0-9]{3,20}$`)
var supportWechatPattern = regexp.MustCompile(`^[A-Za-z0-9_-]*$`)

// validateSupportContact 校验公开联系方式；电话去除展示分隔符后须与客户端拨号规则一致，拒绝控制字符和富文本。
func validateSupportContact(phone, wechat string) error {
	number := strings.NewReplacer(" ", "", "(", "", ")", "", "-", "").Replace(phone)
	if !length(phone, 0, 32) || !supportPhonePattern.MatchString(phone) || phone != "" && !supportDialPhonePattern.MatchString(number) {
		return fail(400, "客服电话最多32字符；去除空格、括号和横线后须为3至20位数字，可带前导+")
	}
	if !length(wechat, 0, 64) || !supportWechatPattern.MatchString(wechat) {
		return fail(400, "客服微信号最多64字符，仅支持字母、数字、下划线及横线")
	}
	return nil
}

// normalizeContactPatch 仅规范化本次提交的联系方式，未提交字段由各领域保留原值。
func normalizeContactPatch(phone, wechat, qr *string) {
	for _, v := range []*string{phone, wechat, qr} {
		if v != nil {
			*v = strings.TrimSpace(*v)
		}
	}
}

// moderateContactPatch 对本次填写的电话和微信号进行同一UGC文字审核，不发送无关商户资料。
func (a *App) moderateContactPatch(r *http.Request, u User, scope string, phone, wechat *string) error {
	parts := []string{}
	if phone != nil {
		parts = append(parts, *phone)
	}
	if wechat != nil {
		parts = append(parts, *wechat)
	}
	return a.moderateText(r, u, scope, parts...)
}

// migrateSupportContactComments 注明JSONB领域中的公开客服字段含义，不修改旧配置或引入支付秘密。
func migrateSupportContactComments(ctx context.Context, q querier) error {
	_, e := q.Exec(ctx, `COMMENT ON COLUMN settings.data IS '服务端校验的配置领域数据；platform中support_qr_url为客服二维码，support_phone为客服电话，support_wechat为客服微信号，不存商户支付密钥';COMMENT ON COLUMN clubs.data IS '俱乐部领域资料；contact_qr_url为客服二维码，contact_phone为客服电话，contact_wechat为客服微信号，均为主动公开的联系信息';`)
	return e
}
