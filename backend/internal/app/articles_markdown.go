// Package app 使用Goldmark解析受限Markdown，禁原始HTML、危险URL和外部图片，公开渲染不保留过期HTML副本。
package app

import (
	"bytes"
	"context"
	"net/url"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/text"
)

const articleMarkdownMaxBytes = 48 * 1024

// articleJSONTransportMaxBytes 容纳原文每字节最坏六字符JSON转义，以及有界标题、摘要和版本元数据。
const articleJSONTransportMaxBytes = 6*articleMarkdownMaxBytes + 16*1024

// safeArticleLink 仅允许站内相对路径、锚点或无凭据HTTPS链接，不允许脚本和协议相对URL。
func safeArticleLink(raw string) bool {
	if raw == "" || raw != strings.TrimSpace(raw) || strings.ContainsAny(raw, "\x00\r\n\t\\") {
		return false
	}
	u, err := url.Parse(raw)
	if err != nil || u.User != nil {
		return false
	}
	if strings.HasPrefix(u.Path, "/api/") {
		return false
	}
	if strings.HasPrefix(raw, "#") {
		return u.Scheme == "" && u.Host == ""
	}
	if strings.HasPrefix(raw, "/") && !strings.HasPrefix(raw, "//") {
		if u.Scheme != "" || u.Host != "" || !contains([]string{"/pages/services/detail", "/pages/requests/detail", "/pages/content/articles/detail"}, u.Path) {
			return false
		}
		query := u.Query()
		return len(query) == 1 && len(query["id"]) == 1 && validContentTarget(&ContentTarget{Kind: "article", ID: query.Get("id")})
	}
	return u.Scheme == "https" && u.Hostname() != "" && u.Opaque == ""
}

// renderArticleMarkdown 在同一AST中校验后渲染，写入时核对图片归属，公开读取时把撤销图片替换为占位。
func renderArticleMarkdown(ctx context.Context, q querier, markdown, previous string, actor User, writing bool) (string, []string, error) {
	if !length(markdown, 1, articleMarkdownMaxBytes) || len(markdown) > articleMarkdownMaxBytes {
		return "", nil, fail(400, "Markdown正文不能为空且不能超过48KiB")
	}
	engine := goldmark.New(goldmark.WithRendererOptions(html.WithHardWraps()))
	source := []byte(markdown)
	node := engine.Parser().Parse(text.NewReader(source))
	previousImages := map[string]bool{}
	if writing && previous != "" {
		old := engine.Parser().Parse(text.NewReader([]byte(previous)))
		_ = ast.Walk(old, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
			if image, ok := n.(*ast.Image); ok && entering {
				previousImages[string(image.Destination)] = true
			}
			return ast.WalkContinue, nil
		})
	}
	images := []string{}
	err := ast.Walk(node, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		switch value := n.(type) {
		case *ast.RawHTML, *ast.HTMLBlock:
			return ast.WalkStop, fail(400, "文章不支持原始HTML，请使用Markdown语法")
		case *ast.Link:
			if !safeArticleLink(string(value.Destination)) {
				return ast.WalkStop, fail(400, "文章链接仅支持站内路径或HTTPS地址")
			}
		case *ast.AutoLink:
			if !safeArticleLink(string(value.URL(source))) {
				return ast.WalkStop, fail(400, "文章自动链接地址不安全")
			}
		case *ast.Image:
			raw := string(value.Destination)
			images = append(images, raw)
			if len(images) > 20 {
				return ast.WalkStop, fail(400, "文章正文最多20张图片")
			}
			old := raw
			if writing && !previousImages[raw] {
				old = ""
			}
			if err := validateContentImage(ctx, q, raw, old, "article_image", actor); err != nil {
				if !writing && publicImagePath.MatchString(raw) {
					media, readErr := get[Evidence](ctx, q, "evidence", strings.TrimPrefix(raw, "/api/media/"), false)
					if missing(readErr) {
						break // 普通资源丢失保留原地址的404，不误标为审核拒绝。
					}
					if readErr != nil {
						return ast.WalkStop, readErr
					}
					if media.Moderation == "rejected" {
						value.Destination = []byte(rejectedImageURL)
						break
					}
				}
				return ast.WalkStop, err
			}
		}
		return ast.WalkContinue, nil
	})
	if err != nil {
		return "", nil, err
	}
	var out bytes.Buffer
	if err = engine.Renderer().Render(&out, source, node); err != nil {
		return "", nil, err
	}
	return out.String(), images, nil
}
