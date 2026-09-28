package contract

import (
	"encoding/json"
	"regexp"
	"strings"
)

// Field 是模板字段定义（也随合同快照存储）。key 是正文占位符的稳定标识，
// 生成后不可改（正文里的 data-field 靠它对应）；label 是展示名。
type Field struct {
	Key         string   `json:"key"`
	Label       string   `json:"label"`
	Type        string   `json:"type"`
	Required    bool     `json:"required"`
	Placeholder string   `json:"placeholder,omitempty"`
	Options     []string `json:"options,omitempty"`
	Width       string   `json:"width,omitempty"`
}

// 字段类型与填写宽度枚举。
var fieldTypes = map[string]bool{
	"text": true, "textarea": true, "number": true, "date": true, "select": true,
}

var fieldKeyRe = regexp.MustCompile(`[^A-Za-z0-9_-]+`)

// parseFields 反序列化字段定义 JSON；空串视为无字段。
func parseFields(raw string) ([]Field, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	var fields []Field
	if err := json.Unmarshal([]byte(raw), &fields); err != nil {
		return nil, err
	}
	return fields, nil
}

// marshalFields 序列化字段定义；nil 存空串。
func marshalFields(fields []Field) (string, error) {
	if len(fields) == 0 {
		return "", nil
	}
	data, err := json.Marshal(fields)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// normalizeFields 校验并规范化字段定义：
//   - key 缺省自动补 field_N，非法字符替换为下划线，模板内必须唯一；
//   - label 必填；type 必须在枚举内；select 至少一个选项；width 只认 full/half。
func normalizeFields(fields []Field) ([]Field, error) {
	normalized := make([]Field, 0, len(fields))
	used := map[string]bool{}
	next := 1
	for _, f := range fields {
		label := strings.TrimSpace(f.Label)
		if label == "" {
			return nil, errInvalidFields
		}
		typ := strings.TrimSpace(f.Type)
		if typ == "" {
			typ = "text"
		}
		if !fieldTypes[typ] {
			return nil, errInvalidFields
		}

		key := fieldKeyRe.ReplaceAllString(strings.TrimSpace(f.Key), "_")
		for key == "" || used[key] {
			key = "field_" + itoa(next)
			next++
		}
		used[key] = true

		var options []string
		if typ == "select" {
			for _, opt := range f.Options {
				if opt = strings.TrimSpace(opt); opt != "" {
					options = append(options, opt)
				}
			}
			if len(options) == 0 {
				return nil, errInvalidFields
			}
		}

		width := strings.TrimSpace(f.Width)
		if width != "half" {
			width = "full"
		}

		normalized = append(normalized, Field{
			Key:         key,
			Label:       label,
			Type:        typ,
			Required:    f.Required,
			Placeholder: strings.TrimSpace(f.Placeholder),
			Options:     options,
			Width:       width,
		})
	}
	return normalized, nil
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var digits []byte
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return string(digits)
}
