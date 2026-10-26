// Package app 提供固定游戏运营短文案，图文保持独立；未知游戏仅用安全通用文字，不拼接用户可编辑名称。
package app

// GameCarouselLabels 返回左上24和右下16的两条独立文字，亮色封面使用已验对比色，其余沿用白色和主题色。
func GameCarouselLabels(game Game) []CoverLabel {
	pairs := map[string][2]string{
		"honor":     {"峡谷开黑有队友", "双排陪练一起练好配合"},
		"peace":     {"战术组队一起出发", "刚枪跑图都能找到搭子"},
		"delta":     {"撤离路上有人同行", "跑图练枪任务一起协作"},
		"tft":       {"双人作战默契上场", "阵容运营陪你一起琢磨"},
		"valorant":  {"配合打好每个回合", "练枪战术一起耐心复盘"},
		"lol":       {"召唤师峡谷找队友", "对线团战都有陪练搭子"},
		"valorantm": {"掌上集结配合开局", "地图战术陪你逐局练习"},
		"cfm":       {"火线集结并肩开黑", "枪法地图一起边玩边练"},
		"arena":     {"暗区探索结伴同行", "熟悉地图练好撤离配合"},
		"codm":      {"战术小队随时集结", "枪法身法一起练好配合"},
		"identityv": {"庄园相约默契配合", "追逃练习一起复盘细节"},
		"narakam":   {"江湖结伴切磋同行", "连招身法都有耐心陪练"},
		"eggy":      {"蛋搭子一起闯关", "地图探索欢乐组队同行"},
	}
	pair, known := pairs[game.ID]
	if !known {
		pair = [2]string{"找个队友一起出发", "组队协作一起享受游戏"}
	}
	secondary := game.PrimaryColor
	if !hexColor.MatchString(secondary) {
		secondary = "#EBD29B"
	}
	primary := "#FFFFFF"
	contrast := map[string][2]string{
		"honor": {"#FFFFFF", "#FFF0D1"}, "peace": {"#102027", "#DDF7FF"},
		"delta": {"#FFFFFF", "#00E59B"}, "tft": {"#172019", "#FFF6DE"},
		"valorant": {"#FFFFFF", "#FFABB2"}, "eggy": {"#2B1D24", "#FFF0DC"},
	}
	if colors, ok := contrast[game.ID]; ok {
		primary, secondary = colors[0], colors[1]
	}
	return []CoverLabel{
		{ID: "game-caption-main", Text: pair[0], Color: primary, Background: "transparent", Position: "top-left", FontSize: 24},
		{ID: "game-caption-sub", Text: pair[1], Color: secondary, Background: "transparent", Position: "bottom-right", FontSize: 16},
	}
}
