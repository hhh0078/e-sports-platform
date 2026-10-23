// Package app 使用隔离PG/Redis和固定官方地址的HTTP替身验证小程序码协议、缓存和并发。
package app

import (
	"bytes"
	"encoding/json"
	"image"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// shareMockToken 输出测试凭证，不访问真实微信或生成可扫码的伪造小程序码。
func shareMockToken(w http.ResponseWriter) {
	json.NewEncoder(w).Encode(map[string]any{"access_token": "secret-token", "expires_in": 7200})
}

// TestIntegrationShareCodeProtocolAndPermanentMapping 验证官方载荷、JPEG规范化、缓存和永久PG映射。
func TestIntegrationShareCodeProtocolAndPermanentMapping(t *testing.T) {
	api := newTestAPI(t)
	share := createShareFixture(api, "service", "delta-service-1")
	jpeg := shareImageFixture(t, 430, 430, "jpeg")
	var calls atomic.Int32
	mockWechat(t, api, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/cgi-bin/stable_token" {
			shareMockToken(w)
			return
		}
		calls.Add(1)
		var payload struct {
			Scene      string `json:"scene"`
			Page       string `json:"page"`
			EnvVersion string `json:"env_version"`
			CheckPath  bool   `json:"check_path"`
			Width      int    `json:"width"`
		}
		if r.URL.Path != "/wxa/getwxacodeunlimit" || r.Method != "POST" || r.URL.Query().Get("access_token") != "secret-token" || json.NewDecoder(r.Body).Decode(&payload) != nil || payload.Scene != share.ID || payload.Page != "pages/shares/detail" || payload.EnvVersion != "release" || !payload.CheckPath || payload.Width != 430 {
			t.Error("小程序码请求必须遵守固定官方合同且scene使用永久分享号")
		}
		w.Header().Set("Content-Type", "image/jpeg")
		w.Write(jpeg)
	})
	for range 2 {
		status, header, raw := shareCodeResponse(api, share.ID)
		config, format, err := image.DecodeConfig(bytes.NewReader(raw))
		if status != 200 || header.Get("Content-Type") != "image/png" || header.Get("Cache-Control") != "no-store" || err != nil || format != "png" || config.Width != 430 {
			t.Fatalf("图片入口必须真实输出PNG而非data封装: %d %v %s %v", status, header, format, err)
		}
	}
	if calls.Load() != 1 {
		t.Fatal("同一AppID和永久映射应使用码缓存")
	}
	if err := api.App.Redis.Del(t.Context(), api.App.shareCodeCacheKey(share.ID)).Err(); err != nil {
		t.Fatal(err)
	}
	api.call("", "GET", "/shares/"+share.ID, nil, 200, nil)
	status, _, _ := shareCodeResponse(api, share.ID)
	if status != 200 || calls.Load() != 2 {
		t.Fatal("清除短期缓存不应使已保存海报的永久映射失效")
	}
	api.call("worker", "POST", "/services/delta-service-1/pause", map[string]any{}, 200, nil)
	status, header, raw := shareCodeResponse(api, share.ID)
	assertShareError(t, status, header, raw, 404)
	if calls.Load() != 2 {
		t.Fatal("目标失效时不能继续输出缓存或请求官方生成")
	}
}

// TestIntegrationShareCodeUnavailable 验证缺失配置不能生成本地假码，微信错误和伪图片不会成功缓存。
func TestIntegrationShareCodeUnavailable(t *testing.T) {
	api := newTestAPI(t)
	share := createShareFixture(api, "home", "")
	api.App.Config.WechatAppID, api.App.Config.WechatSecret = "", ""
	status, header, raw := shareCodeResponse(api, share.ID)
	assertShareError(t, status, header, raw, 503)
	for _, item := range []struct {
		name string
		body []byte
		code int
	}{{"未发布页面", []byte(`{"errcode":41030,"errmsg":"upstream-secret"}`), 503}, {"频率限制", []byte(`{"errcode":45009,"errmsg":"upstream-secret"}`), 429}, {"权限故障", []byte(`{"errcode":48001,"errmsg":"upstream-secret"}`), 502}, {"零码JSON也不是图片", []byte(`{"errcode":0}`), 502}, {"HTML故障", []byte(`<html>upstream-secret</html>`), 502}, {"残缺图片", shareImageFixture(t, 430, 430, "png")[:33], 502}} {
		t.Run(item.name, func(t *testing.T) {
			mockWechat(t, api, func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/cgi-bin/stable_token" {
					shareMockToken(w)
					return
				}
				w.Write(item.body)
			})
			status, header, raw := shareCodeResponse(api, share.ID)
			assertShareError(t, status, header, raw, item.code)
			if api.App.Redis.Exists(t.Context(), api.App.shareCodeCacheKey(share.ID)).Val() != 0 {
				t.Fatal("上游错误不允许缓存为可分享码")
			}
		})
	}
}

// TestIntegrationShareCodeRefreshesInvalidTokenOnce 验证凭证失效仅刷新一次，其他错误不盲目重试。
func TestIntegrationShareCodeRefreshesInvalidTokenOnce(t *testing.T) {
	api := newTestAPI(t)
	share := createShareFixture(api, "player", "worker")
	codeImage := shareImageFixture(t, 430, 430, "png")
	var tokenCalls, codeCalls atomic.Int32
	mockWechat(t, api, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/cgi-bin/stable_token" {
			tokenCalls.Add(1)
			shareMockToken(w)
			return
		}
		if codeCalls.Add(1) == 1 {
			json.NewEncoder(w).Encode(map[string]any{"errcode": 40001, "errmsg": "upstream-secret"})
			return
		}
		w.Write(codeImage)
	})
	status, _, _ := shareCodeResponse(api, share.ID)
	if status != 200 || tokenCalls.Load() != 2 || codeCalls.Load() != 2 {
		t.Fatalf("凭证失效应仅刷新后重试一次: status=%d token=%d code=%d", status, tokenCalls.Load(), codeCalls.Load())
	}
}

// TestIntegrationShareCodeRejectsRedirect 验证带访问凭证的官方请求不会跟随重定向到外部主机。
func TestIntegrationShareCodeRejectsRedirect(t *testing.T) {
	api := newTestAPI(t)
	share := createShareFixture(api, "home", "")
	var leaked atomic.Int32
	evil := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		leaked.Add(1)
		w.Write([]byte("不应访问"))
	}))
	t.Cleanup(evil.Close)
	mockWechat(t, api, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/cgi-bin/stable_token" {
			shareMockToken(w)
			return
		}
		http.Redirect(w, r, evil.URL+"/?access_token=secret-token", http.StatusTemporaryRedirect)
	})
	status, header, raw := shareCodeResponse(api, share.ID)
	assertShareError(t, status, header, raw, 502)
	if leaked.Load() != 0 {
		t.Fatal("不允许将微信访问凭证带到重定向地址")
	}
}

// TestIntegrationShareCodeGenerationIsExclusive 验证同一永久码的并发生成只请求官方一次。
func TestIntegrationShareCodeGenerationIsExclusive(t *testing.T) {
	api := newTestAPI(t)
	share := createShareFixture(api, "club", "shanhai")
	codeImage := shareImageFixture(t, 430, 430, "png")
	started, release := make(chan struct{}), make(chan struct{})
	var calls atomic.Int32
	mockWechat(t, api, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/cgi-bin/stable_token" {
			shareMockToken(w)
			return
		}
		if calls.Add(1) == 1 {
			close(started)
		}
		select {
		case <-release:
			w.Write(codeImage)
		case <-r.Context().Done():
		}
	})
	result := make(chan int, 1)
	go func() {
		status, raw, err := api.request("", "GET", "/shares/"+share.ID+"/code", nil)
		if err != nil || status != 200 || !bytes.HasPrefix(raw, []byte("\x89PNG")) {
			result <- -1
			return
		}
		result <- status
	}()
	select {
	case <-started:
	case <-time.After(3 * time.Second):
		close(release)
		t.Fatal("首个请求未进入生成")
	}
	status, header, raw := shareCodeResponse(api, share.ID)
	assertShareError(t, status, header, raw, 409)
	close(release)
	if <-result != 200 || calls.Load() != 1 {
		t.Fatal("并发生成不应重复消耗微信额度")
	}
	if api.App.Redis.Exists(t.Context(), api.App.shareCodeCacheKey(share.ID)+":generating").Val() != 0 {
		t.Fatal("生成结束必须释放互斥标记")
	}
}

// TestShareCodeCacheSeparatesAppID 验证更换源码买家的AppID后不会复用旧小程序码。
func TestShareCodeCacheSeparatesAppID(t *testing.T) {
	a := &App{Config: Config{WechatAppID: "wx_app_one", RedisPrefix: "test:"}}
	first := a.shareCodeCacheKey(strings.Repeat("a", 32))
	a.Config.WechatAppID = "wx_app_two"
	if first == a.shareCodeCacheKey(strings.Repeat("a", 32)) {
		t.Fatal("不同AppID不可共享小程序码缓存")
	}
}
