// Package app 返回审核失败的真实图片，普通资源缺失与权限错误不伪装成审核失败。
package app

import (
	"jijie/internal/assets"
	"net/http"
	"strconv"
)

// rejectedMedia 为已拒绝媒体与明确占位地址返回稳定PNG，禁止缓存继续展示旧违规图。
func (a *App) rejectedMedia(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Content-Length", strconv.Itoa(len(assets.ModerationRejectedPNG)))
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Write(assets.ModerationRejectedPNG)
}
