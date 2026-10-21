// Package app 验证公开主页隐私、俱乐部装修权限、入会规则确认与成员认证分离。
package app

import (
	"encoding/json"
	"net/url"
	"strings"
	"sync"
	"testing"
)

// TestIntegrationClubSearchBeforePagination 验证名称简介字面搜索、分页与本人挂靠范围同时生效。
func TestIntegrationClubSearchBeforePagination(t *testing.T) {
	api := newTestAPI(t)
	fixtures := []Club{
		{ID: "search-owned-name", Name: "检索 Alpha", Description: "常规小队介绍", OwnerID: "clubowner", Status: "active", CreatedAt: "2026-10-22T00:00:01Z"},
		{ID: "search-owned-description", Name: "普通小队", Description: "检索：描述关键词", OwnerID: "clubowner", Status: "active", CreatedAt: "2026-10-22T00:00:02Z"},
		{ID: "search-member", Name: "检索认证小队", Description: "另一位负责人创建的俱乐部", OwnerID: "outsider", Status: "active", CreatedAt: "2026-10-22T00:00:03Z"},
		{ID: "search-owned-pending", Name: "检索待审核小队", Description: "只有本人可以看到的申请", OwnerID: "clubowner", Status: "pending", CreatedAt: "2026-10-22T00:00:04Z"},
		{ID: "search-foreign", Name: "检索独立小队", Description: "没有本人挂靠记录", OwnerID: "outsider", Status: "active", CreatedAt: "2026-10-22T00:00:05Z"},
		{ID: "search-unmatched", Name: "无关小队", Description: "不应占用搜索结果页", OwnerID: "clubowner", Status: "active", CreatedAt: "2026-10-22T00:00:06Z"},
	}
	for _, club := range fixtures {
		if err := put(t.Context(), api.App.DB, "clubs", club.ID, club); err != nil {
			t.Fatal(err)
		}
	}
	member := ClubMember{UserID: "worker", ClubID: "search-member", Status: "active", CreatedAt: "2026-10-22T00:00:07Z"}
	if err := put(t.Context(), api.App.DB, "members", member.ClubID+":"+member.UserID, member); err != nil {
		t.Fatal(err)
	}
	checks := []struct {
		name, actor, query, extra string
		want                      []string
	}{
		{"名称忽略大小写并去除空白", "", " alpha ", "", []string{"search-owned-name"}},
		{"简介关键词", "", "描述关键词", "", []string{"search-owned-description"}},
		{"不匹配为空", "", "不存在的专属关键词", "", nil},
		{"SQL文本不是指令", "", "' OR 1=1 --", "", nil},
		{"百分号不是通配符", "", "%", "", nil},
		{"筛选后第一页", "", "检索", "&limit=2", []string{"search-foreign", "search-member"}},
		{"筛选后第二页", "", "检索", "&limit=2&offset=2", []string{"search-owned-description", "search-owned-name"}},
		{"翻页结束", "", "检索", "&limit=2&offset=4", nil},
		{"本人负责含待审核", "clubowner", "检索", "&mine=1", []string{"search-owned-pending", "search-owned-description", "search-owned-name"}},
		{"本人范围筛选后分页", "clubowner", "检索", "&mine=1&limit=1&offset=1", []string{"search-owned-description"}},
		{"成员挂靠范围", "worker", "检索", "&mine=1", []string{"search-member"}},
	}
	for _, check := range checks {
		t.Run(check.name, func(t *testing.T) {
			var clubs []Club
			api.call(check.actor, "GET", "/clubs?q="+url.QueryEscape(check.query)+check.extra, nil, 200, &clubs)
			if len(clubs) != len(check.want) {
				t.Fatalf("搜索结果数量不正确：期望 %v，实际 %+v", check.want, clubs)
			}
			for i, club := range clubs {
				if club.ID != check.want[i] {
					t.Fatalf("搜索分页顺序或可见范围不正确：期望 %v，实际 %+v", check.want, clubs)
				}
			}
		})
	}
	api.call("", "GET", "/clubs?mine=1&q="+url.QueryEscape("检索"), nil, 401, nil)
	api.call("", "GET", "/clubs?q="+url.QueryEscape(strings.Repeat("字", 101)), nil, 400, nil)
}

// TestIntegrationCommunityClubLifecycle 覆盖俱乐部装修、重新审核、入会考核及跨俱乐部隔离。
func TestIntegrationCommunityClubLifecycle(t *testing.T) {
	api := newTestAPI(t)
	cover := api.uploadCommunityImage("clubowner", "club_cover", "shanhai", communityPNG(t, 640, 360), 200)
	detail := api.uploadCommunityImage("clubowner", "club_detail", "shanhai", communityPNG(t, 640, 960), 200)
	api.uploadCommunityImage("outsider", "club_cover", "shanhai", communityPNG(t, 640, 360), 403)
	var club Club
	api.call("clubowner", "PATCH", "/clubs/shanhai", map[string]any{"cover_url": cover, "detail_blocks": []CommunityBlock{{Type: "text", Text: "团队规则与服务介绍"}, {Type: "image", ImageURL: detail}}, "deposit_cents": 50000, "assessment": "完成一次语音与游戏配合考核"}, 200, &club)
	if club.Status != "pending" || club.Verified || club.Name != "山海电竞" || club.DepositCents != 50000 || len(club.DetailBlocks) != 2 {
		t.Fatalf("装修应保留基本资料并重新审核：%+v", club)
	}
	api.call("", "GET", "/clubs/shanhai", nil, 404, nil)
	api.call("clubowner", "GET", "/clubs/shanhai", nil, 200, nil)
	api.call("outsider", "PATCH", "/clubs/shanhai", map[string]any{"description": "尝试覆盖其他俱乐部的公开资料"}, 403, nil)
	api.call("outsider", "PATCH", "/clubs/wild", map[string]any{"cover_url": cover}, 403, nil)
	api.call("clubowner", "PATCH", "/clubs/shanhai", map[string]any{"detail_blocks": []CommunityBlock{{Type: "html", Text: "<script>unsafe</script>"}}}, 400, nil)
	api.call("clubowner", "PATCH", "/clubs/shanhai", map[string]any{"deposit_cents": -1}, 400, nil)
	api.call("clubowner", "PATCH", "/clubs/shanhai", map[string]any{"verified": true}, 400, nil)
	api.call("admin", "PATCH", "/admin/clubs/shanhai", map[string]any{"status": "active", "verified": true}, 200, nil)

	api.call("outsider", "POST", "/clubs/shanhai/join", map[string]any{}, 400, nil)
	api.call("outsider", "POST", "/clubs/shanhai/join", map[string]any{"requirements_accepted": true, "application_note": "PRIVATE-APPLICATION-NOTE"}, 200, nil)
	var own struct {
		Members      []map[string]any `json:"members"`
		MyMembership map[string]any   `json:"my_membership"`
	}
	api.call("outsider", "GET", "/clubs/shanhai", nil, 200, &own)
	if own.MyMembership["status"] != "pending" || own.MyMembership["certified"] != false || own.MyMembership["deposit_status"] != "unverified" || own.MyMembership["applied_deposit_cents"] != float64(50000) {
		t.Fatalf("申请不得自动认证或冒充已收押金：%+v", own.MyMembership)
	}
	for _, member := range own.Members {
		if member["user_id"] == "outsider" {
			t.Fatal("待审核申请不得出现在公开成员列表")
		}
	}
	code, raw, err := api.request("", "GET", "/clubs/shanhai", nil)
	if err != nil || code != 200 || strings.Contains(string(raw), "PRIVATE-APPLICATION-NOTE") || strings.Contains(string(raw), "requirements_accepted") {
		t.Fatalf("申请资料不得泄漏给匿名访客：%s %v", raw, err)
	}
	api.call("outsider", "POST", "/clubs/shanhai/members/outsider", map[string]any{"status": "active", "assessment_passed": true}, 403, nil)
	api.call("clubowner", "POST", "/clubs/shanhai/members/outsider", map[string]any{"status": "active"}, 400, nil)
	api.call("clubowner", "POST", "/clubs/shanhai/members/outsider", map[string]any{"status": "active", "assessment_passed": true, "review_note": "PRIVATE-REVIEW-NOTE"}, 200, nil)
	api.call("outsider", "GET", "/clubs/shanhai", nil, 200, &own)
	if own.MyMembership["certified"] != true || own.MyMembership["deposit_status"] != "unverified" {
		t.Fatalf("通过考核仍不得声称押金已经托管：%+v", own.MyMembership)
	}
	_, raw, _ = api.request("", "GET", "/clubs/shanhai", nil)
	if strings.Contains(string(raw), "PRIVATE-REVIEW-NOTE") || strings.Contains(string(raw), "PRIVATE-APPLICATION-NOTE") {
		t.Fatal("公开认证成员不得携带申请或审核私密资料")
	}
	api.call("outsider", "POST", "/clubs/shanhai/leave", map[string]any{}, 200, nil)
	api.call("clubowner", "POST", "/clubs/shanhai/members/outsider", map[string]any{"status": "active", "assessment_passed": true}, 409, nil)
	api.call("outsider", "POST", "/clubs/shanhai/join", map[string]any{"requirements_accepted": true}, 200, nil)
	api.call("clubowner", "PATCH", "/clubs/shanhai", map[string]any{"deposit_cents": 60000}, 200, nil)
	api.call("admin", "PATCH", "/admin/clubs/shanhai", map[string]any{"status": "active", "verified": true}, 200, nil)
	api.call("clubowner", "POST", "/clubs/shanhai/members/outsider", map[string]any{"status": "active", "assessment_passed": true}, 409, nil)
	api.call("outsider", "POST", "/clubs/shanhai/join", map[string]any{"requirements_accepted": true}, 200, nil)
	api.call("admin", "POST", "/clubs/shanhai/members/outsider", map[string]any{"status": "active", "assessment_passed": true}, 200, nil)

	adminCover := api.uploadCommunityImage("admin", "club_cover", "shanhai", communityPNG(t, 640, 360), 200)
	api.call("admin", "PATCH", "/clubs/shanhai", map[string]any{"cover_url": adminCover}, 200, nil)
	api.call("clubowner", "PATCH", "/clubs/shanhai", map[string]any{"description": "管理员更新封面后负责人仍可继续装修这个主页"}, 200, nil)
	api.call("admin", "PATCH", "/admin/clubs/shanhai", map[string]any{"status": "active", "verified": true}, 200, nil)
	api.call("admin", "PATCH", "/admin/users/outsider", map[string]any{"status": "frozen"}, 200, nil)
	var public struct {
		Members []map[string]any `json:"members"`
	}
	api.call("", "GET", "/clubs/shanhai", nil, 200, &public)
	for _, member := range public.Members {
		if member["user_id"] == "outsider" {
			t.Fatal("停用账号不能在公开认证成员中出现")
		}
	}
}

// TestIntegrationCommunityPublicProfile 验证公开资料白名单、服务可见性与私人资料并发保存。
func TestIntegrationCommunityPublicProfile(t *testing.T) {
	api := newTestAPI(t)
	avatar := api.uploadCommunityImage("worker", "avatar", "", communityPNG(t, 256, 256), 200)
	cover := api.uploadCommunityImage("worker", "profile_cover", "", communityPNG(t, 640, 360), 200)
	api.call("worker", "PATCH", "/me/profile", map[string]any{"nickname": "公开资料测试", "bio": "我的公开游戏介绍", "avatar": avatar, "profile_cover": cover}, 200, nil)
	api.call("outsider", "PATCH", "/me/profile", map[string]any{"avatar": avatar}, 403, nil)
	api.call("worker", "PATCH", "/me/profile", map[string]any{"avatar": cover}, 400, nil)
	api.call("worker", "PATCH", "/me/profile", map[string]any{"avatar": "https://example.com/tracker.png"}, 400, nil)
	api.call("worker", "PATCH", "/me/profile", map[string]any{"role": "admin"}, 400, nil)
	api.call("worker", "PATCH", "/me/profile", map[string]any{"nickname": "一"}, 400, nil)
	api.call("worker", "PATCH", "/me/profile", map[string]any{"bio": strings.Repeat("字", 301)}, 400, nil)
	api.call("", "PATCH", "/me/profile", map[string]any{"bio": "guest"}, 401, nil)

	var user map[string]any
	user, err := get[map[string]any](t.Context(), api.App.DB, "users", "worker", false)
	if err != nil {
		t.Fatal(err)
	}
	user["phone"] = "PRIVATE-PHONE"
	user["phone_masked"] = "PRIVATE-PHONE-MASK"
	user["openid"] = "PRIVATE-OPENID"
	user["session_key"] = "PRIVATE-SESSION"
	if err = put(t.Context(), api.App.DB, "users", "worker", user); err != nil {
		t.Fatal(err)
	}
	for _, status := range []string{"pending", "paused", "rejected"} {
		service, e := get[Service](t.Context(), api.App.DB, "services", "delta-service-1", false)
		if e != nil {
			t.Fatal(e)
		}
		service.ID, service.Status, service.Title = "hidden-"+status, status, "PRIVATE-SERVICE-"+status
		if e = put(t.Context(), api.App.DB, "services", service.ID, service); e != nil {
			t.Fatal(e)
		}
	}
	var public struct {
		Profile  map[string]any `json:"profile"`
		Services []Service      `json:"services"`
		Clubs    []Club         `json:"clubs"`
	}
	api.call("", "GET", "/players/worker", nil, 200, &public)
	if len(public.Profile) != 6 || public.Profile["avatar"] != avatar || public.Profile["profile_cover"] != cover || len(public.Services) == 0 || len(public.Clubs) != 1 {
		t.Fatalf("公开资料字段或关联服务异常：%+v", public)
	}
	_, raw, _ := api.request("", "GET", "/players/worker", nil)
	if strings.Contains(string(raw), "PRIVATE-") || strings.Contains(string(raw), "phone") || strings.Contains(string(raw), "\"role\"") {
		t.Fatalf("公开主页泄漏未允许字段：%s", raw)
	}
	api.call("worker", "POST", "/services/delta-service-1/pause", map[string]any{}, 200, nil)
	api.call("", "GET", "/players/worker", nil, 200, &public)
	for _, service := range public.Services {
		if service.ID == "delta-service-1" || service.Status != "active" {
			t.Fatal("玩家主页包含不可售服务")
		}
	}

	var wg sync.WaitGroup
	results := make(chan int, 2)
	wg.Add(2)
	go func() {
		defer wg.Done()
		code, _, _ := api.request("worker", "PATCH", "/me/profile", map[string]any{"nickname": "并发保存昵称"})
		results <- code
	}()
	go func() {
		defer wg.Done()
		code, _, _ := api.request("worker", "PATCH", "/me", map[string]any{"provider_enabled": false})
		results <- code
	}()
	wg.Wait()
	close(results)
	for code := range results {
		if code != 200 {
			t.Fatalf("并发资料保存失败：%d", code)
		}
	}
	var current User
	api.call("worker", "GET", "/me", nil, 200, &current)
	if current.Nickname != "并发保存昵称" || current.ProviderEnabled || current.Avatar != avatar || current.ProfileCover != cover {
		t.Fatalf("并发更新覆盖了其他资料：%+v", current)
	}
	api.call("", "GET", "/players/worker", nil, 200, &public)
	if len(public.Services) != 0 {
		t.Fatal("关闭接单后不得在公开主页展示可售服务")
	}
	api.call("admin", "PATCH", "/admin/users/worker", map[string]any{"status": "frozen"}, 200, nil)
	api.call("", "GET", "/players/worker", nil, 404, nil)
	api.call("", "GET", "/players/missing", nil, 404, nil)
}

// TestCommunityMemberProjection 确保公开成员结构不受后续私有字段扩展影响。
func TestCommunityMemberProjection(t *testing.T) {
	member := ClubMember{UserID: "u", ClubID: "c", Status: "active", ApplicationNote: "私密申请", AppliedDepositCents: 30000, ReviewNote: "私密审核"}
	user := User{ID: "u", Nickname: "队友", Status: "active", ProviderEnabled: true}
	public := communityMemberView(member, user, false)
	raw, _ := json.Marshal(public)
	if len(public) != 6 || public["certified"] != true || strings.Contains(string(raw), "私密") {
		t.Fatalf("公开成员字段越界：%s", raw)
	}
	private := communityMemberView(member, user, true)
	if private["deposit_status"] != "unverified" {
		t.Fatal("未接入押金收款不得声称已支付")
	}
}
