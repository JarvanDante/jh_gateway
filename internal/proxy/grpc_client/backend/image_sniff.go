package backend

import (
	"net/http"
	"path/filepath"
	"strings"
)

var sniffedImageExt = map[string]string{
	"image/jpeg": ".jpg",
	"image/png":  ".png",
	"image/gif":  ".gif",
	"image/webp": ".webp",
}

// sniffImageType 根据前 512 字节检测真实图片类型，返回 mime 与对应扩展名。
// 仅接受 jpeg/png/gif/webp；客户端声明为 image/svg+xml 且根元素确为 SVG 时保持 svg 支持
// （是否允许由 app_service 按 upload code 决定；网关在转发前会 sanitize）。
func sniffImageType(data []byte, declaredType string) (mimeType, ext string, ok bool) {
	head := data
	if len(head) > 512 {
		head = head[:512]
	}
	detected := http.DetectContentType(head)
	if e, found := sniffedImageExt[detected]; found {
		return detected, e, true
	}
	if strings.EqualFold(strings.TrimSpace(declaredType), "image/svg+xml") && looksLikeSVG(data) {
		return "image/svg+xml", ".svg", true
	}
	return detected, "", false
}

// replaceExt 去掉客户端文件名中的目录与扩展名，换成检测出的扩展名
func replaceExt(fileName, ext string) string {
	base := filepath.Base(strings.ReplaceAll(fileName, "\\", "/"))
	if i := strings.LastIndex(base, "."); i > 0 {
		base = base[:i]
	}
	if base == "" || base == "." || base == "/" {
		base = "image"
	}
	return base + ext
}
