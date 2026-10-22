// Package app 验证图像上传与历史引用补审，不允许本地模拟图片被当作正式审核证明。
package app

import (
	"bytes"
	"context"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
)

// uploadModerationEvidence 通过实际私密图片上传接口验证拒绝及失败时不持久化原始数据。
func uploadModerationEvidence(t *testing.T, api *testAPI, orderID string, expected int) {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", "evidence.png")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = part.Write(communityPNG(t, 128, 128)); err != nil {
		t.Fatal(err)
	}
	writer.Close()
	req, _ := http.NewRequest("POST", api.Server.URL+"/api/orders/"+orderID+"/evidence", &body)
	req.Header.Set("Authorization", "Bearer "+api.Tokens["player"])
	req.Header.Set("Content-Type", writer.FormDataContentType())
	res, err := api.Server.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)
	if res.StatusCode != expected {
		t.Fatalf("图片审查状态错误%d %s", res.StatusCode, raw)
	}
}

// TestIntegrationImagesRejectedAndLegacyRechecked 验证图片违规/模型故障拒绝及历史素材真实字节补审。
func TestIntegrationImagesRejectedAndLegacyRechecked(t *testing.T) {
	api := newTestAPI(t)
	o := api.newOrder()
	legacyAvatar := api.uploadCommunityImage("player", "avatar", "", communityPNG(t, 256, 256), 200)
	legacyEvidence := api.uploadTestImage("player", o.ID)
	var count int
	if err := api.App.DB.QueryRow(t.Context(), "SELECT count(*) FROM evidence").Scan(&count); err != nil {
		t.Fatal(err)
	}
	var calls atomic.Int32
	installMockAI(t, api, func(r *http.Request) (*http.Response, error) {
		calls.Add(1)
		raw, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(raw), "data:image/png;base64,") {
			t.Error("历史图片没有发送实际字节")
		}
		return aiMockResponse(`{"decision":"reject","reason":"图片包含违规信息"}`), nil
	})
	api.App.Config.DevMode = false
	api.call("outsider", "PATCH", "/me/profile", map[string]string{"avatar": legacyAvatar}, 403, nil)
	if calls.Load() != 0 {
		t.Fatal("越权读取的图片进入AI服务")
	}
	api.call("player", "PATCH", "/me/profile", map[string]string{"avatar": legacyAvatar}, 422, nil)
	api.call("player", "POST", "/orders/"+o.ID+"/messages", map[string]string{"image_url": legacyEvidence}, 422, nil)
	api.uploadCommunityImage("player", "avatar", "", communityPNG(t, 256, 256), 422)
	uploadModerationEvidence(t, api, o.ID, 422)
	var after int
	api.App.DB.QueryRow(t.Context(), "SELECT count(*) FROM evidence").Scan(&after)
	if after != count {
		t.Fatal("被拒绝图片仍然持久化")
	}
	api.App.AIHTTPClient = &http.Client{Transport: paymentTransport(func(*http.Request) (*http.Response, error) { return nil, context.DeadlineExceeded })}
	api.uploadCommunityImage("player", "avatar", "", communityPNG(t, 256, 256), 503)
	uploadModerationEvidence(t, api, o.ID, 503)
	api.App.AIHTTPClient = &http.Client{Transport: paymentTransport(func(*http.Request) (*http.Response, error) {
		calls.Add(1)
		return aiMockResponse(`{"decision":"allow","reason":""}`), nil
	})}
	api.call("player", "PATCH", "/me/profile", map[string]string{"avatar": legacyAvatar}, 200, nil)
	match := publicImagePath.FindStringSubmatch(legacyAvatar)
	image, err := get[Evidence](t.Context(), api.App.DB, "evidence", match[1], false)
	if err != nil || image.Moderation != "ai" {
		t.Fatal("正式图片补审结果没有持久化")
	}
	before := calls.Load()
	api.call("player", "PATCH", "/me/profile", map[string]string{"avatar": legacyAvatar}, 200, nil)
	if calls.Load() != before {
		t.Fatal("已审不可变图片不应重复调用模型")
	}
}
