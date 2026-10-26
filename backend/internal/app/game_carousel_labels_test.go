// Package app 验证十三款固定游戏短文案和未知游戏安全回退，公开默认图层不写配置或拼接未审名称。
package app

import (
	"reflect"
	"strings"
	"testing"
	"unicode/utf8"
)

// TestGameCarouselLabelsContract 验证十三套唯一短文案、两种字号、纯文字位置与未知名称隔离。
func TestGameCarouselLabelsContract(t *testing.T) {
	seen := map[string]bool{}
	for _, game := range defaultGames() {
		labels := GameCarouselLabels(game)
		if len(labels) != 2 || labels[0].FontSize != 24 || labels[1].FontSize != 16 || labels[0].Position != "top-left" || labels[1].Position != "bottom-right" || !hexColor.MatchString(labels[0].Color) || !hexColor.MatchString(labels[1].Color) {
			t.Fatal("默认图层规格错误", game.ID)
		}
		if game.ID == "delta" && (labels[0].Color != "#FFFFFF" || labels[1].Color != "#00E59B") {
			t.Fatal("三角洲人物背景未应用模拟器复核后的白色标题和绿色副文案")
		}
		for _, label := range labels {
			if utf8.RuneCountInString(label.Text) > 12 || strings.TrimSpace(label.Text) == "" || label.Background != "transparent" {
				t.Fatal("短文案长度或纯文字边界错误", game.ID)
			}
		}
		key := labels[0].Text + "/" + labels[1].Text
		if seen[key] {
			t.Fatal("不同游戏文案意外复用", game.ID)
		}
		seen[key] = true
		labels[0].Text = "外部修改"
		if GameCarouselLabels(game)[0].Text == "外部修改" {
			t.Fatal("调用方修改了共享默认值")
		}
	}
	if len(seen) != 13 {
		t.Fatal("默认游戏目录不是十三款")
	}
	unknown := GameCarouselLabels(Game{ID: "future-game", Name: "<script>未审核名称</script>", PrimaryColor: "rgba(0,0,0,0)"})
	if strings.Contains(unknown[0].Text+unknown[1].Text, "未审核") || unknown[1].Color != "#EBD29B" {
		t.Fatal("未知游戏拼接了未审内容或接受非法颜色")
	}
}

// TestIntegrationGameCarouselFallback13 验证十三款游戏公开回退都有两段字，未知游戏安全，已有显式无字轮播仍保持原样。
func TestIntegrationGameCarouselFallback13(t *testing.T) {
	api := newTestAPI(t)
	before := contentFinanceSnapshot(t, api)
	for _, game := range defaultGames() {
		var shown PublicContentDTO
		api.call("", "GET", "/public-content?game_id="+game.ID, nil, 200, &shown)
		if len(shown.Slides) != 1 || shown.Slides[0].ID != "game-default-"+game.ID || shown.Slides[0].ImageURL != game.CoverURL || !reflect.DeepEqual(shown.Slides[0].Labels, GameCarouselLabels(game)) {
			t.Fatal("默认游戏回退图文不匹配", game.ID)
		}
	}
	var count int
	if err := api.App.DB.QueryRow(t.Context(), "SELECT count(*) FROM settings WHERE id=$1", publicContentID).Scan(&count); err != nil || count != 0 {
		t.Fatal("公开回退写入了配置", err)
	}
	game := defaultGames()[0]
	game.ID = "future-safe"
	game.Name = "未经审核的游戏名称"
	if err := putRaw(t.Context(), api.App.DB, "games", game.ID, game); err != nil {
		t.Fatal(err)
	}
	var shown PublicContentDTO
	api.call("", "GET", "/public-content?game_id=future-safe", nil, 200, &shown)
	if strings.Contains(shown.Slides[0].Labels[0].Text+shown.Slides[0].Labels[1].Text, game.Name) {
		t.Fatal("公开未知游戏文案拼接名称")
	}
	cfg := defaultPublicContent()
	cfg.GameSlides["delta"] = []ContentSlide{{ID: "explicit-empty", Kind: "image", ImageURL: defaultGames()[0].CoverURL, Labels: []CoverLabel{}}}
	if err := putRaw(t.Context(), api.App.DB, "settings", publicContentID, cfg); err != nil {
		t.Fatal(err)
	}
	api.call("", "GET", "/public-content?game_id=delta", nil, 200, &shown)
	if shown.Slides[0].ID != "explicit-empty" || len(shown.Slides[0].Labels) != 0 {
		t.Fatal("自动覆盖了明确配置的无字轮播")
	}
	if !reflect.DeepEqual(before, contentFinanceSnapshot(t, api)) {
		t.Fatal("公开回退改变了交易资金")
	}
}
