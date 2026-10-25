// Package app 使用服务端硬编码登记所有用户图文字段，禁止客户端指定数据库表或JSON路径。
package app

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

const rejectedContentText = "审核失败，请查看消息"
const rejectedImageURL = "/api/media/moderation-rejected.png"

// reviewField 表示一个已登记的内容字段，交易证据仅改变对外投影。
type reviewField struct {
	Code        string
	Label       string
	Kind        string
	Value       string
	MediaID     string
	Path        []string
	Protected   bool
	Replacement string
}

// reviewString 从JSON对象读取纯字符串，缺失字段返回空值。
func reviewString(v map[string]any, key string) string { s, _ := v[key].(string); return s }

// reviewHash 对任务身份或内容求稳定摘要，不在日志中记录原始违规材料。
func reviewHash(s string) string { h := sha256.Sum256([]byte(s)); return hex.EncodeToString(h[:]) }

// reviewOwner 从服务端领域数据确定内容所属账号，不使用客户端审核参数。
func reviewOwner(table string, data map[string]any, actor string) string {
	if table == "orders" || table == "members" || table == "inbox" {
		return actor
	}
	key := map[string]string{"users": "id", "services": "provider_id", "clubs": "owner_id", "members": "user_id", "orders": "buyer_id", "messages": "sender_id", "evidence": "owner_id", "order_adjustments": "proposer_id", "inbox": "user_id"}[table]
	if id := reviewString(data, key); id != "" && id != "system" {
		return id
	}
	return actor
}

// registeredReviewFields 抽取当前真实可编辑文字和托管图片；协议、认证、资金秘密不是UGC审核对象。
func registeredReviewFields(table, id string, v map[string]any) []reviewField {
	out := []reviewField{}
	protected := table == "orders" || table == "messages" || table == "order_adjustments" || table == "reviews"
	text := func(key, label string) {
		if s := reviewString(v, key); strings.TrimSpace(s) != "" {
			out = append(out, reviewField{Code: key, Label: label, Kind: "text", Value: s, Path: []string{key}, Protected: protected, Replacement: rejectedContentText})
		}
	}
	image := func(key, label string) {
		raw := reviewString(v, key)
		if media := reviewMediaID(raw); media != "" {
			out = append(out, reviewField{Code: key, Label: label, Kind: "image", Value: raw, MediaID: media, Path: []string{key}, Protected: protected, Replacement: rejectedImageURL})
		}
	}
	stringsList := func(key, label string) {
		if xs, ok := v[key].([]any); ok {
			for i, x := range xs {
				if s, ok := x.(string); ok && strings.TrimSpace(s) != "" {
					out = append(out, reviewField{Code: fmt.Sprintf("%s.%d", key, i), Label: fmt.Sprintf("%s%d", label, i+1), Kind: "text", Value: s, Path: []string{key, strconv.Itoa(i)}, Protected: protected, Replacement: rejectedContentText})
				}
			}
		}
	}
	switch table {
	case "users":
		text("nickname", "昵称")
		text("bio", "个人简介")
		image("avatar", "头像")
		image("profile_cover", "个人封面")
	case "services":
		text("title", "服务标题")
		text("description", "服务说明")
		text("unit", "计价单位")
		stringsList("tags", "服务标签")
		image("cover_url", "服务封面")
		if xs, ok := v["labels"].([]any); ok {
			for i, x := range xs {
				if block, ok := x.(map[string]any); ok {
					if s := reviewString(block, "text"); s != "" {
						out = append(out, reviewField{Code: "label." + reviewString(block, "id"), Label: "封面文案", Kind: "text", Value: s, Path: []string{"labels", strconv.Itoa(i), "text"}, Replacement: rejectedContentText})
					}
				}
			}
		}
	case "clubs":
		for _, f := range [][2]string{{"name", "俱乐部名称"}, {"description", "俱乐部介绍"}, {"assessment", "考核说明"}, {"join_note", "加入备注"}, {"contact_phone", "俱乐部联系电话"}, {"contact_wechat", "俱乐部联系微信"}} {
			text(f[0], f[1])
		}
		image("cover_url", "俱乐部封面")
		image("avatar_url", "俱乐部头像")
		image("contact_qr_url", "俱乐部联系二维码")
		if xs, ok := v["detail_blocks"].([]any); ok {
			for i, x := range xs {
				if b, ok := x.(map[string]any); ok {
					key, kind := "text", "text"
					if reviewString(b, "type") == "image" {
						key, kind = "image_url", "image"
					}
					s := reviewString(b, key)
					if s != "" {
						field := reviewField{Code: fmt.Sprintf("detail.%d.%s", i, key), Label: fmt.Sprintf("俱乐部详情第%d段", i+1), Kind: kind, Value: s, Path: []string{"detail_blocks", strconv.Itoa(i), key}, Replacement: rejectedContentText}
						if kind == "image" {
							field.MediaID = reviewMediaID(s)
							field.Replacement = rejectedImageURL
							if field.MediaID == "" {
								continue
							}
						}
						out = append(out, field)
					}
				}
			}
		}
	case "members":
		text("application_note", "入会申请说明")
		text("review_note", "成员审核说明")
	case "orders":
		if reviewString(v, "service_id") == "" {
			text("title", "需求标题")
			text("service_description", "需求说明快照")
		}
		for _, f := range [][2]string{{"requirements", "订单需求"}, {"delivery_note", "交付说明"}, {"refund_reason", "退款原因"}, {"refund_reply", "售后回复"}, {"dispute_reason", "争议说明"}} {
			text(f[0], f[1])
		}
	case "messages":
		// 系统事件仅引用已经单独登记的订单或调价说明，显示投影按order_id屏蔽，不重复送AI。
		if reviewString(v, "kind") == "system" {
			break
		}
		text("content", "订单沟通内容")
		image("image_url", "订单沟通图片")
	case "reviews":
		text("content", "订单评价")
	case "order_adjustments":
		text("reason", "订单调价说明")
	case "evidence":
		if reviewString(v, "data") != "" && reviewString(v, "moderation") != "trusted_platform" {
			out = append(out, reviewField{Code: "image", Label: "上传图片", Kind: "image", Value: reviewString(v, "sha256"), MediaID: id, Protected: !reviewBool(v, "public"), Replacement: rejectedImageURL})
		}
	case "settings":
		if id == "platform" {
			text("site_name", "平台名称")
			text("announcement", "平台公告")
			text("support_phone", "客服电话")
			text("support_wechat", "客服微信")
			image("support_qr_url", "客服二维码")
		}
	case "games":
		text("name", "游戏名称")
		text("short_name", "游戏简称")
		text("subtitle", "游戏介绍")
		stringsList("categories", "服务分类")
		image("cover_url", "游戏封面")
		image("icon_url", "游戏图标")
		if xs, ok := v["covers"].([]any); ok {
			for i, x := range xs {
				if s, ok := x.(string); ok {
					if media := reviewMediaID(s); media != "" {
						out = append(out, reviewField{Code: fmt.Sprintf("covers.%d", i), Label: "游戏素材", Kind: "image", Value: s, MediaID: media, Path: []string{"covers", strconv.Itoa(i)}, Replacement: rejectedImageURL})
					}
				}
			}
		}
	case "inbox":
		text("title", "平台消息标题")
		text("body", "平台消息正文")
	}
	out = append(out, registeredPublicContentReviewFields(table, id, v)...)
	for i := range out {
		if out[i].Code == "unit" {
			out[i].Replacement = "审核未通过"
		}
		if strings.Contains(out[i].Code, "phone") || strings.Contains(out[i].Code, "wechat") {
			out[i].Replacement = ""
		}
	}
	return out
}

// reviewBool 读取由服务端结构体编码的布尔值。
func reviewBool(v map[string]any, key string) bool { b, _ := v[key].(bool); return b }

// reviewMediaID 仅识别本站公开图片和私密证据路径，忽略临时签名查询参数。
func reviewMediaID(raw string) string {
	raw = strings.SplitN(raw, "?", 2)[0]
	for _, prefix := range []string{"/api/media/", "/api/evidence/"} {
		if strings.HasPrefix(raw, prefix) {
			id := strings.TrimPrefix(raw, prefix)
			if len(id) == 32 {
				if _, err := hex.DecodeString(id); err == nil {
					return id
				}
			}
		}
	}
	return ""
}

// decodeReviewObject 解码已有领域记录，只在服务器内用于白名单字段比较。
func decodeReviewObject(raw []byte) (map[string]any, error) {
	v := map[string]any{}
	if len(raw) == 0 {
		return v, nil
	}
	err := json.Unmarshal(raw, &v)
	return v, err
}

// setReviewPath 只沿注册字段路径设置显示值，不创建任意路径或改动相邻记录。
func setReviewPath(value any, path []string, replacement string) bool {
	if len(path) == 0 {
		return false
	}
	switch v := value.(type) {
	case map[string]any:
		if len(path) == 1 {
			if _, exists := v[path[0]]; !exists {
				return false
			}
			v[path[0]] = replacement
			if path[0] == "image_url" && reviewString(v, "type") == "image" && replacement == rejectedImageURL {
				v["image_width"] = 320
				v["image_height"] = 180
			}
			return true
		}
		return setReviewPath(v[path[0]], path[1:], replacement)
	case []any:
		i, err := strconv.Atoi(path[0])
		if err != nil || i < 0 || i >= len(v) {
			return false
		}
		if len(path) == 1 {
			v[i] = replacement
			return true
		}
		return setReviewPath(v[i], path[1:], replacement)
	}
	return false
}
