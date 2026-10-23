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

// TestIntegrationImagesPostReviewAndEvidenceRetention 验证图片先上传，公开拒绝删除字节，交易证据拒绝仅隔离。
func TestIntegrationImagesPostReviewAndEvidenceRetention(t *testing.T) {
	api := newTestAPI(t)
	o := api.newOrder()
	avatar := api.uploadCommunityImage("player", "avatar", "", communityPNG(t, 256, 256), 200)
	evidence := api.uploadTestImage("player", o.ID)
	api.call("player", "PATCH", "/me/profile", map[string]string{"avatar": avatar}, 200, nil)
	api.call("player", "POST", "/orders/"+o.ID+"/messages", map[string]string{"image_url": evidence}, 200, nil)
	var calls atomic.Int32
	installMockAI(t, api, func(r *http.Request) (*http.Response, error) {
		calls.Add(1)
		raw, _ := io.ReadAll(r.Body)
		if strings.Contains(string(raw), "data:image/png;base64,") {
			return aiMockResponse(`{"decision":"reject","reason":"图片包含违规信息"}`), nil
		}
		return aiMockResponse(`{"decision":"allow","reason":""}`), nil
	})
	api.App.Config.DevMode = false
	api.call("outsider", "PATCH", "/me/profile", map[string]string{"avatar": avatar}, 403, nil)
	if calls.Load() != 0 {
		t.Fatal("越权读取图进入模型")
	}
	drainReviewAI(t, api)
	image, err := get[PublicImage](t.Context(), api.App.DB, "evidence", reviewMediaID(avatar), false)
	if err != nil || len(image.Data) != 0 || image.Moderation != "rejected" || image.Purpose != "avatar" {
		t.Fatal("公开违规图片未删字节或丢失元数据")
	}
	original, err := get[Evidence](t.Context(), api.App.DB, "evidence", reviewMediaID(evidence), false)
	if err != nil || len(original.Data) == 0 || original.Moderation != "rejected" {
		t.Fatal("交易原证据丢失或未隔离")
	}
	saved, _ := get[User](t.Context(), api.App.DB, "users", "player", false)
	if saved.Avatar != rejectedImageURL {
		t.Fatal("头像引用未替换")
	}
	res, err := api.Server.Client().Get(api.Server.URL + avatar)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != 200 || res.Header.Get("Content-Type") != "image/png" || res.Header.Get("Cache-Control") != "private, no-store" {
		t.Fatal("旧图片ID仍可取原图")
	}
	api.uploadCommunityImage("player", "avatar", "", communityPNG(t, 256, 256), 200)
	api.App.AIHTTPClient = &http.Client{Transport: paymentTransport(func(*http.Request) (*http.Response, error) { return nil, context.DeadlineExceeded })}
	drainReviewAI(t, api)
}
