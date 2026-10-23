// Package assets 为分享素材提供只读内置封面查找，禁止目录穿越和读取非图片资源。
package assets

import (
	"net/http"
	"path"
	"strings"
)

// PublicCoverData 读取精确匹配的内置游戏封面，返回图片字节或不存在错误。
func PublicCoverData(raw string) ([]byte, error) {
	if !strings.HasPrefix(raw, "/static/covers/") || strings.ContainsAny(raw, "?#\\") || path.Clean(raw) != raw {
		return nil, http.ErrMissingFile
	}
	ext := strings.ToLower(path.Ext(raw))
	if ext != ".png" && ext != ".jpg" && ext != ".jpeg" && ext != ".webp" {
		return nil, http.ErrMissingFile
	}
	return files.ReadFile("covers/" + strings.TrimPrefix(raw, "/static/covers/"))
}
