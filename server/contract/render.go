package contract

import (
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// 渲染把正文里的字段占位符替换为填写值：
//
//	<span class="ct-field" data-field="key" contenteditable="false">【标签】</span>
//	  → 有值：<span class="ct-value">值</span>（文本转义）
//	  → 无值/未知字段：<span class="ct-blank"></span>（打印样式渲染为下划线空位）
//
// 输入必须是已清洗的正文（模板保存与合同快照时都过 SanitizeHTML），输出直接
// 进入打印页 v-html。前端预览遵循同一规则，权威快照以本函数结果为准。

// RenderContent 按字段值渲染正文，返回最终 HTML。
func RenderContent(contentHTML string, fields []Field, values map[string]string) string {
	nodes, err := parseFragment(contentHTML)
	if err != nil {
		// 解析失败兜底：原样返回（内容本身已经过清洗）。
		return contentHTML
	}

	known := map[string]bool{}
	for _, f := range fields {
		known[f.Key] = true
	}

	rendered := make([]*html.Node, 0, len(nodes))
	for _, node := range nodes {
		rendered = append(rendered, renderNode(node, known, values))
	}

	var sb strings.Builder
	for _, node := range rendered {
		if err := html.Render(&sb, node); err != nil {
			return contentHTML
		}
	}
	return sb.String()
}

// renderNode 递归渲染：命中占位符 span 时替换为值/空位节点，其余原样重建。
func renderNode(node *html.Node, known map[string]bool, values map[string]string) *html.Node {
	if node.Type == html.TextNode {
		return &html.Node{Type: html.TextNode, Data: node.Data}
	}
	if node.Type != html.ElementNode {
		return nil
	}
	if node.DataAtom == atom.Span {
		for _, attr := range node.Attr {
			if attr.Key == "data-field" {
				return replacementSpan(attr.Val, known, values)
			}
		}
	}

	out := &html.Node{Type: html.ElementNode, Data: node.Data, DataAtom: node.DataAtom, Attr: node.Attr}
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		if rendered := renderNode(child, known, values); rendered != nil {
			out.AppendChild(rendered)
		}
	}
	return out
}

func replacementSpan(key string, known map[string]bool, values map[string]string) *html.Node {
	if !known[key] {
		return blankSpan()
	}
	value := strings.TrimSpace(values[key])
	if value == "" {
		return blankSpan()
	}
	span := &html.Node{
		Type:     html.ElementNode,
		Data:     "span",
		DataAtom: atom.Span,
		Attr:     []html.Attribute{{Key: "class", Val: "ct-value"}},
	}
	span.AppendChild(&html.Node{Type: html.TextNode, Data: value})
	return span
}

func blankSpan() *html.Node {
	return &html.Node{
		Type:     html.ElementNode,
		Data:     "span",
		DataAtom: atom.Span,
		Attr:     []html.Attribute{{Key: "class", Val: "ct-blank"}},
	}
}
