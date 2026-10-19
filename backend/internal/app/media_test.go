// Package app 验证公开图片上传的格式、尺寸、元数据清理与跨用途访问边界。
package app

import (
	"bytes"
	"encoding/json"
	"image"
	"image/png"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
	"testing"
)

// communityPNG 生成合法 PNG，避免测试依赖用户本地图片。
func communityPNG(t *testing.T, width, height int) []byte {
	t.Helper()
	var buffer bytes.Buffer
	if err := png.Encode(&buffer, image.NewRGBA(image.Rect(0, 0, width, height))); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}

// uploadCommunityImage 使用真实上传路由并检查预期状态，成功时返回公开托管地址。
func (api *testAPI) uploadCommunityImage(user, purpose, clubID string, raw []byte, expected int) string {
	api.t.Helper()
	var buffer bytes.Buffer
	writer := multipart.NewWriter(&buffer)
	part, err := writer.CreateFormFile("file", "image.png")
	if err != nil {
		api.t.Fatal(err)
	}
	if _, err = part.Write(raw); err != nil {
		api.t.Fatal(err)
	}
	if err = writer.Close(); err != nil {
		api.t.Fatal(err)
	}
	request, err := http.NewRequest("POST", api.Server.URL+"/api/media/images?purpose="+purpose+"&club_id="+clubID, &buffer)
	if err != nil {
		api.t.Fatal(err)
	}
	request.Header.Set("Content-Type", writer.FormDataContentType())
	if user != "" {
		request.Header.Set("Authorization", "Bearer "+api.Tokens[user])
	}
	response, err := api.Server.Client().Do(request)
	if err != nil {
		api.t.Fatal(err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil || response.StatusCode != expected {
		api.t.Fatalf("图片上传期望%d 实际%d：%s %v", expected, response.StatusCode, body, err)
	}
	if expected != 200 {
		return ""
	}
	var result struct {
		Data struct {
			URL    string `json:"url"`
			Width  int    `json:"width"`
			Height int    `json:"height"`
			Size   int    `json:"size"`
		} `json:"data"`
	}
	if err = json.Unmarshal(body, &result); err != nil {
		api.t.Fatal(err)
	}
	if !publicImagePath.MatchString(result.Data.URL) || result.Data.Width == 0 || result.Data.Height == 0 || result.Data.Size == 0 {
		api.t.Fatalf("上传结果缺少安全路径或尺寸：%s", body)
	}
	return result.Data.URL
}

// TestPublicImageDimensions 覆盖各用途尺寸、比例和像素总数的边界。
func TestPublicImageDimensions(t *testing.T) {
	cases := []struct {
		purpose       string
		width, height int
		valid         bool
	}{
		{"avatar", 128, 128, true}, {"avatar", 2048, 2048, true}, {"avatar", 127, 127, false}, {"avatar", 256, 255, false}, {"avatar", 2049, 2049, false},
		{"profile_cover", 320, 180, true}, {"club_cover", 2560, 1440, true}, {"club_cover", 640, 359, true}, {"club_cover", 600, 400, false}, {"club_cover", 300, 169, false},
		{"club_detail", 640, 5000, true}, {"club_detail", 319, 800, false}, {"club_detail", 2048, 8192, false}, {"unknown", 640, 360, false},
	}
	for _, test := range cases {
		err := validateImageDimensions(test.purpose, test.width, test.height)
		if (err == nil) != test.valid {
			t.Fatalf("尺寸边界错误 %+v: %v", test, err)
		}
	}
}

// TestPublicImageNormalization 验证伪造头部被拒绝且公开图片不保留尾随私密元数据。
func TestPublicImageNormalization(t *testing.T) {
	raw := communityPNG(t, 256, 256)
	if _, _, _, err := normalizePublicImage(raw[:33], "avatar"); err == nil {
		t.Fatal("只有PNG头部不能作为完整图片")
	}
	raw = append(raw, []byte("GPS-PRIVATE-METADATA")...)
	result, mime, cfg, err := normalizePublicImage(raw, "avatar")
	if err != nil || mime != "image/png" || cfg.Width != 256 || bytes.Contains(result, []byte("GPS-PRIVATE-METADATA")) {
		t.Fatalf("公开图片编码或清理失败：%s %+v %v", mime, cfg, err)
	}
}

// TestIntegrationCommunityImageSecurity 验证登录、文件格式、大小、归属、用途及私密证据隔离。
func TestIntegrationCommunityImageSecurity(t *testing.T) {
	api := newTestAPI(t)
	valid := communityPNG(t, 256, 256)
	api.uploadCommunityImage("", "avatar", "", valid, 401)
	api.uploadCommunityImage("player", "bad-purpose", "", valid, 400)
	api.uploadCommunityImage("player", "avatar", "shanhai", valid, 400)
	api.uploadCommunityImage("player", "avatar", "", []byte("<svg onload='alert(1)'></svg>"), 400)
	api.uploadCommunityImage("player", "avatar", "", communityPNG(t, 64, 64), 400)
	api.uploadCommunityImage("player", "avatar", "", communityPNG(t, 256, 128), 400)
	api.uploadCommunityImage("player", "avatar", "", valid[:33], 400)
	api.uploadCommunityImage("player", "avatar", "", append(append([]byte{}, valid...), make([]byte, publicImageMaxBytes)...), 400)
	api.uploadCommunityImage("player", "avatar", "", make([]byte, 6*1024*1024), 400)
	path := api.uploadCommunityImage("player", "avatar", "", append(valid, []byte("SECRET-EXIF-TAIL")...), 200)
	response, err := api.Server.Client().Get(api.Server.URL + path)
	if err != nil {
		t.Fatal(err)
	}
	body, err := io.ReadAll(response.Body)
	response.Body.Close()
	if err != nil || response.StatusCode != 200 || response.Header.Get("Content-Type") != "image/png" || response.Header.Get("X-Content-Type-Options") != "nosniff" || bytes.Contains(body, []byte("SECRET-EXIF-TAIL")) {
		t.Fatalf("公开图响应不安全: %d %v", response.StatusCode, err)
	}
	api.call("player", "PATCH", "/me/profile", map[string]any{"avatar": path}, 200, nil)
	api.call("outsider", "PATCH", "/me/profile", map[string]any{"avatar": path}, 403, nil)
	api.call("player", "PATCH", "/me/profile", map[string]any{"avatar": path + "?anything=1"}, 400, nil)
	api.call("player", "PATCH", "/me/profile", map[string]any{"avatar": "/api/media/00000000000000000000000000000000"}, 400, nil)
	order := api.newOrder()
	private := api.uploadTestImage("player", order.ID)
	private = strings.Replace(strings.Split(private, "?")[0], "/evidence/", "/media/", 1)
	api.call("player", "PATCH", "/me/profile", map[string]any{"avatar": private}, 400, nil)
	response, err = api.Server.Client().Get(api.Server.URL + private)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != 404 {
		t.Fatal("订单私密图片不得成为公开图片")
	}
	api.call("player", "PATCH", "/me/profile", map[string]any{"avatar": ""}, 200, nil)
}
