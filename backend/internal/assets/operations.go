// Package assets 仅公开两张受信平台运营素材，并按实际用途校验固定路径，不开放目录或任意静态文件。
package assets

import (
	"bytes"
	"net/http"
	"strings"
	"time"
)

// OperationImageData 精确匹配平台首图和新人公告及其允许用途，目录穿越、外部地址和伪装用途均拒绝。
func OperationImageData(raw, purpose string) ([]byte, error) {
	allowed := false
	switch raw {
	case "/static/operations/platform-intro-v1.png":
		allowed = purpose == "promotion_banner" || purpose == "article_cover" || purpose == "article_image"
	case "/static/operations/newcomer-rules-v1.png":
		allowed = purpose == "announcement_image" || purpose == "article_image"
	}
	if !allowed {
		return nil, http.ErrMissingFile
	}
	return files.ReadFile("operations/" + strings.TrimPrefix(raw, "/static/operations/"))
}

// DefaultArticleText 只读取两个稳定默认文章编号的内嵌正文，不接受任意文件名。
func DefaultArticleText(id string) (string, error) {
	if id != "platform-intro-v1" && id != "newcomer-rules-v1" {
		return "", http.ErrMissingFile
	}
	raw, err := files.ReadFile("operations/" + id + ".md")
	return string(raw), err
}

// OperationsHandler 仅提供固定两张PNG，支持HEAD，不能通过URL读取Markdown原稿、目录和服务器文件。
func OperationsHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		purpose := "promotion_banner"
		if r.URL.Path == "/static/operations/newcomer-rules-v1.png" {
			purpose = "announcement_image"
		}
		raw, err := OperationImageData(r.URL.Path, purpose)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "image/png")
		w.Header().Set("Cache-Control", "public, max-age=86400")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		http.ServeContent(w, r, "operation.png", time.Time{}, bytes.NewReader(raw))
	})
}
