// Package app 验证文章JSON转义预算与原文限制分离，同时保持统一解码安全、其它接口64KiB及Markdown图片上限。
package app

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestDecodeWithLimitSecurity 验证不同固定预算复用未知字段、尾部JSON和实际传输大小限制。
func TestDecodeWithLimitSecurity(t *testing.T) {
	for _, limit := range []int64{64 * 1024, articleJSONTransportMaxBytes} {
		for _, raw := range []string{`{"known":"正文","unknown":true}`, `{"known":"正文"} {"known":"尾部对象"}`} {
			request := httptest.NewRequest("POST", "/", strings.NewReader(raw))
			var input struct {
				Known string `json:"known"`
			}
			if err := decodeWithLimit(request, &input, limit); err == nil {
				t.Fatal("扩大预算绕过了统一JSON安全规则")
			}
		}
		for _, extra := range []int64{0, 1} {
			raw := `{"known":"正文"}`
			payload := raw + strings.Repeat(" ", int(limit)-len(raw)+int(extra))
			request := httptest.NewRequest("POST", "/", strings.NewReader(payload))
			var input struct {
				Known string `json:"known"`
			}
			err := decodeWithLimit(request, &input, limit)
			if (err == nil) != (extra == 0) {
				t.Fatalf("JSON传输边界错误：预算=%d 超出=%d 错误=%v", limit, extra, err)
			}
		}
	}
}

// articlePaddedHTTP 仅发送生成的本地隔离请求，用尾部空白检验传输上限，不把大正文或令牌打印到日志。
func articlePaddedHTTP(t *testing.T, api *testAPI, method, route string, input any, size int, expected int) {
	t.Helper()
	raw, err := json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	if size < len(raw) {
		t.Fatal("传输边界夹具小于合法JSON")
	}
	raw = append(raw, bytes.Repeat([]byte(" "), size-len(raw))...)
	request, err := http.NewRequest(method, api.Server.URL+"/api"+route, bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+api.Tokens["admin"])
	response, err := api.Server.Client().Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	io.Copy(io.Discard, response.Body)
	if response.StatusCode != expected {
		t.Fatalf("传输边界请求状态错误：%s %s 字节=%d 状态=%d", method, route, size, response.StatusCode)
	}
}

// TestIntegrationArticleJSONEscapesBudget 验证三个真实文章入口接受48KiB转义原文，同时拒绝原文及传输层超限。
func TestIntegrationArticleJSONEscapesBudget(t *testing.T) {
	api := newTestAPI(t)
	linePattern := "文\"\\\n"
	markdown := strings.Repeat(linePattern, articleMarkdownMaxBytes/len(linePattern))
	if len(markdown) != articleMarkdownMaxBytes {
		t.Fatal("换行引号原文夹具没有覆盖48KiB精确边界")
	}
	input := map[string]any{"title": "转义正文边界", "summary": "", "body_markdown": markdown, "cover_url": "", "status": "draft"}
	raw, err := json.Marshal(input)
	if err != nil || len(raw) <= 64*1024 || len(raw) > articleJSONTransportMaxBytes {
		t.Fatal("JSON转义夹具未超过旧传输上限或错误超过新预算", len(raw), err)
	}
	var article Article
	api.call("admin", "POST", "/admin/articles", input, 200, &article)
	if article.BodyMarkdown != markdown {
		t.Fatal("创建接口截断或改变了合法48KiB原文")
	}
	// ASCII控制字符每个需要六字节JSON转义，验证最大转义倍率也落在有界预算内。
	controlText := strings.Repeat("\x01", articleMarkdownMaxBytes-3) + "文"
	input["body_markdown"], input["revision"] = controlText, article.Revision
	raw, err = json.Marshal(input)
	if err != nil || len(raw) < 5*articleMarkdownMaxBytes || len(raw) > articleJSONTransportMaxBytes {
		t.Fatal("控制字符夹具未覆盖最坏JSON转义预算", len(raw), err)
	}
	api.call("admin", "PATCH", "/admin/articles/"+article.ID, input, 200, &article)
	if article.BodyMarkdown != controlText {
		t.Fatal("修改接口改变了合法48KiB控制字符原文")
	}
	api.call("admin", "POST", "/admin/articles/preview", map[string]string{"body_markdown": controlText}, 200, nil)
	input["body_markdown"], input["revision"] = controlText+"x", article.Revision
	api.call("admin", "POST", "/admin/articles", input, 400, nil)
	api.call("admin", "PATCH", "/admin/articles/"+article.ID, input, 400, nil)
	api.call("admin", "POST", "/admin/articles/preview", map[string]string{"body_markdown": controlText + "x"}, 400, nil)
	input["body_markdown"] = "合法短正文"
	articlePaddedHTTP(t, api, "POST", "/admin/articles", input, articleJSONTransportMaxBytes+1, 400)
	articlePaddedHTTP(t, api, "PATCH", "/admin/articles/"+article.ID, input, articleJSONTransportMaxBytes+1, 400)
	articlePaddedHTTP(t, api, "POST", "/admin/articles/preview", map[string]string{"body_markdown": "合法短正文"}, articleJSONTransportMaxBytes+1, 400)
	stored, err := get[Article](t.Context(), api.App.DB, "articles", article.ID, false)
	if err != nil || stored.Revision != article.Revision || stored.BodyMarkdown != controlText {
		t.Fatal("被拒请求改变了已保存文章", err)
	}
	articlePaddedHTTP(t, api, "PATCH", "/admin/public-content", map[string]any{"revision": 0, "service_notice": "普通配置保持原预算"}, 64*1024+1, 400)
	var cfg PublicContentSettings
	api.call("admin", "GET", "/admin/public-content", nil, 200, &cfg)
	if cfg.Revision != 0 || cfg.ServiceNotice != "" {
		t.Fatal("扩大文章预算影响了其它接口64KiB边界")
	}
}

// TestArticleMarkdownImageLimit 验证增加JSON传输预算没有改变成熟Markdown解析器的20张图片上限。
func TestArticleMarkdownImageLimit(t *testing.T) {
	image := "![图文说明](/static/covers/official/delta-1.jpg)\n\n"
	for _, count := range []int{20, 21} {
		_, images, err := renderArticleMarkdown(context.Background(), nil, strings.Repeat(image, count), "", User{}, true)
		if count == 20 && (err != nil || len(images) != 20) || count == 21 && err == nil {
			t.Fatalf("Markdown图片数量限制变化：图片=%d 已解析=%d 错误=%v", count, len(images), err)
		}
	}
}
