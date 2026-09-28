package contract

import (
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// parseFragment 把 HTML 片段解析为顶层节点列表（div 上下文，不补全文档骨架）。
// 输入按 UTF-8 文本处理，解析错误（理论上合法输入不会）交调用方兜底。
func parseFragment(src string) ([]*html.Node, error) {
	context := &html.Node{
		Type:     html.ElementNode,
		Data:     "div",
		DataAtom: atom.Div,
	}
	return html.ParseFragment(strings.NewReader(src), context)
}

// wrapSequence 把一组节点串成兄弟链（供 unwrap 场景返回多个节点使用）。
func wrapSequence(nodes []*html.Node) *html.Node {
	if len(nodes) == 0 {
		return nil
	}
	for i, node := range nodes {
		node.Parent = nil
		node.PrevSibling = nil
		node.NextSibling = nil
		if i > 0 {
			node.PrevSibling = nodes[i-1]
			nodes[i-1].NextSibling = node
		}
	}
	return nodes[0]
}
