// Package app 管理公开协议版本、管理员修订及登录时的明确同意记录。
package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strings"
)

type agreementDocument struct {
	Version   string `json:"version"`
	Title     string `json:"title"`
	Content   string `json:"content"`
	UpdatedAt string `json:"updated_at"`
}
type agreementSet struct {
	UserAgreement agreementDocument `json:"user_agreement"`
	PrivacyPolicy agreementDocument `json:"privacy_policy"`
}
type consentInput struct {
	Agreed               bool   `json:"agreed"`
	UserAgreementVersion string `json:"user_agreement_version"`
	PrivacyPolicyVersion string `json:"privacy_policy_version"`
}

// defaultAgreements 提供可阅读的初始条款，管理员可按真实运营主体修订后发布新版本。
func defaultAgreements() agreementSet {
	return agreementSet{
		UserAgreement: agreementDocument{Version: "2026-10-22.2", Title: "集结用户服务协议", UpdatedAt: "2026-10-22T00:00:00Z", Content: "欢迎使用集结游戏服务平台。请完整阅读并自主决定是否同意本协议。\n\n一、平台与身份\n集结提供游戏陪伴、组队协作和合法任务服务的信息发布与交易管理。同一账号可以发单，也默认拥有接单身份，无需另行开启。俱乐部挂靠为平台内组织关系，认证图标不代表游戏官方背书。请提供真实、合法的资料；不得冒用身份、欺诈、骚扰或侵犯他人权益。\n\n二、服务约定\n下单前请核对游戏、服务内容、时长、单价、验收标准和预计交付时间。禁止外挂、脚本作弊、账号买卖、赌博、色情服务及其他违法或违反游戏规则的内容。请勿在聊天中提供账号密码、支付密码或验证码。服务方不得私自变更已确认条件。未成年人应遵守游戏防沉迷规定，并在监护人指导和授权下使用。\n\n三、订单与争议\n付款、接单、服务完成及买方确认均以平台订单记录为准。退款请求可由双方协商；无法达成一致时可以提交平台介入。平台结合订单约定、聊天和证据处理争议，相关俱乐部管理员可在授权范围参与。评价应真实客观。\n\n四、平台管理\n平台可以审核违规内容、处理投诉并限制或冻结违规账号，管理操作保留审计。用户可在个人资料页更新信息，并通过订单争议入口或平台管理员申请信息更正、删除和账号停用。用户提交的文字和图片需通过内容审核；审核拒绝会说明原因，审核服务故障时暂停提交。平台系统通知首次为未读，阅读后保存在当前设备并删除服务器通知副本；订单聊天及争议证据不适用该删除规则。\n\n五、本地测试说明与条款更新\n当前开发环境内的测试支付、测试手机号和演示账号仅用于验收，不代表真实资金结算或手机运营商认证。正式商户和运营主体信息应在上线前由运营者补齐。条款更新将发布新版本；登录时应阅读并明确同意当前用户协议和隐私政策。"},
		PrivacyPolicy: agreementDocument{Version: "2026-10-22.2", Title: "集结隐私政策", UpdatedAt: "2026-10-22T00:00:00Z", Content: "本政策说明集结如何处理使用平台时必需的个人信息。\n\n一、收集目的和范围\n启动小程序时，平台可通过微信官方登录接口取得小程序内的身份标识，仅用于短期识别凭据；在你主动勾选同意并登录前，不创建平台用户及长期登录会话。登录后保存平台昵称、头像、个人简介和你同意的协议版本与时间。所有账号默认拥有接单身份；发布和交易仍需满足手机号验证及平台规则。\n\n二、手机号授权\n发布服务、发布需求和下单前需要完成手机号验证，用于账户安全和交易联系。你点击微信手机号授权后，服务器以官方一次性凭证获取手机号；拒绝授权仍可以浏览公开内容。原始手机号保存在私密身份记录中，公开资料仅展示验证状态和脱敏号码，不向其他用户公开完整手机号。开发环境的测试绑定有明确标识。\n\n三、交易信息\n我们保存服务、订单、支付渠道结果、评价、订单沟通及你主动上传的证据，以履行交易、处理退款和争议。订单原始证据只允许有权限的交易参与者、相关俱乐部管理者及平台管理员按职责访问。请避免提交无关敏感信息。\n\n四、安全与共享\n微信身份标识、登录会话、支付密钥不公开。手机号和身份记录不会出现在公开用户列表。支付必要数据由微信支付处理；平台不收集银行卡密码。管理员访问受权限控制，重要操作保留审计。我们不会出售个人信息。为审查违法违规内容，你主动提交的昵称、简介、服务、俱乐部、需求、订单沟通、评价及图片（含图片中文字）会发送给运营者在后台配置的AI服务。处理范围仅限本次必要内容，不发送无关手机号认证记录、微信身份秘密、支付密钥或提现实名资料。运营者应在正式上线前明确AI服务提供方、数据处理地点和保存期限，并更新本政策。审核结果可能存在错误，你可以修改内容后重试或联系平台申诉。\n\n五、保存、权利与联系\n数据仅按实现功能、履行订单、解决争议及法律要求所必需期限保存。你可在个人资料页查看和修改资料，通过平台管理员申请更正、删除、撤回授权或停用账号；未结订单和依法须留存记录需先完成处理。退出登录会删除当前服务器会话。消息中心的系统通知在本机成功保存并阅读后会通知服务器删除该通知副本；已读全文仅在当前设备按账号保留，退出登录不会清除，清理应用缓存或更换设备可能丢失本机历史。未读通知仍保存在服务器；订单聊天、原始证据和依法应留存审计记录不随系统通知删除。正式上线前运营者应补充具体主体、联系方式及数据保存期限。\n\n六、政策更新\n政策内容更新使用独立版本，登录时展示当前政策并记录你的明确同意。"},
	}
}

// readAgreements 读取当前公开条款，首次使用时回退到完整默认文本。
func readAgreements(ctx context.Context, q querier) (agreementSet, error) {
	v, err := get[agreementSet](ctx, q, "settings", "agreements", false)
	if missing(err) {
		return defaultAgreements(), nil
	}
	return v, err
}

// agreements 允许访客在登录前阅读两份当前条款。
func (a *App) agreements(w http.ResponseWriter, r *http.Request) (any, error) {
	return readAgreements(r.Context(), a.DB)
}

// adminAgreements 在管理端读取同一份公开条款并强制管理员身份。
func (a *App) adminAgreements(w http.ResponseWriter, r *http.Request) (any, error) {
	if _, e := a.admin(r); e != nil {
		return nil, e
	}
	return readAgreements(r.Context(), a.DB)
}

// agreementHistoryID 将版本映射为固定长度历史主键，防止任意版本字符串影响数据布局。
func agreementHistoryID(kind, version string) string {
	h := sha256.Sum256([]byte(version))
	return "agreement:" + kind + ":" + hex.EncodeToString(h[:])
}

// sameAgreement 判断相同版本的正文和标题是否完全一致。
func sameAgreement(a, b agreementDocument) bool {
	return a.Version == b.Version && a.Title == b.Title && a.Content == b.Content
}

// saveAgreements 以串行事务发布条款并保留不可变版本，拒绝旧版本被覆盖。
func (a *App) saveAgreements(w http.ResponseWriter, r *http.Request) (any, error) {
	u, err := a.admin(r)
	if err != nil {
		return nil, err
	}
	var in agreementSet
	if err = decode(r, &in); err != nil {
		return nil, err
	}
	tx, err := a.DB.Begin(r.Context())
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(r.Context())
	if _, err = tx.Exec(r.Context(), "SELECT pg_advisory_xact_lock(8173020)"); err != nil {
		return nil, err
	}
	current, err := readAgreements(r.Context(), tx)
	if err != nil {
		return nil, err
	}
	pairs := []struct {
		kind string
		next *agreementDocument
		old  agreementDocument
	}{{"user", &in.UserAgreement, current.UserAgreement}, {"privacy", &in.PrivacyPolicy, current.PrivacyPolicy}}
	for _, p := range pairs {
		p.next.Version = strings.TrimSpace(p.next.Version)
		p.next.Title = strings.TrimSpace(p.next.Title)
		p.next.Content = strings.TrimSpace(p.next.Content)
		if !length(p.next.Version, 1, 64) || !length(p.next.Title, 2, 80) || !length(p.next.Content, 30, 8000) {
			return nil, fail(400, "协议需填写版本、标题和完整正文（30 至 8000 字）")
		}
		if err = put(r.Context(), tx, "settings", agreementHistoryID(p.kind, p.old.Version), p.old); err != nil {
			return nil, err
		}
		history, e := get[agreementDocument](r.Context(), tx, "settings", agreementHistoryID(p.kind, p.next.Version), false)
		if e != nil && !missing(e) {
			return nil, e
		}
		if e == nil && !sameAgreement(history, *p.next) {
			return nil, fail(409, "正文变更必须使用新版本号，历史版本不能覆盖")
		}
		if e == nil {
			p.next.UpdatedAt = history.UpdatedAt
		} else {
			p.next.UpdatedAt = now()
		}
		if err = put(r.Context(), tx, "settings", agreementHistoryID(p.kind, p.next.Version), p.next); err != nil {
			return nil, err
		}
	}
	if err = put(r.Context(), tx, "settings", "agreements", in); err != nil {
		return nil, err
	}
	if err = audit(r.Context(), tx, u.ID, "agreements.save", "agreements", in.UserAgreement.Version+" / "+in.PrivacyPolicy.Version); err != nil {
		return nil, err
	}
	return in, tx.Commit(r.Context())
}

// validateConsent 在协议写入相同事务锁下核对当前版本，避免勾选旧条款建立会话。
func validateConsent(ctx context.Context, q querier, in consentInput) error {
	if !in.Agreed {
		return fail(400, "请阅读并同意用户服务协议和隐私政策")
	}
	if _, err := q.Exec(ctx, "SELECT pg_advisory_xact_lock(8173020)"); err != nil {
		return err
	}
	current, err := readAgreements(ctx, q)
	if err != nil {
		return err
	}
	if in.UserAgreementVersion != current.UserAgreement.Version || in.PrivacyPolicyVersion != current.PrivacyPolicy.Version {
		return fail(409, "协议已更新，请重新阅读并同意当前版本")
	}
	return nil
}

// mergeIdentity 在调用者持有用户行锁时合并私密字段，避免登录或手机绑定互相覆盖。
func mergeIdentity(ctx context.Context, q querier, id string, fields map[string]string) error {
	v, err := get[map[string]string](ctx, q, "identities", id, false)
	if err != nil && !missing(err) {
		return err
	}
	if v == nil {
		v = map[string]string{}
	}
	for key, value := range fields {
		v[key] = value
	}
	return put(ctx, q, "identities", id, v)
}

// consentFields 生成仅存储于私密身份记录的用户明确同意审计字段。
func consentFields(in consentInput) map[string]string {
	return map[string]string{"user_agreement_version": in.UserAgreementVersion, "privacy_policy_version": in.PrivacyPolicyVersion, "consented_at": now()}
}
