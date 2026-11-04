package backend

import (
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"unicode"
)

// SVG 安全策略（mobile_logo 专用，网关与 app_service 保持一致）：
// 1) 根元素必须是 <svg（拒绝正文里仅包含 "<svg" 字样的纯文本）；
// 2) 上传时剥离 script/foreignObject/iframe/embed/object/use、on* 事件属性、javascript:/vbscript:/data:text/html URL；
// 3) 存盘时 Content-Type=image/svg+xml，并设置 Content-Disposition: attachment。

var (
	reSVGEventAttr        = regexp.MustCompile(`(?i)\s+on[a-z][a-z0-9_-]*\s*=\s*("[^"]*"|'[^']*'|[^\s>]+)`)
	reSVGDangerousURLAttr = regexp.MustCompile(`(?i)\s(href|xlink:href|src|action|formaction|data|poster)\s*=\s*(['"]?)\s*(javascript:|vbscript:|data\s*:\s*text\s*/\s*html)`)
	reSVGDangerousPair    []*regexp.Regexp
	reSVGDangerousSelf    []*regexp.Regexp
)

func init() {
	tags := []string{"script", "foreignobject", "iframe", "embed", "object", "link", "meta", "base", "use"}
	for _, tag := range tags {
		reSVGDangerousPair = append(reSVGDangerousPair, regexp.MustCompile(`(?is)<\s*`+tag+`\b[^>]*>.*?<\s*/\s*`+tag+`\s*>`))
		reSVGDangerousSelf = append(reSVGDangerousSelf, regexp.MustCompile(`(?is)<\s*`+tag+`\b[^>]*/?\s*>`))
	}
}

func stripSVGDangerousElements(s string) string {
	out := s
	for _, re := range reSVGDangerousPair {
		out = re.ReplaceAllString(out, "")
	}
	for _, re := range reSVGDangerousSelf {
		out = re.ReplaceAllString(out, "")
	}
	return out
}

func looksLikeSVG(data []byte) bool {
	head := data
	if len(head) > 8192 {
		head = head[:8192]
	}
	detected := http.DetectContentType(head)
	if !strings.HasPrefix(detected, "text/xml") &&
		!strings.HasPrefix(detected, "text/plain") &&
		!strings.HasPrefix(detected, "application/xml") {
		return false
	}
	s := strings.TrimSpace(strings.TrimPrefix(string(head), "\ufeff"))
	lower := strings.ToLower(s)
	for {
		lower = strings.TrimSpace(lower)
		switch {
		case strings.HasPrefix(lower, "<?xml"):
			end := strings.Index(lower, "?>")
			if end < 0 {
				return false
			}
			lower = lower[end+2:]
		case strings.HasPrefix(lower, "<!--"):
			end := strings.Index(lower, "-->")
			if end < 0 {
				return false
			}
			lower = lower[end+3:]
		case strings.HasPrefix(lower, "<!doctype"):
			end := strings.Index(lower, ">")
			if end < 0 {
				return false
			}
			lower = lower[end+1:]
		default:
			if !strings.HasPrefix(lower, "<svg") {
				return false
			}
			if len(lower) == 4 {
				return false
			}
			next := rune(lower[4])
			return unicode.IsSpace(next) || next == '>' || next == '/'
		}
	}
}

func sanitizeSVG(data []byte) ([]byte, error) {
	if !looksLikeSVG(data) {
		return nil, fmt.Errorf("不是有效的 SVG 根文档")
	}
	out := string(data)
	out = stripSVGDangerousElements(out)
	out = reSVGEventAttr.ReplaceAllString(out, "")
	out = reSVGDangerousURLAttr.ReplaceAllString(out, ` $1=$2#blocked`)
	if !looksLikeSVG([]byte(out)) {
		return nil, fmt.Errorf("清洗后不再是有效 SVG")
	}
	lower := strings.ToLower(out)
	for _, bad := range []string{"<script", "<foreignobject", "javascript:", "vbscript:", "onerror=", "onload="} {
		if strings.Contains(lower, bad) {
			return nil, fmt.Errorf("SVG 仍包含不安全内容")
		}
	}
	return []byte(out), nil
}
