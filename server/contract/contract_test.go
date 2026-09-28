package contract

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"smallgo/server/database"
)

// newTestAPI 建一个只挂 contract 路由的测试服务：请求带 X-Test-User 头即可
// 模拟不同登录用户（框架的鉴权中间件在生产装配时已覆盖，这里只测业务逻辑）。
func newTestAPI(t *testing.T) (*gin.Engine, *gorm.DB) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "contract-test.db")), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&ContractTemplate{}, &Contract{}, &database.AuditLog{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	r := gin.New()
	api := r.Group("/api")
	api.Use(func(c *gin.Context) {
		uid := atoiDefault(c.GetHeader("X-Test-User"), 1)
		c.Set("userID", uint(uid))
		c.Set("username", fmt.Sprintf("user-%d", uid))
		c.Next()
	})
	setupRoutes(api, db)
	return r, db
}

func atoiDefault(s string, def int) int {
	n := 0
	for _, r := range s {
		if r < '0' || r > '9' {
			return def
		}
		n = n*10 + int(r-'0')
	}
	if s == "" {
		return def
	}
	return n
}

func doJSON(t *testing.T, r *gin.Engine, method, path string, user int, body interface{}) (*httptest.ResponseRecorder, map[string]interface{}) {
	t.Helper()
	var reader *bytes.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal body: %v", err)
		}
		reader = bytes.NewReader(data)
	} else {
		reader = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Test-User", fmt.Sprintf("%d", user))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	var parsed map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &parsed)
	return w, parsed
}

func dataMap(t *testing.T, parsed map[string]interface{}) map[string]interface{} {
	t.Helper()
	data, ok := parsed["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected data object, got: %v", parsed)
	}
	return data
}

func createTemplateForTest(t *testing.T, r *gin.Engine, user int, name string) map[string]interface{} {
	t.Helper()
	w, parsed := doJSON(t, r, http.MethodPost, "/api/contract/templates", user, gin.H{
		"name": name,
		"content": `<p>甲方 <span class="ct-field" data-field="party_a" contenteditable="false">【甲方名称】</span>，` +
			`乙方 <span class="ct-field" data-field="party_b" contenteditable="false">【乙方名称】</span>。</p>`,
		"fields": []gin.H{
			{"key": "party_a", "label": "甲方名称", "type": "text", "required": true},
			{"key": "party_b", "label": "乙方名称", "type": "text"},
		},
	})
	if w.Code != http.StatusOK {
		t.Fatalf("create template: %d %s", w.Code, w.Body.String())
	}
	return dataMap(t, parsed)
}

func TestNormalizeFields(t *testing.T) {
	fields, err := normalizeFields([]Field{
		{Label: "甲方", Type: "text"},
		{Label: "选项", Type: "select", Options: []string{" A ", "", "B"}},
		{Label: "半行", Type: "number", Key: "半 行!", Width: "half"},
		{Label: "日期", Type: "date", Key: "field_1"},
	})
	if err != nil {
		t.Fatalf("normalizeFields: %v", err)
	}
	if fields[0].Key != "field_1" {
		t.Errorf("auto key = %q, want field_1", fields[0].Key)
	}
	if fields[1].Key == "" || fields[1].Key == fields[0].Key {
		t.Errorf("options key not auto-filled/deduped: %q", fields[1].Key)
	}
	opts := fields[1].Options
	if len(opts) != 2 || opts[0] != "A" || opts[1] != "B" {
		t.Errorf("options not trimmed: %v", opts)
	}
	if fields[2].Key != "_" {
		t.Errorf("invalid key chars not replaced: %q", fields[2].Key)
	}
	if fields[3].Key != "field_1" && fields[3].Key != fields[0].Key {
		// 第 4 个字段与第 1 个撞 key，必须被重排
		t.Logf("deduped key: %q", fields[3].Key)
	}
	if fields[3].Key == fields[0].Key {
		t.Errorf("duplicate key not deduped: %q", fields[3].Key)
	}

	if _, err := normalizeFields([]Field{{Label: "", Type: "text"}}); err == nil {
		t.Error("empty label should fail")
	}
	if _, err := normalizeFields([]Field{{Label: "x", Type: "dict"}}); err == nil {
		t.Error("unknown type should fail")
	}
	if _, err := normalizeFields([]Field{{Label: "x", Type: "select"}}); err == nil {
		t.Error("select without options should fail")
	}
}

func TestRenderContent(t *testing.T) {
	fields := []Field{{Key: "a", Label: "甲"}, {Key: "b", Label: "乙"}}
	content := `<p><span class="ct-field" data-field="a" contenteditable="false">【甲】</span>` +
		`与<span class="ct-field" data-field="b" contenteditable="false">【乙】</span>` +
		`及<span class="ct-field" data-field="gone" contenteditable="false">【已删】</span></p>`

	out := RenderContent(content, fields, map[string]string{"a": "张三<b>", "b": " "})
	if !strings.Contains(out, `<span class="ct-value">张三&lt;b&gt;</span>`) {
		t.Errorf("value not escaped/substituted: %s", out)
	}
	if !strings.Contains(out, `<span class="ct-blank"></span>`) {
		t.Errorf("blank value should render blank span: %s", out)
	}
	if strings.Count(out, "ct-blank") != 2 {
		t.Errorf("expected 2 blanks (empty + unknown field): %s", out)
	}
	if strings.Contains(out, "ct-field") {
		t.Errorf("placeholder span should be replaced: %s", out)
	}
}

func TestSanitizeHTML(t *testing.T) {
	src := `<p onclick="evil()">ok</p><script>alert(1)</script>` +
		`<p style="text-align:center;position:fixed;color:red">样式</p>` +
		`<a href="javascript:x()">坏链接</a><a href="https://ok.example/x">好链接</a>` +
		`<span class="ct-field" data-field="party_a" contenteditable="false">【甲方】</span>` +
		`<font size="5">降级</font>`

	out, err := SanitizeHTML(src)
	if err != nil {
		t.Fatalf("SanitizeHTML: %v", err)
	}
	if strings.Contains(out, "script") || strings.Contains(out, "alert") {
		t.Errorf("script not dropped: %s", out)
	}
	if strings.Contains(out, "onclick") {
		t.Errorf("event attribute kept: %s", out)
	}
	if strings.Contains(out, "position") {
		t.Errorf("style prop outside whitelist kept: %s", out)
	}
	if !strings.Contains(out, "text-align") || !strings.Contains(out, "color") {
		t.Errorf("whitelisted style props lost: %s", out)
	}
	if strings.Contains(out, "javascript") {
		t.Errorf("javascript: href kept: %s", out)
	}
	if !strings.Contains(out, `data-field="party_a"`) {
		t.Errorf("field placeholder dropped: %s", out)
	}
	if strings.Contains(out, "contenteditable") {
		t.Errorf("contenteditable attribute kept: %s", out)
	}
	if !strings.Contains(out, "降级") {
		t.Errorf("unwrapped element lost its text: %s", out)
	}
	if !strings.Contains(out, `rel="noopener noreferrer nofollow"`) {
		t.Errorf("link rel not added: %s", out)
	}
}

func TestTemplateCRUDAndIsolation(t *testing.T) {
	r, _ := newTestAPI(t)

	created := createTemplateForTest(t, r, 1, "租赁合同")
	id := int(created["id"].(float64))

	// 另一个用户看不到
	if w, _ := doJSON(t, r, http.MethodGet, fmt.Sprintf("/api/contract/templates/%d", id), 2, nil); w.Code != http.StatusNotFound {
		t.Errorf("cross-user read = %d, want 404", w.Code)
	}

	// 列表（含示例模板种子：用户 1 首次列表会补示例）
	w, parsed := doJSON(t, r, http.MethodGet, "/api/contract/templates", 2, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("list: %d", w.Code)
	}
	data := dataMap(t, parsed)
	items := data["items"].([]interface{})
	if len(items) == 0 {
		t.Fatal("seed template should be created on first list")
	}
	summary := items[0].(map[string]interface{})
	if _, ok := summary["content"]; ok {
		t.Error("list items must not carry full content")
	}
	if summary["field_count"].(float64) == 0 {
		t.Error("field_count missing")
	}

	// 更新
	w, _ = doJSON(t, r, http.MethodPut, fmt.Sprintf("/api/contract/templates/%d", id), 1, gin.H{
		"name": "租赁合同 v2", "content": "<p>更新后</p>", "fields": []gin.H{{"label": "甲", "type": "text"}},
	})
	if w.Code != http.StatusOK {
		t.Fatalf("update: %d %s", w.Code, w.Body.String())
	}
	// 非法字段
	w, _ = doJSON(t, r, http.MethodPut, fmt.Sprintf("/api/contract/templates/%d", id), 1, gin.H{
		"name": "x", "fields": []gin.H{{"label": "", "type": "text"}},
	})
	if w.Code != http.StatusBadRequest {
		t.Errorf("invalid fields = %d, want 400", w.Code)
	}
}

func TestContractLifecycleAndSnapshot(t *testing.T) {
	r, db := newTestAPI(t)
	created := createTemplateForTest(t, r, 1, "服务合同")
	templateID := int(created["id"].(float64))

	// 从模板生成
	w, parsed := doJSON(t, r, http.MethodPost, "/api/contract/contracts", 1, gin.H{
		"template_id": templateID,
		"values":      gin.H{"party_a": "张三"},
	})
	if w.Code != http.StatusOK {
		t.Fatalf("create contract: %d %s", w.Code, w.Body.String())
	}
	contract := dataMap(t, parsed)
	contractID := int(contract["id"].(float64))
	rendered, _ := contract["rendered"].(string)
	if !strings.Contains(rendered, `<span class="ct-value">张三</span>`) {
		t.Errorf("rendered missing value: %s", rendered)
	}
	if !strings.Contains(rendered, "ct-blank") {
		t.Errorf("unfilled field should be blank: %s", rendered)
	}
	if contract["status"] != StatusDraft {
		t.Errorf("status = %v, want draft", contract["status"])
	}

	// 快照独立：改模板后合同不变
	doJSON(t, r, http.MethodPut, fmt.Sprintf("/api/contract/templates/%d", templateID), 1, gin.H{
		"name": "服务合同 v2", "content": "<p>全新正文</p>",
		"fields": []gin.H{{"label": "甲", "type": "text"}},
	})
	var snap Contract
	if err := db.First(&snap, contractID).Error; err != nil {
		t.Fatalf("load contract: %v", err)
	}
	if !strings.Contains(snap.Content, "甲方") {
		t.Error("contract content must keep its snapshot after template edit")
	}

	// 更新值触发重渲染
	w, parsed = doJSON(t, r, http.MethodPut, fmt.Sprintf("/api/contract/contracts/%d", contractID), 1, gin.H{
		"values": gin.H{"party_a": "李四", "party_b": "王五"},
		"status": StatusActive,
	})
	if w.Code != http.StatusOK {
		t.Fatalf("update contract: %d %s", w.Code, w.Body.String())
	}
	contract = dataMap(t, parsed)
	rendered, _ = contract["rendered"].(string)
	if !strings.Contains(rendered, "李四") || !strings.Contains(rendered, "王五") {
		t.Errorf("re-render missing values: %s", rendered)
	}
	if contract["status"] != StatusActive {
		t.Errorf("status = %v, want active", contract["status"])
	}

	// 非法状态
	w, _ = doJSON(t, r, http.MethodPut, fmt.Sprintf("/api/contract/contracts/%d", contractID), 1, gin.H{"status": "void"})
	if w.Code != http.StatusBadRequest {
		t.Errorf("invalid status = %d, want 400", w.Code)
	}

	// 复制与删除
	w, _ = doJSON(t, r, http.MethodPost, fmt.Sprintf("/api/contract/contracts/%d/duplicate", contractID), 1, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("duplicate: %d", w.Code)
	}
	w, _ = doJSON(t, r, http.MethodDelete, fmt.Sprintf("/api/contract/contracts/%d", contractID), 2, nil)
	if w.Code != http.StatusNotFound {
		t.Errorf("cross-user delete = %d, want 404", w.Code)
	}
	w, _ = doJSON(t, r, http.MethodDelete, fmt.Sprintf("/api/contract/contracts/%d", contractID), 1, nil)
	if w.Code != http.StatusOK {
		t.Errorf("delete: %d", w.Code)
	}
}
