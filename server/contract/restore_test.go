package contract

import (
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"smallgo/server/backup"
	"smallgo/server/config"
	"smallgo/server/database"
)

// newRestoreDB 建一个已迁移的临时库。
func newRestoreDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "restore-test.db")), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := database.AutoMigrate(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

// makeBackup 造一份当前库的备份，返回绝对路径。
func makeBackup(t *testing.T, db *gorm.DB, dataDir string) string {
	t.Helper()
	name, err := backup.Create(db, dataDir)
	if err != nil {
		t.Fatalf("create backup: %v", err)
	}
	return filepath.Join(backup.Dir(dataDir), name)
}

func TestRestoreFromBackupRollsBackData(t *testing.T) {
	db := newRestoreDB(t)
	dataDir := t.TempDir()

	// 备份时的基线数据
	if err := db.Create(&ContractTemplate{UserID: 1, Name: "原始模板", Content: "<p>a</p>", Fields: "[]"}).Error; err != nil {
		t.Fatalf("seed template: %v", err)
	}
	if err := db.Create(&Contract{UserID: 1, TemplateName: "原始模板", Title: "原始合同", Content: "<p>a</p>", Fields: "[]", Values: "{}", Rendered: "<p>a</p>"}).Error; err != nil {
		t.Fatalf("seed contract: %v", err)
	}
	src := makeBackup(t, db, dataDir)
	if err := verifyBackupFile(src); err != nil {
		t.Fatalf("verify backup: %v", err)
	}

	// 备份之后破坏数据：改名、删合同、插新行
	if err := db.Model(&ContractTemplate{}).Where("name = ?", "原始模板").
		Update("name", "被改过的名字").Error; err != nil {
		t.Fatalf("mutate: %v", err)
	}
	if err := db.Unscoped().Where("1 = 1").Delete(&Contract{}).Error; err != nil {
		t.Fatalf("delete contracts: %v", err)
	}
	if err := db.Create(&Contract{UserID: 1, TemplateName: "t", Title: "备份之后新增", Content: "", Fields: "[]", Values: "{}", Rendered: ""}).Error; err != nil {
		t.Fatalf("insert after backup: %v", err)
	}

	tables, rows, err := restoreFromBackup(db, src)
	if err != nil {
		t.Fatalf("restore: %v", err)
	}
	if tables == 0 || rows < 2 {
		t.Fatalf("restored tables=%d rows=%d, want >0 tables and >=2 rows", tables, rows)
	}

	var tpl ContractTemplate
	if err := db.First(&tpl).Error; err != nil {
		t.Fatalf("query template after restore: %v", err)
	}
	if tpl.Name != "原始模板" {
		t.Errorf("template name = %q, want 原始模板", tpl.Name)
	}
	var contracts []Contract
	if err := db.Find(&contracts).Error; err != nil {
		t.Fatalf("query contracts: %v", err)
	}
	if len(contracts) != 1 || contracts[0].Title != "原始合同" {
		t.Errorf("contracts after restore = %+v, want exactly [原始合同]", contracts)
	}
}

func TestRestoreRejectsBadFile(t *testing.T) {
	db := newRestoreDB(t)
	dataDir := t.TempDir()

	// 不存在的文件
	if _, _, err := restoreFromBackup(db, filepath.Join(dataDir, "nope.db")); err == nil {
		t.Error("missing file should fail")
	}

	// 非 SQLite 内容（模拟损坏备份）
	bad := filepath.Join(dataDir, "bad.db")
	if err := os.WriteFile(bad, []byte("this is not a database"), 0o644); err != nil {
		t.Fatalf("write bad file: %v", err)
	}
	if err := verifyBackupFile(bad); err == nil {
		t.Error("verifyBackupFile should reject non-sqlite content")
	}

	// 合法 SQLite 但没有 users 表：不是本应用的备份
	noUsers := filepath.Join(dataDir, "nousers.db")
	other, err := gorm.Open(sqlite.Open(noUsers), &gorm.Config{})
	if err != nil {
		t.Fatalf("open other db: %v", err)
	}
	if err := other.Exec("CREATE TABLE unrelated (id INTEGER PRIMARY KEY)").Error; err != nil {
		t.Fatalf("create table: %v", err)
	}
	if sqlDB, _ := other.DB(); sqlDB != nil {
		sqlDB.Close()
	}
	if err := verifyBackupFile(noUsers); err == nil {
		t.Error("verifyBackupFile should reject a db without users table")
	}
}

func TestRestoreHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := newRestoreDB(t)

	config.C.DataDir = t.TempDir()
	if err := db.Create(&ContractTemplate{UserID: 1, Name: "基线", Content: "<p>a</p>", Fields: "[]"}).Error; err != nil {
		t.Fatalf("seed: %v", err)
	}
	name := filepath.Base(makeBackup(t, db, config.C.DataDir))

	r := gin.New()
	api := r.Group("/api")
	api.Use(func(c *gin.Context) {
		c.Set("userID", uint(1))
		c.Set("username", "admin")
		c.Next()
	})
	setupAdminRoutes(api, db)

	// 破坏数据后经接口恢复
	if err := db.Unscoped().Where("1 = 1").Delete(&ContractTemplate{}).Error; err != nil {
		t.Fatalf("delete: %v", err)
	}

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/backups/"+name+"/restore", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("restore status = %d, body = %s", w.Code, w.Body.String())
	}
	body := w.Body.String()
	if !containsJSON(body, "pre_backup") {
		t.Errorf("response should carry pre_backup, got %s", body)
	}

	var tpl ContractTemplate
	if err := db.First(&tpl).Error; err != nil || tpl.Name != "基线" {
		t.Errorf("template after handler restore = %+v (err %v), want 基线", tpl, err)
	}

	// 非法文件名 → 400
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/backups/evil.db/restore", nil))
	if w.Code != http.StatusBadRequest {
		t.Errorf("bad name status = %d, want 400", w.Code)
	}

	// 不存在的备份 → 404
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost,
		"/api/backups/backup_20260101_000000.db/restore", nil))
	if w.Code != http.StatusNotFound {
		t.Errorf("missing backup status = %d, want 404", w.Code)
	}
}

func containsJSON(body, key string) bool {
	return strings.Contains(body, `"`+key+`"`)
}

// TestUploadBackupHandler 验证本地上传备份：合法 SQLite 落进备份目录并可被
// 恢复流程使用；垃圾内容与缺 users 表的库被拒，半截文件不进列表。
func TestUploadBackupHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := newRestoreDB(t)
	config.C.DataDir = t.TempDir()

	r := gin.New()
	api := r.Group("/api")
	api.Use(func(c *gin.Context) {
		c.Set("userID", uint(1))
		c.Set("username", "admin")
		c.Next()
	})
	setupAdminRoutes(api, db)

	// 造一份合法备份作为上传源
	if err := db.Create(&ContractTemplate{UserID: 1, Name: "上传源模板", Content: "<p>a</p>", Fields: "[]"}).Error; err != nil {
		t.Fatalf("seed: %v", err)
	}
	src := makeBackup(t, db, config.C.DataDir)
	srcBytes, err := os.ReadFile(src)
	if err != nil {
		t.Fatalf("read source backup: %v", err)
	}

	upload := func(name string, body []byte) *httptest.ResponseRecorder {
		var buf strings.Builder
		w := multipart.NewWriter(&buf)
		part, _ := w.CreateFormFile("file", name)
		part.Write(body)
		w.Close()
		req := httptest.NewRequest(http.MethodPost, "/api/backups/upload", strings.NewReader(buf.String()))
		req.Header.Set("Content-Type", w.FormDataContentType())
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		return rec
	}

	// 合法备份上传成功，返回的文件名满足备份名规则
	w := upload("下载的备份.db", srcBytes)
	if w.Code != http.StatusOK {
		t.Fatalf("upload status = %d, body = %s", w.Code, w.Body.String())
	}
	uploaded := decodeUploadName(t, w.Body.String())
	items, err := backup.List(config.C.DataDir)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	found := false
	for _, it := range items {
		if it.Name == uploaded {
			found = true
		}
	}
	if !found {
		t.Fatalf("uploaded backup %q not in list: %+v", uploaded, items)
	}

	// 上传的备份能走恢复流程：清空数据后恢复
	if err := db.Unscoped().Where("1 = 1").Delete(&ContractTemplate{}).Error; err != nil {
		t.Fatalf("delete: %v", err)
	}
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/backups/"+uploaded+"/restore", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("restore uploaded backup status = %d, body = %s", w.Code, w.Body.String())
	}
	var tpl ContractTemplate
	if err := db.First(&tpl).Error; err != nil || tpl.Name != "上传源模板" {
		t.Errorf("after restore = %+v (err %v), want 上传源模板", tpl, err)
	}

	// 非 .db 后缀 → 400
	if w := upload("not-a-backup.txt", srcBytes); w.Code != http.StatusBadRequest {
		t.Errorf("upload .txt status = %d, want 400", w.Code)
	}
	// 垃圾内容 → 400
	if w := upload("bad.db", []byte("this is not a database")); w.Code != http.StatusBadRequest {
		t.Errorf("upload garbage status = %d, want 400", w.Code)
	}
	// 合法 SQLite 但没有 users 表 → 400
	noUsers := filepath.Join(t.TempDir(), "nousers.db")
	other, err := gorm.Open(sqlite.Open(noUsers), &gorm.Config{})
	if err != nil {
		t.Fatalf("open other db: %v", err)
	}
	if err := other.Exec("CREATE TABLE unrelated (id INTEGER PRIMARY KEY)").Error; err != nil {
		t.Fatalf("create table: %v", err)
	}
	if sqlDB, _ := other.DB(); sqlDB != nil {
		sqlDB.Close()
	}
	otherBytes, _ := os.ReadFile(noUsers)
	if w := upload("nousers.db", otherBytes); w.Code != http.StatusBadRequest {
		t.Errorf("upload db without users table status = %d, want 400", w.Code)
	}
	// 校验失败的不留残余文件
	entries, _ := os.ReadDir(backup.Dir(config.C.DataDir))
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".upload-") {
			t.Errorf("temp upload file left behind: %s", e.Name())
		}
	}
}

func decodeUploadName(t *testing.T, body string) string {
	t.Helper()
	i := strings.Index(body, `"name":"`)
	if i < 0 {
		t.Fatalf("no name in response: %s", body)
	}
	rest := body[i+len(`"name":"`):]
	j := strings.Index(rest, `"`)
	if j < 0 {
		t.Fatalf("unterminated name: %s", body)
	}
	return rest[:j]
}
