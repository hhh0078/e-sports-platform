// Package app 验证管理员封面上传与订单私密图片之间的访问边界。
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

// TestIntegrationCustomCovers 验证自定义封面可配置发布且私密图片不能公开读取。
func TestIntegrationCustomCovers(t *testing.T) {
	api := newTestAPI(t)
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", "cover.png")
	if err != nil {
		t.Fatal(err)
	}
	png.Encode(part, image.NewRGBA(image.Rect(0, 0, 4, 4)))
	writer.Close()
	req, _ := http.NewRequest("POST", api.Server.URL+"/api/admin/covers", &body)
	req.Header.Set("Authorization", "Bearer "+api.Tokens["admin"])
	req.Header.Set("Content-Type", writer.FormDataContentType())
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := io.ReadAll(res.Body)
	res.Body.Close()
	if res.StatusCode != 200 {
		t.Fatalf("封面上传失败：%s", raw)
	}
	var response struct {
		Data struct {
			URL string `json:"url"`
		} `json:"data"`
	}
	json.Unmarshal(raw, &response)
	var games []Game
	api.call("admin", "GET", "/admin/games", nil, 200, &games)
	g := games[0]
	g.Covers = append(g.Covers, response.Data.URL)
	g.CoverURL = response.Data.URL
	api.call("admin", "PATCH", "/admin/games/"+g.ID, g, 200, nil)
	res, err = http.Get(api.Server.URL + response.Data.URL)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != 200 {
		t.Fatal("已发布封面必须可公开读取")
	}
	g.Covers = append(g.Covers, "/api/media/00000000000000000000000000000000")
	api.call("admin", "PATCH", "/admin/games/"+g.ID, g, 400, nil)
	o := api.newOrder()
	private := api.uploadTestImage("player", o.ID)
	private = strings.Split(private, "?")[0]
	res, err = http.Get(api.Server.URL + strings.Replace(private, "/evidence/", "/media/", 1))
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != 404 {
		t.Fatal("私密证据不得经封面接口泄漏")
	}
}
