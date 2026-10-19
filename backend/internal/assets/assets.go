// Package assets 将游戏官方封面嵌入后端，减少小程序包体积并由同一 HTTPS 域名提供图片。
package assets

import (
	"embed"
	"io/fs"
	"net/http"
	"path"
	"strings"
)

//go:embed covers
var files embed.FS

// Handler 仅提供已嵌入的图片文件，不列目录、不访问服务器其他文件。
func Handler() http.Handler {
	subtree, _ := fs.Sub(files, "covers")
	server := http.StripPrefix("/static/covers/", http.FileServer(http.FS(subtree)))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ext := strings.ToLower(path.Ext(r.URL.Path))
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if ext != ".jpg" && ext != ".jpeg" && ext != ".png" && ext != ".webp" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Cache-Control", "public, max-age=86400")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		server.ServeHTTP(w, r)
	})
}
