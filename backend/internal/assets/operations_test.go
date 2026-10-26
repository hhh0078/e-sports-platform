// Package assets 验证固定运营素材的真实像素、用途白名单、透明公告及只读HTTP边界。
package assets

import (
	"bytes"
	"image/png"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestOperationAssetsScope 验证平台首图16:9、新人公告透明和目的精确匹配，未知静态资源不被开放。
func TestOperationAssetsScope(t *testing.T) {
	intro, err := OperationImageData("/static/operations/platform-intro-v1.png", "promotion_banner")
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := png.DecodeConfig(bytes.NewReader(intro))
	if err != nil || cfg.Width != 1672 || cfg.Height != 941 {
		t.Fatal("平台首图真实尺寸异常", err)
	}
	for _, test := range [][2]string{{"/static/operations/newcomer-rules-v1.png", "promotion_banner"}, {"/static/operations/newcomer-rules-v1.png", "article_cover"}, {"/static/operations/platform-intro-v1.png", "avatar"}, {"/static/operations/../avatars/cat_face.png", "article_image"}, {"/static/operations/platform-intro-v1.png?x=1", "promotion_banner"}, {"/static/operations/unknown.png", "announcement_image"}} {
		if _, err = OperationImageData(test[0], test[1]); err == nil {
			t.Fatal("非法素材或用途被接受", test)
		}
	}
	raw, err := OperationImageData("/static/operations/newcomer-rules-v1.png", "announcement_image")
	if err != nil {
		t.Fatal(err)
	}
	image, err := png.Decode(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	_, _, _, alpha := image.At(0, 0).RGBA()
	if alpha == 65535 {
		t.Fatal("公告透明背景未保留")
	}
	for _, id := range []string{"platform-intro-v1", "newcomer-rules-v1"} {
		body, err := DefaultArticleText(id)
		if err != nil || !strings.HasPrefix(body, "# ") {
			t.Fatal("正式文章未打包", id, err)
		}
	}
}

// TestOperationsHandlerScope 验证HTTP仅允许两张PNG的GET与HEAD，目录、Markdown和未知文件保持404。
func TestOperationsHandlerScope(t *testing.T) {
	for _, test := range []struct {
		method, path string
		status       int
	}{
		{"GET", "/static/operations/platform-intro-v1.png", 200}, {"HEAD", "/static/operations/newcomer-rules-v1.png", 200},
		{"GET", "/static/operations/", 404}, {"GET", "/static/operations/platform-intro-v1.md", 404},
		{"GET", "/static/operations/missing.png", 404}, {"POST", "/static/operations/platform-intro-v1.png", 405},
	} {
		response := httptest.NewRecorder()
		OperationsHandler().ServeHTTP(response, httptest.NewRequest(test.method, test.path, nil))
		if response.Code != test.status {
			t.Fatalf("%s %s: %d", test.method, test.path, response.Code)
		}
		if test.method == "HEAD" && response.Body.Len() != 0 {
			t.Fatal("HEAD携带图片正文")
		}
	}
}
