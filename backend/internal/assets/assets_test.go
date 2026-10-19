// Package assets 验证嵌入图片的读取、类型以及目录访问边界。
package assets

import (
 "net/http/httptest"
 "testing"
)

// TestEmbeddedCover 验证游戏图片可读取而目录和非图片文件不会公开。
func TestEmbeddedCover(t *testing.T) {
 for _, scenario := range []struct{ path string; status int }{{"/static/covers/official/delta-icon.jpg",200},{"/static/covers/",404},{"/static/covers/../assets.go",404}} {
  response := httptest.NewRecorder()
  Handler().ServeHTTP(response,httptest.NewRequest("GET",scenario.path,nil))
  if response.Code != scenario.status { t.Fatalf("%s: %d",scenario.path,response.Code) }
  if scenario.status == 200 && (response.Body.Len()<100 || response.Header().Get("Content-Type")!="image/jpeg") { t.Fatal("图片内容或类型错误") }
 }
}
