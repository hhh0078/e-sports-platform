// Package app 验证公共内容图片撤销、动态字段旧审核防覆盖与实际管理俱乐部完整分页范围。
package app

import (
	"fmt"
	"strings"
	"testing"
)

// TestIntegrationManagedClubsScope 验证完整管理集合先于分页，普通成员和平台角色不能冒充实际负责组织。
func TestIntegrationManagedClubsScope(t *testing.T) {
	api := newTestAPI(t)
	for index := 0; index < 110; index++ {
		club := Club{ID: fmt.Sprintf("other-club-%03d", index), Name: "其它公开组织", OwnerID: "outsider", Description: "不属于当前负责人", Status: "active", GameIDs: []string{"delta"}, CreatedAt: "2099-01-01T00:00:00Z"}
		if err := put(t.Context(), api.App.DB, "clubs", club.ID, club); err != nil {
			t.Fatal(err)
		}
	}
	api.call("", "GET", "/clubs?managed=1", nil, 401, nil)
	api.call("worker", "GET", "/clubs?managed=1&mine=1", nil, 400, nil)
	var clubs []Club
	api.call("clubowner", "GET", "/clubs?managed=1&limit=20&offset=0", nil, 200, &clubs)
	if len(clubs) != 1 || clubs[0].ID != "shanhai" || clubs[0].MembershipStatus != "owner" || clubs[0].ManagementRole != "owner" {
		t.Fatalf("未从完整管理集合筛选：%+v", clubs)
	}
	api.call("admin", "GET", "/clubs?managed=1", nil, 200, &clubs)
	if len(clubs) != 0 {
		t.Fatal("平台管理员身份不应冒充本人负责全部组织")
	}
	api.call("worker", "GET", "/clubs?managed=1", nil, 200, &clubs)
	if len(clubs) != 0 {
		t.Fatal("普通认证成员不等于组织管理员")
	}
	deputy := ClubDeputy{ClubID: "shanhai", UserID: "worker", Status: "active", AssignedBy: "clubowner", CreatedAt: now()}
	if err := put(t.Context(), api.App.DB, "club_deputies", "shanhai:worker", deputy); err != nil {
		t.Fatal(err)
	}
	api.call("worker", "GET", "/clubs?managed=1", nil, 200, &clubs)
	if len(clubs) != 1 || clubs[0].MembershipStatus != "deputy" || clubs[0].CanManageAdmins {
		t.Fatal("有效副管理员未加入管理范围或错误获得授撤权")
	}
	member, err := get[ClubMember](t.Context(), api.App.DB, "members", "shanhai:worker", false)
	if err != nil {
		t.Fatal(err)
	}
	member.Status = "left"
	if err = put(t.Context(), api.App.DB, "members", "shanhai:worker", member); err != nil {
		t.Fatal(err)
	}
	api.call("worker", "GET", "/clubs?managed=1", nil, 200, &clubs)
	if len(clubs) != 0 {
		t.Fatal("成员关系失效后仍有管理入口")
	}
}

// TestIntegrationPublicContentLateReview 验证轮播换序后的迟到旧文字决定不覆盖新图层或其它位置。
func TestIntegrationPublicContentLateReview(t *testing.T) {
	api := newTestAPI(t)
	banner := api.uploadCommunityImage("admin", "promotion_banner", "", communityPNG(t, 640, 360), 200)
	first := ContentSlide{ID: "first", Kind: "image", ImageURL: banner, Labels: []CoverLabel{{ID: "line", Text: "旧版文字", Color: "#FFFFFF", Background: "transparent", Position: "bottom-left", FontSize: 16}}}
	second := first
	second.ID = "second"
	second.Labels = []CoverLabel{{ID: "line", Text: "第二张文字", Color: "#FFFFFF", Background: "transparent", Position: "top-left", FontSize: 20}}
	var config PublicContentSettings
	api.call("admin", "PATCH", "/admin/public-content", map[string]any{"revision": 0, "global_slides": []ContentSlide{first, second}}, 200, &config)
	code := "global_slide." + reviewHash("first") + ".label." + reviewHash("line")
	oldTask := latestReviewTask(t, api, "settings", publicContentID, code)
	first.Labels[0].Text = "新版文字"
	api.call("admin", "PATCH", "/admin/public-content", map[string]any{"revision": config.Revision, "global_slides": []ContentSlide{second, first}}, 200, &config)
	api.call("admin", "POST", "/admin/moderation/"+oldTask.ID+"/decision", map[string]string{"decision": "reject", "reason": "迟到旧结果"}, 409, nil)
	var public PublicContentDTO
	api.call("", "GET", "/public-content?game_id=delta", nil, 200, &public)
	if public.Slides[0].Labels[0].Text != "第二张文字" || public.Slides[1].Labels[0].Text != "新版文字" {
		t.Fatal("旧审核按旧数组索引覆盖了重排后的内容")
	}
	newTask := latestReviewTask(t, api, "settings", publicContentID, code)
	api.call("admin", "POST", "/admin/moderation/"+newTask.ID+"/decision", map[string]string{"decision": "reject", "reason": "拒绝新字段"}, 200, nil)
	api.call("", "GET", "/public-content?game_id=delta", nil, 200, &public)
	if public.Slides[0].Labels[0].Text != "第二张文字" || public.Slides[1].Labels[0].Text != rejectedContentText {
		t.Fatal("新审核未只作用于对应重排字段")
	}
}

// TestIntegrationArticleImageRevocation 验证正文图只接受本用途、审核拒绝删除实际字节且文章返回占位。
func TestIntegrationArticleImageRevocation(t *testing.T) {
	api := newTestAPI(t)
	image := api.uploadCommunityImage("admin", "article_image", "", communityPNG(t, 333, 777), 200)
	var article Article
	input := map[string]any{"title": "图文服务说明", "summary": "正常图文", "body_markdown": "图文服务说明。\n\n![说明](" + image + ")", "cover_url": "", "status": "published"}
	api.call("admin", "POST", "/admin/articles", input, 200, &article)
	task := latestReviewTask(t, api, "evidence", strings.TrimPrefix(image, "/api/media/"), "image")
	api.call("admin", "POST", "/admin/moderation/"+task.ID+"/decision", map[string]string{"decision": "reject", "reason": "图片测试拒绝"}, 200, nil)
	var public struct {
		BodyHTML string `json:"body_html"`
	}
	api.call("", "GET", "/articles/"+article.ID, nil, 200, &public)
	if !strings.Contains(public.BodyHTML, rejectedImageURL) || strings.Contains(public.BodyHTML, image) {
		t.Fatal("图片拒绝后文章仍使用原图地址")
	}
	media, err := get[Evidence](t.Context(), api.App.DB, "evidence", strings.TrimPrefix(image, "/api/media/"), false)
	if err != nil || len(media.Data) != 0 || media.Moderation != "rejected" {
		t.Fatal("公开违规图片原始字节未删除")
	}
	wrongPurpose := api.uploadCommunityImage("admin", "article_cover", "", communityPNG(t, 640, 360), 200)
	input["body_markdown"] = "错误用途图片。\n\n![封面](" + wrongPurpose + ")"
	api.call("admin", "POST", "/admin/articles", input, 400, nil)
	api.call("admin", "POST", "/admin/articles/preview", map[string]string{"body_markdown": "<iframe src='https://example.com'></iframe>"}, 400, nil)
}

// TestIntegrationArticleOrdinaryMissingImage 验证普通丢失图片仍由原媒体入口返回404，不冒充审核失败。
func TestIntegrationArticleOrdinaryMissingImage(t *testing.T) {
	api := newTestAPI(t)
	image := api.uploadCommunityImage("admin", "article_image", "", communityPNG(t, 350, 400), 200)
	var article Article
	api.call("admin", "POST", "/admin/articles", map[string]any{"title": "普通图文说明", "summary": "", "body_markdown": "普通文字内容。\n\n![说明](" + image + ")", "cover_url": "", "status": "published"}, 200, &article)
	if _, err := api.App.DB.Exec(t.Context(), "DELETE FROM evidence WHERE id=$1", strings.TrimPrefix(image, "/api/media/")); err != nil {
		t.Fatal(err)
	}
	var public struct {
		BodyHTML string `json:"body_html"`
	}
	api.call("", "GET", "/articles/"+article.ID, nil, 200, &public)
	if strings.Contains(public.BodyHTML, rejectedImageURL) || !strings.Contains(public.BodyHTML, image) {
		t.Fatal("普通资源丢失被错误标记为审核失败")
	}
}
