// Package app 生成并缓存真实微信小程序码，严格验证上游图片且不将错误JSON伪装成二维码。
package app

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"image"
	"image/png"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

const shareCodePage = "pages/shares/detail"
const shareCodeMaxBytes = 2 * 1024 * 1024

// shareCodeError 返回可公开显示的稳定错误码，禁止包含上游令牌、密钥或原始响应。
func shareCodeError(status int, message string) error {
	return &apiError{Status: status, Code: "share_code_unavailable", Message: message}
}

// shareCodeCacheKey 将缓存绑定到当前AppID、发布版本、页面及永久分享号。
func (a *App) shareCodeCacheKey(id string) string {
	digest := sha256.Sum256([]byte(a.Config.WechatAppID + ":" + shareEnvironment + ":" + shareCodePage + ":" + id))
	return a.redisKey("share-code:" + hex.EncodeToString(digest[:]))
}

// normalizeShareCode 完整解码官方PNG/JPEG并转成PNG，拒绝JSON、假头部和超大图片。
func normalizeShareCode(raw []byte) ([]byte, error) {
	if len(raw) == 0 || len(raw) > shareCodeMaxBytes {
		return nil, shareCodeError(502, "微信小程序码图片大小异常，请稍后重试")
	}
	cfg, format, err := image.DecodeConfig(bytes.NewReader(raw))
	if err != nil || (format != "png" && format != "jpeg") || cfg.Width != cfg.Height || cfg.Width < 280 || cfg.Width > 1280 {
		return nil, shareCodeError(502, "微信小程序码图片无效，请稍后重试")
	}
	decoded, _, err := image.Decode(bytes.NewReader(raw))
	if err != nil {
		return nil, shareCodeError(502, "微信小程序码图片不完整，请稍后重试")
	}
	var output bytes.Buffer
	if err = png.Encode(&output, decoded); err != nil || output.Len() > shareCodeMaxBytes {
		return nil, shareCodeError(502, "微信小程序码图片编码失败，请稍后重试")
	}
	return output.Bytes(), nil
}

// fetchShareCode 调用固定微信官方地址，最多刷新一次失效凭证，拒绝任何重定向。
func (a *App) fetchShareCode(ctx context.Context, id string) ([]byte, error) {
	if !shareIDPattern.MatchString(id) {
		return nil, fail(404, "分享不存在或已失效")
	}
	if err := a.wechatConfigured(); err != nil {
		return nil, shareCodeError(503, "小程序码服务尚未配置微信凭证，请联系管理员")
	}
	payload, err := json.Marshal(map[string]any{"scene": id, "page": shareCodePage, "check_path": true, "env_version": shareEnvironment, "width": 430, "auto_color": false, "line_color": map[string]int{"r": 0, "g": 0, "b": 0}, "is_hyaline": false})
	if err != nil {
		return nil, err
	}
	if a.HTTPClient == nil {
		return nil, shareCodeError(503, "小程序码服务暂不可用，请稍后重试")
	}
	client := *a.HTTPClient
	// 禁止官方地址返回的跳转携带查询凭证前往其他主机。
	client.CheckRedirect = func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }
	for attempt := 0; attempt < 2; attempt++ {
		token, tokenErr := a.wechatAccessToken(ctx, attempt > 0)
		if tokenErr != nil {
			return nil, shareCodeError(503, "无法取得微信小程序码凭证，请检查服务端配置后重试")
		}
		request, requestErr := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.weixin.qq.com/wxa/getwxacodeunlimit?"+url.Values{"access_token": {token}}.Encode(), bytes.NewReader(payload))
		if requestErr != nil {
			return nil, shareCodeError(502, "微信小程序码请求无效")
		}
		request.Header.Set("Content-Type", "application/json")
		response, requestErr := client.Do(request)
		if requestErr != nil {
			return nil, shareCodeError(502, "微信小程序码服务暂时不可用，请稍后重试")
		}
		raw, readErr := io.ReadAll(io.LimitReader(response.Body, shareCodeMaxBytes+1))
		response.Body.Close()
		if readErr != nil || len(raw) > shareCodeMaxBytes || response.StatusCode != http.StatusOK {
			return nil, shareCodeError(502, "微信小程序码服务响应异常，请稍后重试")
		}
		var upstream struct {
			ErrCode int `json:"errcode"`
		}
		if json.Unmarshal(raw, &upstream) == nil {
			if attempt == 0 && (upstream.ErrCode == 40001 || upstream.ErrCode == 40014 || upstream.ErrCode == 42001) {
				continue
			}
			if upstream.ErrCode == 41030 {
				return nil, shareCodeError(503, "分享页面尚未发布，请发布当前小程序版本后再生成正式码")
			}
			if upstream.ErrCode == 45009 || upstream.ErrCode == 45011 {
				return nil, shareCodeError(429, "微信小程序码生成达到频率限制，请稍后再试")
			}
			return nil, shareCodeError(502, "微信未能生成小程序码，请检查小程序发布状态和服务端授权")
		}
		return normalizeShareCode(raw)
	}
	return nil, shareCodeError(502, "微信小程序码凭证失效，请稍后重试")
}

// shareCode 在实时公开权限校验后读取短期缓存，单码生成互斥且不会使永久映射过期。
func (a *App) shareCode(r *http.Request) ([]byte, error) {
	ctx := r.Context()
	record, err := a.shareRecord(ctx, r.PathValue("id"))
	if err != nil {
		return nil, err
	}
	if _, err = a.shareMetadata(ctx, record); err != nil {
		return nil, err
	}
	key := a.shareCodeCacheKey(record.ID)
	cached, err := a.Redis.Get(ctx, key).Bytes()
	if err == nil {
		if normalized, decodeErr := normalizeShareCode(cached); decodeErr == nil {
			return normalized, nil
		}
	} else if err != redis.Nil {
		return nil, shareCodeError(503, "小程序码缓存服务暂不可用，请稍后重试")
	}
	ip, _, _ := net.SplitHostPort(r.RemoteAddr)
	if err = a.throttle(ctx, "share-code-ip", ip, 10); err != nil {
		return nil, err
	}
	if err = a.throttle(ctx, "share-code-owner", record.SharerID, 20); err != nil {
		return nil, err
	}
	lease, lockErr := a.Redis.SetNX(ctx, key+":generating", "1", 30*time.Second).Result()
	if lockErr != nil {
		return nil, shareCodeError(503, "小程序码生成服务暂不可用，请稍后重试")
	}
	if !lease {
		return nil, shareCodeError(409, "小程序码正在生成，请稍后重试")
	}
	defer a.Redis.Del(context.WithoutCancel(ctx), key+":generating")
	requestCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	raw, err := a.fetchShareCode(requestCtx, record.ID)
	if err != nil {
		return nil, err
	}
	if err = a.Redis.Set(ctx, key, raw, 7*24*time.Hour).Err(); err != nil {
		return nil, shareCodeError(503, "小程序码保存失败，请稍后重试")
	}
	return raw, nil
}

// shareCodeHTTP 直接输出图片或脱敏JSON错误，禁止给二进制尾部追加业务响应封装。
func (a *App) shareCodeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "no-store")
	raw, err := a.shareCode(r)
	if err != nil {
		var problem *apiError
		if !errors.As(err, &problem) {
			problem = &apiError{Status: 500, Message: "小程序码服务暂不可用，请稍后重试", Code: "share_code_unavailable"}
		}
		respond(w, problem.Status, map[string]string{"error": problem.Message, "code": problem.Code})
		return
	}
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Content-Length", strconv.Itoa(len(raw)))
	w.WriteHeader(http.StatusOK)
	w.Write(raw)
}
