// Package app 为公共运营文字、独立轮播图层和Markdown文章登记固定字段审核，不允许客户端指定审核路径。
package app

import (
	"context"
	"fmt"
	"strconv"
	"strings"
)

// registeredPublicContentReviewFields 从已知公共结构抽取内容，稳定字段编码兼容排序和迟到审核保护。
func registeredPublicContentReviewFields(table, id string, value map[string]any) []reviewField {
	out := []reviewField{}
	textField := func(code, label, text string, path ...string) {
		if strings.TrimSpace(text) != "" {
			out = append(out, reviewField{Code: code, Label: label, Kind: "text", Value: text, Path: path, Replacement: rejectedContentText})
		}
	}
	imageField := func(code, label, raw string, path ...string) {
		if media := reviewMediaID(raw); media != "" {
			out = append(out, reviewField{Code: code, Label: label, Kind: "image", Value: raw, MediaID: media, Path: path, Replacement: rejectedImageURL})
		}
	}
	if table == "articles" {
		for _, field := range [][2]string{{"title", "文章标题"}, {"summary", "文章摘要"}, {"body_markdown", "文章正文"}} {
			textField(field[0], field[1], reviewString(value, field[0]), field[0])
		}
		imageField("cover_url", "文章封面", reviewString(value, "cover_url"), "cover_url")
		if images, ok := value["body_images"].([]any); ok {
			for index, raw := range images {
				if image, ok := raw.(string); ok {
					imageField(fmt.Sprintf("body_images.%d", index), "文章正文图片", image, "body_images", strconv.Itoa(index))
				}
			}
		}
	}
	if table != "settings" || id != publicContentID {
		return out
	}
	for _, field := range [][2]string{{"service_notice", "服务注意事项"}, {"request_notice", "需求注意事项"}} {
		textField(field[0], field[1], reviewString(value, field[0]), field[0])
	}
	if tips, ok := value["page_tips"].(map[string]any); ok {
		for _, key := range []string{"player", "worker", "orders", "me"} {
			if tip, ok := tips[key].(map[string]any); ok {
				textField("page_tips."+key+".text", "页面顶部提示", reviewString(tip, "text"), "page_tips", key, "text")
			}
		}
	}
	if notice, ok := value["announcement"].(map[string]any); ok {
		textField("announcement.title", "公告标题", reviewString(notice, "title"), "announcement", "title")
		textField("announcement.body", "公告正文", reviewString(notice, "body"), "announcement", "body")
		imageField("announcement.image_url", "公告图片", reviewString(notice, "image_url"), "announcement", "image_url")
	}
	addSlides := func(slides []any, prefix string, base []string) {
		for index, raw := range slides {
			slide, ok := raw.(map[string]any)
			if !ok || reviewString(slide, "kind") != "image" {
				continue
			}
			path := append(append([]string{}, base...), strconv.Itoa(index))
			code := prefix + "." + reviewHash(reviewString(slide, "id"))
			imageField(code+".image_url", "运营轮播图片", reviewString(slide, "image_url"), append(path, "image_url")...)
			if labels, ok := slide["labels"].([]any); ok {
				for labelIndex, rawLabel := range labels {
					if label, ok := rawLabel.(map[string]any); ok {
						textField(code+".label."+reviewHash(reviewString(label, "id")), "轮播独立文字", reviewString(label, "text"), append(append([]string{}, path...), "labels", strconv.Itoa(labelIndex), "text")...)
					}
				}
			}
		}
	}
	if slides, ok := value["global_slides"].([]any); ok {
		addSlides(slides, "global_slide", []string{"global_slides"})
	}
	if games, ok := value["game_slides"].(map[string]any); ok {
		for gameID, raw := range games {
			if slides, ok := raw.([]any); ok {
				addSlides(slides, "game_slide."+gameID, []string{"game_slides", gameID})
			}
		}
	}
	return out
}

// migratePublicContent 为公开文章按状态排序建立索引，表与列中文注释由统一迁移白名单创建。
func migratePublicContent(ctx context.Context, q querier) error {
	_, err := q.Exec(ctx, `CREATE INDEX IF NOT EXISTS articles_status_created_idx ON articles((data->>'status'),(data->>'created_at'),id);
COMMENT ON INDEX articles_status_created_idx IS '文章发布状态、创建时间与稳定分页排序';`)
	return err
}
