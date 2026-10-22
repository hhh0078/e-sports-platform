// Package assets 提供带明确来源许可的三十个非人动物头像，仅开放固定目录清单中的PNG。
package assets

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strings"
	"time"
)

var avatarNames = [...]string{
	"cat_face", "dog_face", "fox", "wolf", "lion", "tiger_face", "panda", "koala", "bear", "polar_bear",
	"rabbit_face", "hamster", "mouse_face", "raccoon", "unicorn", "horse_face", "cow_face", "pig_face", "frog", "monkey_face",
	"owl", "penguin", "baby_chick", "hatching_chick", "bird", "duck", "whale", "dolphin", "octopus", "turtle",
}

// DefaultAvatarPaths 返回固定头像清单的独立副本，调用者不能修改服务端允许的资源集合。
func DefaultAvatarPaths() []string {
	paths := make([]string, len(avatarNames))
	for i, name := range avatarNames {
		paths[i] = "/static/avatars/" + name + ".png"
	}
	return paths
}

// IsDefaultAvatar 精确匹配内置头像路径，带查询、目录穿越、外部地址或未知名称均不通过。
func IsDefaultAvatar(raw string) bool {
	for _, name := range avatarNames {
		if raw == "/static/avatars/"+name+".png" {
			return true
		}
	}
	return false
}

// DefaultAvatarData 读取清单内已嵌入的PNG，拒绝通过文件路径访问许可或其他服务器文件。
func DefaultAvatarData(raw string) ([]byte, error) {
	if !IsDefaultAvatar(raw) {
		return nil, http.ErrMissingFile
	}
	return files.ReadFile("avatars/" + strings.TrimPrefix(raw, "/static/avatars/"))
}

// AvatarHandler 只公开固定PNG清单，并提供明确类型、缓存校验与HEAD支持；不公开许可和目录。
func AvatarHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		raw, err := DefaultAvatarData(r.URL.Path)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		sum := sha256.Sum256(raw)
		w.Header().Set("Content-Type", "image/png")
		w.Header().Set("Cache-Control", "public, max-age=86400")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("ETag", `"`+hex.EncodeToString(sum[:])+`"`)
		http.ServeContent(w, r, "avatar.png", time.Time{}, bytes.NewReader(raw))
	})
}
