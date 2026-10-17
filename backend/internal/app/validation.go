// Package app 对金额、文字、游戏主题和封面标签执行统一边界验证。
package app

import (
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"
)

var hexColor = regexp.MustCompile(`^#[0-9A-Fa-f]{6}$`)
var localCover = regexp.MustCompile(`^/static/covers/[a-z0-9_-]+\.(jpg|jpeg|png|webp)$`)
var gameIDPattern = regexp.MustCompile(`^[a-z][a-z0-9_-]{1,30}$`)

// length 判断去除首尾空白后的 Unicode 字符长度。
func length(s string, min, max int) bool {
	n := utf8.RuneCountInString(strings.TrimSpace(s))
	return n >= min && n <= max
}

// contains 判断枚举是否包含指定值。
func contains(items []string, value string) bool {
	for _, item := range items {
		if item == value {
			return true
		}
	}
	return false
}

// validCover 只允许本站静态封面或 HTTPS 公共图片地址。
func validCover(s string) bool {
	if localCover.MatchString(s) || regexp.MustCompile(`^/api/media/[a-f0-9]{32}$`).MatchString(s) {
		return true
	}
	u, err := url.Parse(s)
	return err == nil && u.Scheme == "https" && u.Hostname() != "" && u.User == nil && len(s) <= 500 && !strings.ContainsAny(s, "\r\n")
}

// validateGame 校验管理员可配置的完整游戏主题。
func validateGame(g *Game) error {
	if !gameIDPattern.MatchString(g.ID) || !length(g.Name, 2, 30) || !length(g.ShortName, 1, 8) || !length(g.Subtitle, 0, 80) {
		return fail(400, "游戏名称、简称或编号不符合要求")
	}
	for _, c := range []string{g.PrimaryColor, g.BgColor, g.SurfaceColor, g.TextColor} {
		if !hexColor.MatchString(c) {
			return fail(400, "主题颜色必须为六位十六进制色值")
		}
	}
	if !validCover(g.CoverURL) || len(g.Covers) < 1 || len(g.Covers) > 12 {
		return fail(400, "请配置 1 至 12 张合法封面")
	}
	for _, c := range g.Covers {
		if !validCover(c) {
			return fail(400, "封面地址不合法")
		}
	}
	if len(g.Categories) < 1 || len(g.Categories) > 12 {
		return fail(400, "请配置 1 至 12 个服务分类")
	}
	seen := map[string]bool{}
	for _, c := range g.Categories {
		if !length(c, 1, 16) || seen[c] {
			return fail(400, "分类名称不能为空、重复或超过 16 字")
		}
		seen[c] = true
	}
	if g.SortOrder < 0 || g.SortOrder > 999 {
		return fail(400, "排序范围为 0 至 999")
	}
	return nil
}

// validateService 验证卖家服务内容，不接受任意 CSS 或位置表达式。
func validateService(s *Service, g Game) error {
	if !length(s.Title, 2, 40) || !length(s.Description, 10, 2000) || !length(s.Unit, 1, 8) {
		return fail(400, "标题 2 至 40 字，服务说明 10 至 2000 字，单位 1 至 8 字")
	}
	if s.PriceCents < 100 || s.PriceCents > 10000000 {
		return fail(400, "价格范围为 1 至 100000 元")
	}
	if !contains(g.Categories, s.Category) {
		return fail(400, "请选择当前游戏的服务分类")
	}
	if !contains(g.Covers, s.CoverURL) {
		return fail(400, "请选择当前游戏配置的封面")
	}
	if len(s.Labels) > 6 {
		return fail(400, "最多添加 6 个封面文字")
	}
	positions := []string{"top-left", "top-center", "top-right", "middle-left", "middle-center", "middle-right", "bottom-left", "bottom-center", "bottom-right"}
	seen := map[string]bool{}
	for i := range s.Labels {
		l := &s.Labels[i]
		if l.ID == "" {
			l.ID = newID()
		}
		if seen[l.ID] || !length(l.ID, 1, 80) {
			return fail(400, "封面文字标识重复或无效")
		}
		seen[l.ID] = true
		if l.Position == "" {
			l.Position = "bottom-left"
		}
		if l.FontSize == 0 {
			l.FontSize = 16
		}
		if !length(l.Text, 1, 12) || !hexColor.MatchString(l.Color) || (!hexColor.MatchString(l.Background) && l.Background != "transparent") || !contains(positions, l.Position) || l.FontSize < 12 || l.FontSize > 24 {
			return fail(400, "封面文字、颜色、字号或位置不合法")
		}
	}
	if s.Labels == nil {
		s.Labels = []CoverLabel{}
	}
	if len(s.Tags) > 5 {
		return fail(400, "最多添加 5 个标签")
	}
	for _, t := range s.Tags {
		if !length(t, 1, 12) {
			return fail(400, "标签需为 1 至 12 字")
		}
	}
	if s.Tags == nil {
		s.Tags = []string{}
	}
	return nil
}

// pageLimit 统一列表分页上限，拒绝负数以保护服务稳定性。
func pageLimit(r *http.Request) (int, int) {
	limit := 100
	offset := 0
	if n, err := strconv.Atoi(r.URL.Query().Get("limit")); err == nil && n > 0 && n <= 100 {
		limit = n
	}
	if n, err := strconv.Atoi(r.URL.Query().Get("offset")); err == nil && n >= 0 && n <= 1000000 {
		offset = n
	}
	return limit, offset
}

// paginate 为已过滤的结果应用分页。
func paginate[T any](r *http.Request, items []T) []T {
	limit, offset := pageLimit(r)
	if offset >= len(items) {
		return []T{}
	}
	end := offset + limit
	if end > len(items) {
		end = len(items)
	}
	return items[offset:end]
}
