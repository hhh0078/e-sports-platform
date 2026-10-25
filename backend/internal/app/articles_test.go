// Package app 验证Markdown解析、文章发布权限、公开分页、后置审核及跳转目标最小化输出。
package app

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

// TestArticleMarkdownSecurity 验证成熟解析器正常排版及脚本、原始HTML和外部图片拒绝。
func TestArticleMarkdownSecurity(t *testing.T) {
	body, _, err := renderArticleMarkdown(context.Background(), nil, "# 标题\n\n**重点**与`<script>`。\n\n[官网](https://example.com/path)", "", User{}, true)
	if err != nil || !strings.Contains(body, "<h1>") || !strings.Contains(body, "<strong>重点</strong>") || strings.Contains(body, "<script>") {
		t.Fatalf("Markdown安全渲染异常：%s %v", body, err)
	}
	for _, markdown := range []string{"<script>alert(1)</script>", "文本<img src=x onerror=alert(1)>", "[危险](javascript:alert(1))", "[危险](data:text/html,bad)", "[危险](//evil.example/path)", "[证据](/api/evidence/0123456789abcdef0123456789abcdef)", "[私密](https://game.ai.58it.cn/api/orders/private)", "![外图](https://evil.example/a.png)", "![证据](/api/evidence/0123456789abcdef0123456789abcdef)", strings.Repeat("文", articleMarkdownMaxBytes)} {
		if _, _, err = renderArticleMarkdown(context.Background(), nil, markdown, "", User{}, true); err == nil {
			t.Fatalf("不安全Markdown未拒绝：%.60s", markdown)
		}
	}
}

// TestArticleMarkdownChineseHardWraps 验证用户正常输入的中文两行由同一安全解析器保留换行。
func TestArticleMarkdownChineseHardWraps(t *testing.T) {
	body, _, err := renderArticleMarkdown(context.Background(), nil, "第一行中文说明\n第二行中文说明", "", User{}, true)
	if err != nil || !strings.Contains(body, "第一行中文说明<br>\n第二行中文说明") {
		t.Fatalf("中文普通换行未保留：%s %v", body, err)
	}
}

// TestIntegrationContentTargetSearchScope 验证超过百条后的搜索、真实需求过滤及最小公开DTO。
func TestIntegrationContentTargetSearchScope(t *testing.T) {
	api := newTestAPI(t)
	for index := 0; index < 105; index++ {
		service := Service{ID: fmt.Sprintf("target-%03d", index), Title: fmt.Sprintf("目标服务-%03d", index), ProviderID: "worker", GameID: "delta", Status: "active", CreatedAt: "2099-01-01T00:00:00Z"}
		if err := put(t.Context(), api.App.DB, "services", service.ID, service); err != nil {
			t.Fatal(err)
		}
	}
	var page struct {
		Items []ContentTargetOption `json:"items"`
		Total int                   `json:"total"`
	}
	api.call("admin", "GET", "/admin/content-targets?kind=service&q=目标服务-000&limit=1", nil, 200, &page)
	if page.Total != 1 || len(page.Items) != 1 || page.Items[0].ID != "target-000" {
		t.Fatal("目标搜索只筛了已加载首屏")
	}
	for index, status := range []string{"open", "open", "accepted", "open"} {
		order := Order{ID: fmt.Sprintf("target-request-%d", index), OrderNo: fmt.Sprintf("TARGET-%d", index), Title: "目标需求", GameID: "delta", BuyerID: "player", Status: status, PaymentStatus: "paid", RefundStatus: "none", Requirements: "PRIVATE-REQUIREMENTS-SECRET", PayOpenID: "PRIVATE-OPENID", PaymentTransactionID: fmt.Sprintf("PRIVATE-TRANSACTION-%d", index)}
		if index == 1 {
			order.PaymentStatus = "unpaid"
		}
		if index == 3 {
			order.ServiceID = "delta-service-1"
		}
		if err := put(t.Context(), api.App.DB, "orders", order.ID, order); err != nil {
			t.Fatal(err)
		}
	}
	api.call("admin", "GET", "/admin/content-targets?kind=request&q=目标需求", nil, 200, &page)
	if page.Total != 1 || len(page.Items) != 1 || page.Items[0].ID != "target-request-0" {
		t.Fatal("未付款、已接单或服务订单进入公开需求选择器")
	}
	raw, _ := json.Marshal(page)
	if strings.Contains(string(raw), "PRIVATE-") {
		t.Fatal("需求目标DTO泄露原始约定或支付身份")
	}
}

// TestIntegrationArticlesAndTargets 验证草稿公开边界、真实搜索分页、审核替换和图像来源校验。
func TestIntegrationArticlesAndTargets(t *testing.T) {
	api := newTestAPI(t)
	api.call("player", "POST", "/admin/articles", map[string]any{}, 403, nil)
	api.call("", "GET", "/admin/articles", nil, 401, nil)
	image := api.uploadCommunityImage("admin", "article_image", "", communityPNG(t, 400, 900), 200)
	cover := api.uploadCommunityImage("admin", "article_cover", "", communityPNG(t, 640, 360), 200)
	var article Article
	input := map[string]any{"title": "组队服务指南", "summary": "保持礼貌，先沟通", "cover_url": cover, "body_markdown": "# 服务规则\n\n文字说明。\n\n![图文说明](" + image + ")", "status": "draft"}
	api.call("admin", "POST", "/admin/articles", input, 200, &article)
	api.call("", "GET", "/articles/"+article.ID, nil, 404, nil)
	api.call("", "GET", "/articles?status=draft", nil, 200, nil)
	input["status"], input["revision"] = "published", article.Revision
	api.call("admin", "PATCH", "/admin/articles/"+article.ID, input, 200, &article)
	var public map[string]any
	api.call("", "GET", "/articles/"+article.ID, nil, 200, &public)
	if public["body_markdown"] != nil || public["editor_id"] != nil || !strings.Contains(public["body_html"].(string), image) {
		t.Fatal("文章公开DTO或托管图渲染异常")
	}
	api.call("admin", "PATCH", "/admin/articles/"+article.ID, input, 409, nil)
	var targets struct {
		Items []ContentTargetOption `json:"items"`
		Total int                   `json:"total"`
	}
	api.call("admin", "GET", "/admin/content-targets?kind=article&q=组队&limit=1&offset=0", nil, 200, &targets)
	if targets.Total != 1 || len(targets.Items) != 1 || targets.Items[0].ID != article.ID {
		t.Fatal("目标搜索未在公开全集分页")
	}
	api.call("player", "GET", "/admin/content-targets?kind=article", nil, 403, nil)
	api.call("admin", "GET", "/admin/content-targets?kind=club", nil, 400, nil)
	task := latestReviewTask(t, api, "articles", article.ID, "body_markdown")
	if task.Status != "manual_pending" {
		t.Fatal("文章未进入后置人工审核")
	}
	api.call("admin", "POST", "/admin/moderation/"+task.ID+"/decision", map[string]string{"decision": "reject", "reason": "人工测试拒绝"}, 200, nil)
	api.call("", "GET", "/articles/"+article.ID, nil, 200, &public)
	if strings.Contains(public["body_html"].(string), "服务规则") || !strings.Contains(public["body_html"].(string), rejectedContentText) {
		t.Fatal("审核修改后仍公开旧HTML缓存")
	}
	input["revision"], input["body_markdown"], input["status"] = article.Revision, "正常更新后的正文内容", "hidden"
	api.call("admin", "PATCH", "/admin/articles/"+article.ID, input, 200, &article)
	api.call("", "GET", "/articles/"+article.ID, nil, 404, nil)
	api.call("admin", "GET", "/admin/content-targets?kind=article&q=组队", nil, 200, &targets)
	if targets.Total != 0 {
		t.Fatal("已隐藏文章仍可选作跳转")
	}
	for _, key := range []string{"openid", "phone_number", "buyer_id", "payment_transaction_id", "editor_id"} {
		raw, _ := json.Marshal(targets)
		if strings.Contains(string(raw), key) {
			t.Fatalf("跳转搜索泄露私密字段：%s", key)
		}
	}
}
