// Package assets 验证分享只读素材仅能访问真实嵌入图片，不允许目录、参数或路径穿越。
package assets

import "testing"

// TestShareCoverBoundaries 验证官方图片存在且非法路径不会被分享素材读取。
func TestShareCoverBoundaries(t *testing.T) {
	if data, err := PublicCoverData("/static/covers/official/delta-1.jpg"); err != nil || len(data) == 0 {
		t.Fatal("应可读取真实内置官方封面")
	}
	for _, raw := range []string{"", "/static/covers/", "/static/covers/../assets.go", "/static/covers/official/../delta-1.jpg", "/static/covers/official/delta-1.jpg?x=1", "/static/covers/official/delta-1.jpg#x", "/static/covers/official/missing.jpg", "/static/covers/avatars/LICENSE", "https://example.com/static/covers/official/delta-1.jpg"} {
		if _, err := PublicCoverData(raw); err == nil {
			t.Fatalf("不允许读取的路径: %q", raw)
		}
	}
}
