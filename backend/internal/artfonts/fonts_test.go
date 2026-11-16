// Package artfonts 验证真实六款字体的字形裁剪、授权保留、HTTP缓存及输入门禁。
package artfonts

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"testing"
)

// TestSubsetFonts 真实裁剪每款字体，并用FontTools检查字集、授权和文件体积。
func TestSubsetFonts(t *testing.T) {
	mux := http.NewServeMux()
	mux.Handle("GET /api/fonts/{version}/{font}", Handler())
	server := httptest.NewServer(mux)
	defer server.Close()
	python := os.Getenv("FONT_SUBSET_PYTHON")
	if python == "" {
		python = "python3"
	}
	for _, font := range catalog.Fonts {
		t.Run(font.ID, func(t *testing.T) {
			link := server.URL + "/api/fonts/" + catalog.Version + "/" + font.ID + ".woff?text=" + url.QueryEscape("限时优惠限时")
			response, err := http.Get(link)
			if err != nil {
				t.Fatal(err)
			}
			defer response.Body.Close()
			raw, _ := io.ReadAll(response.Body)
			if response.StatusCode != 200 {
				t.Fatalf("字体不可用: %d %s", response.StatusCode, raw)
			}
			original, _ := sources.ReadFile(font.File)
			if !bytes.HasPrefix(raw, []byte("wOFF")) || len(raw)*20 >= len(original) {
				t.Fatalf("没有按字裁剪: %d/%d", len(raw), len(original))
			}
			check := exec.Command(python, "-c", `import sys,io,json
from fontTools.ttLib import TTFont
f=TTFont(io.BytesIO(sys.stdin.buffer.read()))
assert set(f.getBestCmap()) == set(map(ord,'限时优惠'))
assert 'SIL OPEN FONT LICENSE' in f['name'].getDebugName(13)
assert f['name'].getDebugName(0)
print(json.dumps({'glyphs':len(f.getBestCmap()),'bytes':len(f.reader.file.getvalue())}))`)
			check.Stdin = bytes.NewReader(raw)
			info, err := check.CombinedOutput()
			if err != nil {
				t.Fatalf("字形或授权检查失败 %v %s", err, info)
			}
			var summary map[string]int
			if json.Unmarshal(info, &summary) != nil {
				t.Fatal(string(info))
			}
			t.Logf("原字体%d字节，四字子集%d字节，码点%d", len(original), len(raw), summary["glyphs"])
			if response.Header.Get("Content-Type") != "font/woff" || response.Header.Get("Access-Control-Allow-Origin") != "*" || response.Header.Get("ETag") == "" {
				t.Fatal("字体响应头无效")
			}
			request, _ := http.NewRequest("GET", link, nil)
			request.Header.Set("If-None-Match", response.Header.Get("ETag"))
			cached, err := http.DefaultClient.Do(request)
			if err != nil {
				t.Fatal(err)
			}
			cached.Body.Close()
			if cached.StatusCode != 304 {
				t.Fatal("浏览器缓存没有生效")
			}
		})
	}
}

// TestFontInputBounds 拒绝任意文件、整包下载、控制字符、空文案和超长字集。
func TestFontInputBounds(t *testing.T) {
	mux := http.NewServeMux()
	mux.Handle("GET /api/fonts/{version}/{font}", Handler())
	for _, item := range []struct {
		path   string
		status int
	}{
		{"/api/fonts/1/zcoolkuaile.woff", 400},
		{"/api/fonts/1/unknown.woff?text=a", 404},
		{"/api/fonts/old/zcoolkuaile.woff?text=a", 404},
		{"/api/fonts/1/zcoolkuaile.ttf?text=a", 404},
		{"/api/fonts/1/zcoolkuaile.woff?text=%0a", 400},
		{"/api/fonts/1/zcoolkuaile.woff?text=" + string(bytes.Repeat([]byte("a"), 145)), 400},
	} {
		r := httptest.NewRequest("GET", item.path, nil)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, r)
		if w.Code != item.status {
			t.Fatalf("%s: %d", item.path, w.Code)
		}
	}
	if Valid("url(https://example.com/font.ttf)") || !Valid("") {
		t.Fatal("字体保存白名单无效")
	}
}
