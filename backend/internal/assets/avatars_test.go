// Package assets 核验默认头像的完整来源、MIT许可、校验值与静态接口白名单。
package assets

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"image/png"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestDefaultAvatarManifest 验证三十张真实PNG均与固定来源清单一致，且许可随资源内嵌交付。
func TestDefaultAvatarManifest(t *testing.T) {
	var manifest struct {
		Revision string `json:"revision"`
		Images   []struct {
			Path   string `json:"public_path"`
			SHA256 string `json:"sha256"`
			Bytes  int    `json:"bytes"`
			Source string `json:"source_url"`
		} `json:"images"`
	}
	raw, err := files.ReadFile("avatars/manifest.json")
	if err != nil || json.Unmarshal(raw, &manifest) != nil || len(manifest.Images) != 30 || len(DefaultAvatarPaths()) != 30 {
		t.Fatalf("头像来源清单缺失或数量错误：%v", err)
	}
	seen := map[string]bool{}
	for _, item := range manifest.Images {
		data, err := DefaultAvatarData(item.Path)
		if err != nil || seen[item.Path] || !strings.Contains(item.Source, "/"+manifest.Revision+"/assets/") {
			t.Fatalf("头像缺失、重复或未固定来源：%s %v", item.Path, err)
		}
		seen[item.Path] = true
		sum := sha256.Sum256(data)
		decoded, err := png.Decode(bytes.NewReader(data))
		if err != nil || decoded.Bounds().Dx() != 256 || decoded.Bounds().Dy() != 256 || len(data) != item.Bytes || hex.EncodeToString(sum[:]) != item.SHA256 {
			t.Fatalf("头像尺寸、内容或校验值不符：%s %v", item.Path, err)
		}
	}
	license, err := files.ReadFile("avatars/LICENSE")
	if err != nil || !bytes.Contains(license, []byte("MIT License")) || !bytes.Contains(license, []byte("Microsoft Corporation.")) {
		t.Fatal("必须随源码保留Microsoft版权与MIT许可")
	}
}

// TestDefaultAvatarWhitelist 验证静态读取、目录隔离、查询路径拒绝和缓存验证不泄露其他文件。
func TestDefaultAvatarWhitelist(t *testing.T) {
	for _, raw := range []string{"", "/static/avatars/", "/static/avatars/LICENSE", "/static/avatars/manifest.json", "/static/avatars/../covers/official/delta-icon.jpg", "/static/avatars/cat_face.png?x=1", "https://example.com/static/avatars/cat_face.png", "/static/avatars/unknown.png"} {
		if IsDefaultAvatar(raw) {
			t.Fatalf("不应允许路径：%s", raw)
		}
	}
	for _, method := range []string{"GET", "HEAD", "POST"} {
		response := httptest.NewRecorder()
		AvatarHandler().ServeHTTP(response, httptest.NewRequest(method, "/static/avatars/cat_face.png", nil))
		want := 200
		if method == "POST" {
			want = 405
		}
		if response.Code != want {
			t.Fatalf("%s状态错误：%d", method, response.Code)
		}
		if method != "POST" && (response.Header().Get("Content-Type") != "image/png" || response.Header().Get("ETag") == "") {
			t.Fatal("图片类型或缓存校验缺失")
		}
		if method == "HEAD" && response.Body.Len() != 0 {
			t.Fatal("HEAD不应返回图片正文")
		}
	}
	for _, path := range []string{"/static/avatars/", "/static/avatars/LICENSE", "/static/avatars/unknown.png", "/static/avatars/../covers/official/delta-icon.jpg"} {
		response := httptest.NewRecorder()
		AvatarHandler().ServeHTTP(response, httptest.NewRequest("GET", path, nil))
		if response.Code != 404 {
			t.Fatalf("非清单资源被公开：%s", path)
		}
	}
	response := httptest.NewRecorder()
	AvatarHandler().ServeHTTP(response, httptest.NewRequest("GET", "/static/avatars/cat_face.png", nil))
	request := httptest.NewRequest("GET", "/static/avatars/cat_face.png", nil)
	request.Header.Set("If-None-Match", response.Header().Get("ETag"))
	cached := httptest.NewRecorder()
	AvatarHandler().ServeHTTP(cached, request)
	if cached.Code != 304 || cached.Body.Len() != 0 {
		t.Fatal("缓存命中仍重复发送头像")
	}
}
