// Package app 验证共享微信JSON请求遇到跳转时不会重放AppSecret、登录code或访问凭证。
package app

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// TestWechatJSONRejectsCredentialRedirects 使用真实HTTP传输验证307/308与同域、跨域跳转都不泄露凭证。
func TestWechatJSONRejectsCredentialRedirects(t *testing.T) {
	for _, status := range []int{http.StatusTemporaryRedirect, http.StatusPermanentRedirect} {
		for _, crossHost := range []bool{false, true} {
			t.Run(http.StatusText(status)+map[bool]string{false: "同主机", true: "跨主机"}[crossHost], func(t *testing.T) {
				var originRequests, movedRequests atomic.Int32
				var receivedSecret atomic.Bool
				moved := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					movedRequests.Add(1)
					w.Write([]byte(`{"access_token":"不允许读取"}`))
				}))
				t.Cleanup(moved.Close)
				origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.URL.Path == "/moved" {
						movedRequests.Add(1)
						w.Write([]byte(`{"access_token":"不允许读取"}`))
						return
					}
					originRequests.Add(1)
					body, _ := io.ReadAll(r.Body)
					receivedSecret.Store(strings.Contains(string(body), "test-appsecret-never-redirect"))
					target := "/moved"
					if crossHost {
						target = moved.URL + "/moved"
					}
					http.Redirect(w, r, target, status)
				}))
				t.Cleanup(origin.Close)
				base, _ := url.Parse(origin.URL)
				a := &App{HTTPClient: &http.Client{Timeout: 3 * time.Second, Transport: paymentTransport(func(r *http.Request) (*http.Response, error) {
					if r.URL.Host == "api.weixin.qq.com" {
						clone := r.Clone(r.Context())
						target := *r.URL
						target.Host, target.Scheme = base.Host, base.Scheme
						clone.URL = &target
						return http.DefaultTransport.RoundTrip(clone)
					}
					return http.DefaultTransport.RoundTrip(r)
				})}}
				var output map[string]any
				err := a.wechatJSON(context.Background(), http.MethodPost, "https://api.weixin.qq.com/cgi-bin/stable_token", map[string]string{"appid": "wx_test", "secret": "test-appsecret-never-redirect"}, &output)
				var problem *apiError
				if !errors.As(err, &problem) || problem.Status != 502 || originRequests.Load() != 1 || !receivedSecret.Load() || movedRequests.Load() != 0 || output != nil || strings.Contains(err.Error(), "test-appsecret-never-redirect") {
					t.Fatalf("微信凭证请求必须在首个跳转处失败且不得重放: origin=%d moved=%d error=%v", originRequests.Load(), movedRequests.Load(), err)
				}
			})
		}
	}
}
