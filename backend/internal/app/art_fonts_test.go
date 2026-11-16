// Package app 验证轮播与服务复用的艺术字体白名单和JSON保存契约。
package app

import (
	"encoding/json"
	"testing"
)

// TestCoverFontValidationAndRoundTrip 确保六种字体均可保存，旧数据兼容，任意外链被拒绝。
func TestCoverFontValidationAndRoundTrip(t *testing.T) {
	for _, font := range []string{"", "zcoolkuaile", "zcoolqingkehuangyou", "zcoolxiaowei", "mashanzheng", "longcang", "zhimangxing"} {
		labels, err := normalizeContentLabels([]CoverLabel{{Text: "限时优惠", Color: "#FFFFFF", Background: "transparent", FontFamily: font}})
		if err != nil || len(labels) != 1 || labels[0].FontFamily != font {
			t.Fatalf("字体保存失败 %s: %v", font, err)
		}
		raw, err := json.Marshal(Service{Labels: labels})
		if err != nil {
			t.Fatal(err)
		}
		var service Service
		if json.Unmarshal(raw, &service) != nil || service.Labels[0].FontFamily != font {
			t.Fatal("保存回读丢失字体")
		}
	}
	if _, err := normalizeContentLabels([]CoverLabel{{Text: "外链字体", Color: "#FFFFFF", Background: "transparent", FontFamily: "https://example.com/font.ttf"}}); err == nil {
		t.Fatal("不允许任意字体地址")
	}
}
