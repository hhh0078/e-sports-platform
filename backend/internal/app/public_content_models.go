// Package app 定义平台公共运营内容与安全跳转模型，不复用游戏服务封面池或私密设置。
package app

import "strings"

const publicContentID = "public-content-v1"

// ContentTarget 只允许平台公开服务、需求和文章，禁止客户端输入任意页面路径。
type ContentTarget struct {
	Kind string `json:"kind"`
	ID   string `json:"id"`
}

// ContentSlide 把图片与最多六条可配置文字保持为独立数据层。
type ContentSlide struct {
	ID       string         `json:"id"`
	Kind     string         `json:"kind"`
	ImageURL string         `json:"image_url"`
	Labels   []CoverLabel   `json:"labels"`
	Target   *ContentTarget `json:"target"`
}

// ContentAnnouncement 表示唯一首页公告，版本由服务端在内容变化时递增。
type ContentAnnouncement struct {
	ID        string         `json:"id"`
	Version   int64          `json:"version"`
	Enabled   bool           `json:"enabled"`
	Frequency string         `json:"frequency"`
	Mode      string         `json:"mode"`
	ImageURL  string         `json:"image_url"`
	Title     string         `json:"title"`
	Body      string         `json:"body"`
	Target    *ContentTarget `json:"target"`
}

// ContentPageTip 是单页提示及可选已发布文章引用，空文章编号表示无跳转。
type ContentPageTip struct {
	Text      string `json:"text"`
	ArticleID string `json:"article_id"`
}

// ContentPageTips 保留四个主页面独立提示，内容是保留换行的纯文字。
type ContentPageTips struct {
	Player ContentPageTip `json:"player"`
	Worker ContentPageTip `json:"worker"`
	Orders ContentPageTip `json:"orders"`
	Me     ContentPageTip `json:"me"`
}

// PublicContentSettings 是固定settings记录，不含身份、支付或AI秘密。
type PublicContentSettings struct {
	Revision      int64                     `json:"revision"`
	GlobalSlides  []ContentSlide            `json:"global_slides"`
	GameSlides    map[string][]ContentSlide `json:"game_slides"`
	Announcement  ContentAnnouncement       `json:"announcement"`
	PageTips      ContentPageTips           `json:"page_tips"`
	ServiceNotice string                    `json:"service_notice"`
	RequestNotice string                    `json:"request_notice"`
	UpdatedAt     string                    `json:"updated_at"`
}

// PublicContentDTO 只向访客提供当前游戏已展开的轮播和公共信息。
type PublicContentDTO struct {
	Revision      int64               `json:"revision"`
	GameID        string              `json:"game_id"`
	Slides        []ContentSlide      `json:"slides"`
	Announcement  ContentAnnouncement `json:"announcement"`
	PageTips      ContentPageTips     `json:"page_tips"`
	ServiceNotice string              `json:"service_notice"`
	RequestNotice string              `json:"request_notice"`
}

// defaultPublicContent 保留当前游戏展示槽，首次读取不写入数据库。
func defaultPublicContent() PublicContentSettings {
	return PublicContentSettings{GlobalSlides: []ContentSlide{{ID: "selected-game", Kind: "game_slot", Labels: []CoverLabel{}}}, GameSlides: map[string][]ContentSlide{}, Announcement: ContentAnnouncement{ID: "home", Frequency: "once", Mode: "text"}}
}

// normalizeContentLabels 复用服务封面文字的全部尺寸、位置与颜色校验，不创建虚拟服务记录。
func normalizeContentLabels(labels []CoverLabel) ([]CoverLabel, error) {
	if len(labels) > 6 {
		return nil, fail(400, "最多添加6条轮播文字")
	}
	game := defaultGames()[0]
	s := Service{Title: "公共信息", Description: "校验独立轮播文字图层", Unit: "单", PriceCents: 100, Category: game.Categories[0], CoverURL: game.Covers[0], Labels: append([]CoverLabel{}, labels...)}
	if err := validateService(&s, game); err != nil {
		return nil, err
	}
	return s.Labels, nil
}

// validContentTarget 拒绝空编号、演示资源及未授权跳转种类。
func validContentTarget(target *ContentTarget) bool {
	return target == nil || contains([]string{"service", "request", "article"}, target.Kind) && length(target.ID, 1, 128) && !strings.HasPrefix(target.ID, "demo_") && !strings.ContainsAny(target.ID, "/\\?#\r\n\t ")
}
