package contract

import (
	"errors"
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// errInvalidFields 表示字段定义不合法（label/type/options/唯一性等）。
var errInvalidFields = errors.New("字段定义不合法")

// 富文本白名单清洗。正文来自 contenteditable 编辑器（本质是用户输入的 HTML），
// 存库前必须过滤掉脚本与危险属性，渲染进 v-html 才安全。
//
// 规则：
//   - 白名单外的标签降级为其子节点（unwrap）；script/style/iframe 等连同内容整体丢弃；
//   - 属性按标签白名单保留：span 的 data-field/class、a 的 href（限安全协议）、
//     少量排版标签的 style（逐条声明校验）；
//   - 注释、事件属性一律剔除。

var dropWithContent = map[string]bool{
	"script": true, "style": true, "iframe": true, "object": true,
	"embed": true, "noscript": true, "template": true, "svg": true, "math": true,
}

var allowedElements = map[string]bool{
	"p": true, "div": true, "h1": true, "h2": true, "h3": true, "h4": true,
	"h5": true, "h6": true, "ul": true, "ol": true, "li": true, "blockquote": true,
	"br": true, "hr": true, "table": true, "thead": true, "tbody": true,
	"tfoot": true, "tr": true, "th": true, "td": true,
	"span": true, "strong": true, "b": true, "em": true, "i": true, "u": true,
	"s": true, "sub": true, "sup": true, "a": true,
}

// 允许出现在 style 声明里的属性与值的粗校验。
var allowedStyleProps = map[string]bool{
	"text-align": true, "text-indent": true, "font-weight": true,
	"font-style": true, "text-decoration": true, "color": true,
	"background-color": true,
}

var classAllowed = map[string]bool{"ct-field": true, "ct-value": true, "ct-blank": true}

// SanitizeHTML 清洗富文本 HTML，返回白名单内的安全 HTML。
func SanitizeHTML(src string) (string, error) {
	nodes, err := parseFragment(src)
	if err != nil {
		return "", err
	}

	cleaned := make([]*html.Node, 0, len(nodes))
	for _, node := range nodes {
		if out := cleanNode(node); out != nil {
			cleaned = append(cleaned, out)
		}
	}

	var sb strings.Builder
	for _, node := range cleaned {
		if err := html.Render(&sb, node); err != nil {
			return "", err
		}
	}
	return sb.String(), nil
}

// cleanNode 递归清洗一个顶层节点，返回净化后的新树（输入节点不被修改）。
func cleanNode(node *html.Node) *html.Node {
	switch node.Type {
	case html.TextNode:
		return &html.Node{Type: html.TextNode, Data: node.Data}
	case html.ElementNode:
	default:
		return nil // 注释、doctype 等一律丢弃
	}

	tag := strings.ToLower(node.Data)
	if dropWithContent[tag] {
		return nil
	}
	if node.DataAtom != 0 && node.DataAtom != atom.Lookup([]byte(tag)) {
		return nil
	}

	if !allowedElements[tag] {
		// 白名单外但无危险的标签：降级为其子节点。
		var children []*html.Node
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			if out := cleanNode(child); out != nil {
				children = append(children, out)
			}
		}
		return wrapSequence(children)
	}

	out := &html.Node{Type: html.ElementNode, Data: tag, DataAtom: node.DataAtom}
	for _, attr := range node.Attr {
		if kept, ok := cleanAttr(tag, attr); ok {
			out.Attr = append(out.Attr, kept)
		}
	}
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		if cleaned := cleanNode(child); cleaned != nil {
			out.AppendChild(cleaned)
		}
	}
	if tag == "a" {
		out.Attr = append(out.Attr,
			html.Attribute{Key: "rel", Val: "noopener noreferrer nofollow"},
			html.Attribute{Key: "target", Val: "_blank"},
		)
	}
	return out
}

func cleanAttr(tag string, attr html.Attribute) (html.Attribute, bool) {
	key := strings.ToLower(attr.Key)
	switch key {
	case "data-field":
		if tag != "span" || !validFieldKey(attr.Val) {
			return html.Attribute{}, false
		}
		return attr, true
	case "class":
		if tag != "span" || !classAllowed[attr.Val] {
			return html.Attribute{}, false
		}
		return attr, true
	case "href":
		if tag != "a" || !safeURL(attr.Val) {
			return html.Attribute{}, false
		}
		return attr, true
	case "style":
		val := cleanStyle(attr.Val)
		if val == "" {
			return html.Attribute{}, false
		}
		return html.Attribute{Key: "style", Val: val}, true
	default:
		return html.Attribute{}, false
	}
}

// cleanStyle 逐条过滤 style 声明，只保留白名单属性且值不含危险片段的。
func cleanStyle(val string) string {
	var kept []string
	for _, decl := range strings.Split(val, ";") {
		decl = strings.TrimSpace(decl)
		if decl == "" {
			continue
		}
		parts := strings.SplitN(decl, ":", 2)
		if len(parts) != 2 {
			continue
		}
		prop := strings.ToLower(strings.TrimSpace(parts[0]))
		value := strings.TrimSpace(parts[1])
		if !allowedStyleProps[prop] {
			continue
		}
		lower := strings.ToLower(value)
		if strings.Contains(lower, "url(") || strings.Contains(lower, "expression") ||
			strings.Contains(lower, "javascript:") || strings.Contains(lower, "@") {
			continue
		}
		kept = append(kept, prop+": "+value)
	}
	return strings.Join(kept, "; ")
}

// safeURL 只放行 http/https/mailto 与页内锚点，拦截 javascript:/data: 等。
func safeURL(val string) bool {
	v := strings.TrimSpace(strings.ToLower(val))
	if strings.HasPrefix(v, "#") {
		return true
	}
	for _, prefix := range []string{"http://", "https://", "mailto:"} {
		if strings.HasPrefix(v, prefix) {
			return true
		}
	}
	return false
}

func validFieldKey(key string) bool {
	if key == "" || len(key) > 64 {
		return false
	}
	for _, r := range key {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '_', r == '-':
		default:
			return false
		}
	}
	return true
}
