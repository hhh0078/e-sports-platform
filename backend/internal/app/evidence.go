// Package app 将订单图片证据保存在数据库，并通过短期签名链接授权显示。
package app

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	_ "golang.org/x/image/webp"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type Evidence struct {
	ID        string `json:"id"`
	OrderID   string `json:"order_id"`
	OwnerID   string `json:"owner_id"`
	SHA256    string `json:"sha256"`
	MIME      string `json:"mime"`
	Data      []byte `json:"data"`
	CreatedAt string `json:"created_at"`
	Public    bool   `json:"public"`
}

// uploadEvidence 校验四方权限、大小、图片魔数与可解析尺寸后持久化原始证据。
func (a *App) uploadEvidence(w http.ResponseWriter, r *http.Request) (any, error) {
	u, err := a.actor(r)
	if err != nil {
		return nil, err
	}
	o, err := a.authorizedOrder(r, u, r.PathValue("id"), false)
	if err != nil {
		return nil, err
	}
	if err = requireLiveOrder(o); err != nil {
		return nil, err
	}
	if err = a.throttle(r.Context(), "evidence", u.ID, 20); err != nil {
		return nil, err
	}
	raw, mime, err := parseImage(w, r)
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256(raw)
	v := Evidence{ID: newID(), OrderID: o.ID, OwnerID: u.ID, SHA256: hex.EncodeToString(sum[:]), MIME: mime, Data: raw, CreatedAt: now()}
	if err = put(r.Context(), a.DB, "evidence", v.ID, v); err != nil {
		return nil, err
	}
	return map[string]string{"url": a.signEvidenceURL("/api/evidence/" + v.ID), "sha256": v.SHA256}, nil
}

// evidenceSignature 根据图片标识和到期时间计算不可伪造的访问签名。
func (a *App) evidenceSignature(id, expires string) string {
	h := hmac.New(sha256.New, a.EvidenceKey)
	h.Write([]byte(id + "\n" + expires))
	return hex.EncodeToString(h.Sum(nil))
}

// signEvidenceURL 为持久化路径生成十五分钟有效的查看链接。
func (a *App) signEvidenceURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	id := strings.TrimPrefix(u.Path, "/api/evidence/")
	expires := strconv.FormatInt(time.Now().Add(15*time.Minute).Unix(), 10)
	return "/api/evidence/" + id + "?expires=" + expires + "&signature=" + a.evidenceSignature(id, expires)
}

// validateEvidence 保证聊天引用的图片属于当前订单，拒绝任意外部链接。
func (a *App) validateEvidence(r *http.Request, orderID, raw string) (string, error) {
	u, err := url.Parse(raw)
	if err != nil || u.IsAbs() || !strings.HasPrefix(u.Path, "/api/evidence/") {
		return "", fail(400, "图片证据链接无效")
	}
	id := strings.TrimPrefix(u.Path, "/api/evidence/")
	v, err := get[Evidence](r.Context(), a.DB, "evidence", id, false)
	if err != nil || v.OrderID != orderID {
		return "", fail(403, "图片不属于此订单")
	}
	return "/api/evidence/" + id, nil
}

// viewEvidence 允许订单参与者或持有短期签名链接的客户端查看图片。
func (a *App) viewEvidence(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	expires := r.URL.Query().Get("expires")
	sig := r.URL.Query().Get("signature")
	expiry, err := strconv.ParseInt(expires, 10, 64)
	signed := err == nil && expiry >= time.Now().Unix() && expiry <= time.Now().Add(16*time.Minute).Unix() && hmac.Equal([]byte(sig), []byte(a.evidenceSignature(id, expires)))
	v, e := get[Evidence](r.Context(), a.DB, "evidence", id, false)
	if e != nil {
		respond(w, 404, map[string]string{"error": "图片不存在"})
		return
	}
	if !signed {
		u, e := a.actor(r)
		if e != nil {
			respond(w, 401, map[string]string{"error": "图片访问凭证已过期"})
			return
		}
		if _, e = a.authorizedOrder(r, u, v.OrderID, false); e != nil {
			respond(w, 403, map[string]string{"error": "无权访问图片"})
			return
		}
	}
	w.Header().Set("Content-Type", v.MIME)
	w.Header().Set("Content-Length", strconv.Itoa(len(v.Data)))
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "private, max-age=60")
	w.Header().Set("Content-Disposition", fmt.Sprintf("inline; filename=%q", id))
	w.Write(v.Data)
}

// parseImage 统一校验上传文件格式和尺寸，防止伪造图片与超量上传。
func parseImage(w http.ResponseWriter, r *http.Request) ([]byte, string, error) {
	r.Body = http.MaxBytesReader(w, r.Body, 5*1024*1024+65536)
	if err := r.ParseMultipartForm(5*1024*1024 + 65536); err != nil {
		return nil, "", fail(400, "图片上传失败，文件不能超过 5 MiB")
	}
	if r.MultipartForm != nil {
		defer r.MultipartForm.RemoveAll()
	}
	f, _, err := r.FormFile("file")
	if err != nil {
		return nil, "", fail(400, "请选择图片文件")
	}
	defer f.Close()
	raw, err := io.ReadAll(io.LimitReader(f, 5*1024*1024+1))
	if err != nil || len(raw) == 0 || len(raw) > 5*1024*1024 {
		return nil, "", fail(400, "图片不能为空或超过 5 MiB")
	}
	mime := http.DetectContentType(raw)
	if !contains([]string{"image/png", "image/jpeg", "image/webp"}, mime) {
		return nil, "", fail(400, "只支持 PNG、JPEG 或 WebP 图片")
	}
	{
		cfg, _, e := image.DecodeConfig(bytes.NewReader(raw))
		if e != nil || cfg.Width < 1 || cfg.Height < 1 || cfg.Width > 12000 || cfg.Height > 12000 || int64(cfg.Width)*int64(cfg.Height) > 40000000 {
			return nil, "", fail(400, "图片无效或尺寸过大")
		}
	}

	return raw, mime, nil
}

// evidenceLink 对订单参与者重新签发短期图片链接，禁止跨订单刷新证据。
func (a *App) evidenceLink(w http.ResponseWriter, r *http.Request) (any, error) {
	u, err := a.actor(r)
	if err != nil {
		return nil, err
	}
	o, err := a.authorizedOrder(r, u, r.PathValue("id"), false)
	if err != nil {
		return nil, err
	}
	id := r.URL.Query().Get("evidence_id")
	v, err := get[Evidence](r.Context(), a.DB, "evidence", id, false)
	if err != nil || v.OrderID != o.ID || v.Public {
		return nil, fail(403, "图片不属于此订单")
	}
	return map[string]string{"url": a.signEvidenceURL("/api/evidence/" + id)}, nil
}
