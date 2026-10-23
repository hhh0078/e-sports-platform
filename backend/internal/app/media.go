// Package app 校验并托管用户主动公开的头像和主页图片，与订单私密证据隔离。
package app

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"image/png"
	"net/http"
	"regexp"
)

const publicImageMaxBytes = 3 * 1024 * 1024

var publicImagePath = regexp.MustCompile(`^/api/media/([a-f0-9]{32})$`)

// PublicImage 在既有图片存储中记录上传用途和归属，不与订单证据混用。
type PublicImage struct {
	Evidence
	Purpose string `json:"purpose"`
	ClubID  string `json:"club_id,omitempty"`
	Width   int    `json:"width"`
	Height  int    `json:"height"`
}

// validateImageDimensions 按用途校验像素和裁剪比例，避免超大解码或客户端跳过裁剪。
func validateImageDimensions(purpose string, width, height int) error {
	if width < 1 || height < 1 || int64(width)*int64(height) > 8000000 {
		return fail(400, "图片尺寸无效或超过 800 万像素")
	}
	switch purpose {
	case "avatar":
		if width < 128 || width > 2048 || width != height {
			return fail(400, "头像需裁剪为 1:1，边长 128 至 2048 像素")
		}
	case "support_qr", "club_contact_qr":
		if width < 128 || width > 2048 || width != height {
			return fail(400, "客服二维码需裁剪为1:1，边长128至2048像素")
		}
	case "profile_cover", "club_cover":
		difference := width*9 - height*16
		if width < 320 || height < 180 || width > 2560 || height > 1440 || difference < -16 || difference > 16 {
			return fail(400, "封面需裁剪为 16:9，尺寸 320×180 至 2560×1440 像素")
		}
	case "club_detail":
		if width < 320 || width > 2560 || height < 160 || height > 8192 {
			return fail(400, "详情图宽度需为 320 至 2560 像素，高度 160 至 8192 像素")
		}
	default:
		return fail(400, "图片用途不支持")
	}
	return nil
}

// normalizePublicImage 完整解码并重新编码，移除位置等元数据并拒绝只有伪造头部的文件。
func normalizePublicImage(raw []byte, purpose string) ([]byte, string, image.Config, error) {
	cfg, format, err := image.DecodeConfig(bytes.NewReader(raw))
	if err != nil || !contains([]string{"png", "jpeg", "webp"}, format) || len(raw) > publicImageMaxBytes {
		return nil, "", cfg, fail(400, "图片无效或超过 3 MiB")
	}
	if err = validateImageDimensions(purpose, cfg.Width, cfg.Height); err != nil {
		return nil, "", cfg, err
	}
	decoded, _, err := image.Decode(bytes.NewReader(raw))
	if err != nil {
		return nil, "", cfg, fail(400, "图片内容不完整或无法读取")
	}
	// 微信原生图片预览不继承页面白底，透明黑码只在客服二维码用途合成白底，避免码点落到黑底消失。
	if (purpose == "support_qr" || purpose == "club_contact_qr") && !publicImageOpaque(decoded) {
		bounds := decoded.Bounds()
		flattened := image.NewRGBA(bounds)
		draw.Draw(flattened, bounds, &image.Uniform{C: color.White}, image.Point{}, draw.Src)
		draw.Draw(flattened, bounds, decoded, bounds.Min, draw.Over)
		decoded = flattened
		format = "png"
	}
	var output bytes.Buffer
	mime := "image/jpeg"
	if format == "png" {
		mime = "image/png"
		err = png.Encode(&output, decoded)
	} else {
		err = jpeg.Encode(&output, decoded, &jpeg.Options{Quality: 86})
	}
	if err != nil || output.Len() > publicImageMaxBytes {
		return nil, "", cfg, fail(400, "图片编码后超过 3 MiB，请压缩后上传")
	}
	return output.Bytes(), mime, cfg, nil
}

// uploadImage 接收公开展示图片，已有俱乐部仅允许其负责人或平台管理员上传装修素材。
func (a *App) uploadImage(w http.ResponseWriter, r *http.Request) (any, error) {
	u, err := a.actor(r)
	if err != nil {
		return nil, err
	}
	purpose, clubID := r.URL.Query().Get("purpose"), r.URL.Query().Get("club_id")
	if !contains([]string{"avatar", "profile_cover", "club_cover", "club_detail", "support_qr", "club_contact_qr"}, purpose) {
		return nil, fail(400, "图片用途不支持")
	}
	if purpose == "support_qr" && u.Role != "admin" {
		return nil, fail(403, "仅平台管理员可上传平台客服二维码")
	}
	if clubID != "" {
		if !contains([]string{"club_cover", "club_detail", "club_contact_qr"}, purpose) {
			return nil, fail(400, "个人图片不能关联俱乐部")
		}
		if _, err = a.clubManager(r, u, clubID); err != nil {
			return nil, err
		}
	}
	if err = a.throttle(r.Context(), "public-image", u.ID, 20); err != nil {
		return nil, err
	}
	raw, _, err := parseImage(w, r)
	if err != nil {
		return nil, err
	}
	raw, mime, cfg, err := normalizePublicImage(raw, purpose)
	if err != nil {
		return nil, err
	}
	if err = a.moderateContent(r, u, "上传公开图片："+purpose, nil, []moderationImage{{MIME: mime, Data: raw}}); err != nil {
		return nil, err
	}
	sum := sha256.Sum256(raw)
	v := PublicImage{Evidence: Evidence{ID: newID(), OwnerID: u.ID, SHA256: hex.EncodeToString(sum[:]), MIME: mime, Data: raw, CreatedAt: now(), Public: true, Moderation: a.imageModerationStamp()}, Purpose: purpose, ClubID: clubID, Width: cfg.Width, Height: cfg.Height}
	tx, err := a.DB.Begin(r.Context())
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(r.Context())
	if err = put(r.Context(), tx, "evidence", v.ID, v); err != nil {
		return nil, err
	}
	if err = audit(r.Context(), tx, u.ID, "image.upload."+purpose, v.ID, v.SHA256); err != nil {
		return nil, err
	}
	if err = tx.Commit(r.Context()); err != nil {
		return nil, err
	}
	return map[string]any{"url": "/api/media/" + v.ID, "width": v.Width, "height": v.Height, "mime": v.MIME, "size": len(raw), "sha256": v.SHA256}, nil
}

// validateOwnedImage 只接收本用途、本用户或本俱乐部的托管图片，拒绝跨订单与跨俱乐部引用。
func validateOwnedImage(ctx context.Context, q querier, raw, purpose, ownerID, clubID string, actor User) error {
	if raw == "" || raw == rejectedImageURL {
		return nil
	}
	if trustedDefaultAvatar(raw, purpose) {
		return nil
	}
	match := publicImagePath.FindStringSubmatch(raw)
	if match == nil {
		return fail(400, "请使用本平台上传的图片")
	}
	v, err := get[PublicImage](ctx, q, "evidence", match[1], false)
	if err != nil || !v.Public || v.OrderID != "" || v.Purpose != purpose || v.Moderation == "rejected" || len(v.Data) == 0 {
		return fail(400, "图片不存在或用途不匹配")
	}
	if purpose == "support_qr" {
		if actor.Role != "admin" || v.OwnerID != actor.ID || v.ClubID != "" {
			return fail(403, "平台客服二维码必须由当前管理员本人上传")
		}
		return validateImageDimensions(purpose, v.Width, v.Height)
	}
	if purpose == "club_contact_qr" {
		if v.OwnerID != actor.ID || v.ClubID != "" && v.ClubID != clubID {
			return fail(403, "俱乐部客服二维码须由本人上传且属于当前俱乐部")
		}
		return validateImageDimensions(purpose, v.Width, v.Height)
	}
	if clubID == "" {
		if v.ClubID != "" || v.OwnerID != ownerID {
			return fail(403, "不能使用其他用户的图片")
		}
	} else if v.ClubID != clubID && !(v.ClubID == "" && (v.OwnerID == ownerID || actor.Role == "admin" && v.OwnerID == actor.ID)) {
		return fail(403, "图片不属于此俱乐部")
	}
	return validateImageDimensions(purpose, v.Width, v.Height)
}

// publicImageOpaque 检查图片是否完全不透明，标准解码结果优先使用自身判定以避免额外逐像素扫描。
func publicImageOpaque(source image.Image) bool {
	if opaque, ok := source.(interface{ Opaque() bool }); ok {
		return opaque.Opaque()
	}
	bounds := source.Bounds()
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			_, _, _, alpha := source.At(x, y).RGBA()
			if alpha != 65535 {
				return false
			}
		}
	}
	return true
}
