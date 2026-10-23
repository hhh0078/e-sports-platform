// Package assets 内嵌审核失败状态图，不依赖外网或服务器中文字体。
package assets

import _ "embed"

// ModerationRejectedPNG 为公开媒体撤销后返回的统一PNG占位图。
//
//go:embed moderation-rejected.png
var ModerationRejectedPNG []byte
