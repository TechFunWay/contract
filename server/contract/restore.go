// 恢复功能：把某个历史备份恢复为当前数据库。
//
// 恢复属于框架 backup 能力的补充，但框架层包（backup/ 等）要与 smallgo 基线
// 保持一致，不能直接改，因此这里经业务 App 的 SetupAdmin 把恢复接口挂到同一个
// adminGroup 上（POST /backups/:name/restore），实现完全放在实例层。
//
// 恢复语义：
//  1. 校验：文件名合法 → SQLite 文件头 → PRAGMA integrity_check → 必须含 users 表
//  2. 恢复前先给当前库自动拍一份快照（backup.Create），保证可回退
//  3. ATTACH 备份库，按「当前结构为准」逐表拷贝：交集列 INSERT…SELECT，
//     当前新增列按类型回填零值，当前多出的表清空；全部写操作在一个事务里，
//     失败整体回滚，数据库保持原样
//  4. 恢复后跑一遍 AutoMigrate，把旧备份缺的表/列补上
//  5. 写审计日志，返回恢复前快照文件名
package contract

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"smallgo/server/audit"
	"smallgo/server/backup"
	"smallgo/server/config"
	"smallgo/server/database"
	"smallgo/server/logger"
	"smallgo/server/response"
)

// 与框架 backup 包同规则（那边是私有变量，这里复刻一份）。
var backupNamePattern = regexp.MustCompile(`^backup_\d{8}_\d{6}(?:-\d+)?\.db$`)

const sqliteHeader = "SQLite format 3\x00"

// setupAdminRoutes 挂管理员恢复接口。由 contract 的 apps.App.SetupAdmin 调用。
// 同时注入自动备份任务的运行时依赖（见 backup_interval.go）。
func setupAdminRoutes(api *gin.RouterGroup, db *gorm.DB) {
	setAutoBackupDB(db)
	api.POST("/backups/:name/restore", handleRestoreBackup(db))
	api.POST("/backups/upload", handleUploadBackup(db))
}

func handleRestoreBackup(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		name := c.Param("name")
		if !backupNamePattern.MatchString(name) {
			response.ErrorBadRequest(c, "无效的备份文件名")
			return
		}
		src := filepath.Join(backup.Dir(config.C.DataDir), name)
		if _, err := os.Stat(src); err != nil {
			response.ErrorNotFound(c, "备份不存在")
			return
		}
		if err := verifyBackupFile(src); err != nil {
			response.ErrorBadRequest(c, "备份校验失败："+err.Error())
			return
		}

		// 恢复前快照：无论恢复结果如何，当前数据都先留一份可回退的备份。
		pre, err := backup.Create(db, config.C.DataDir)
		if err != nil {
			response.ErrorInternal(c, "创建恢复前快照失败："+err.Error())
			return
		}

		tables, rows, err := restoreFromBackup(db, src)
		if err != nil {
			logger.Error("恢复备份 %s 失败: %v（恢复前快照 %s）", name, err, pre)
			response.ErrorInternal(c, "恢复失败，数据未变更（恢复前快照："+pre+"）："+err.Error())
			return
		}
		// 旧备份可能缺表/列，按当前模型补齐结构。
		if err := database.AutoMigrate(db); err != nil {
			response.ErrorInternal(c, "数据已恢复但结构迁移失败（恢复前快照："+pre+"）："+err.Error())
			return
		}

		logger.Info("恢复备份 %s 完成：%d 张表 / %d 行，恢复前快照 %s", name, tables, rows, pre)
		audit.Log(db, c, "backup_restore", "backup", 0,
			fmt.Sprintf("%s（恢复前快照 %s，%d 表 / %d 行）", name, pre, tables, rows))
		response.Success(c, gin.H{"pre_backup": pre, "tables": tables, "rows": rows})
	}
}

// verifyBackupFile 校验备份可用：文件头是 SQLite、integrity_check 通过、
// 且包含 users 表（防止把任意 SQLite 文件丢进目录就当备份恢复）。
func verifyBackupFile(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("打开备份失败: %w", err)
	}
	defer f.Close()
	head := make([]byte, len(sqliteHeader))
	if _, err := io.ReadFull(f, head); err != nil {
		return fmt.Errorf("读取备份文件头失败: %w", err)
	}
	if string(head) != sqliteHeader {
		return fmt.Errorf("不是有效的 SQLite 数据库文件")
	}

	probe, err := gorm.Open(sqlite.Open(path), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
	if err != nil {
		return fmt.Errorf("打开备份失败: %w", err)
	}
	defer func() {
		if sqlDB, e := probe.DB(); e == nil {
			_ = sqlDB.Close()
		}
	}()
	sqlDB, err := probe.DB()
	if err != nil {
		return fmt.Errorf("获取备份连接失败: %w", err)
	}

	rows, err := sqlDB.Query("PRAGMA integrity_check")
	if err != nil {
		return fmt.Errorf("完整性检查失败: %w", err)
	}
	var problems []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			rows.Close()
			return fmt.Errorf("读取完整性检查结果失败: %w", err)
		}
		problems = append(problems, s)
	}
	rows.Close()
	if len(problems) == 0 || problems[0] != "ok" {
		return fmt.Errorf("完整性检查未通过: %s", strings.Join(problems, "; "))
	}

	var n int
	if err := sqlDB.QueryRow(
		`SELECT count(*) FROM sqlite_master WHERE type='table' AND name='users'`,
	).Scan(&n); err != nil {
		return fmt.Errorf("检查备份表结构失败: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("备份中缺少 users 表，不是本应用的数据库备份")
	}
	return nil
}

type colInfo struct {
	name string
	typ  string
}

// restoreFromBackup 把 srcPath 的内容恢复进当前库，返回恢复的表数与行数。
// 全部写操作在一个事务内，失败回滚，当前数据不变。
func restoreFromBackup(db *gorm.DB, srcPath string) (int, int, error) {
	// ATTACH 对不存在的路径会静默创建一个空库，必须先确认文件真实存在。
	if _, err := os.Stat(srcPath); err != nil {
		return 0, 0, fmt.Errorf("备份文件不存在或不可读: %w", err)
	}
	escaped := strings.ReplaceAll(srcPath, "'", "''")

	// 外键约束临时关闭：跨表删插不按依赖顺序。连接池固定单连接
	//（InitDB SetMaxOpenConns(1)），PRAGMA 与随后的事务必然同一连接。
	if err := db.Exec("PRAGMA foreign_keys=OFF").Error; err != nil {
		return 0, 0, fmt.Errorf("关闭外键约束失败: %w", err)
	}
	defer func() { _ = db.Exec("PRAGMA foreign_keys=ON") }()

	if err := db.Exec(fmt.Sprintf("ATTACH DATABASE '%s' AS bak", escaped)).Error; err != nil {
		return 0, 0, fmt.Errorf("挂载备份库失败: %w", err)
	}
	defer func() { _ = db.Exec("DETACH DATABASE bak") }()

	bakTables, err := readTableNames(db, "bak")
	if err != nil {
		return 0, 0, fmt.Errorf("读取备份表清单失败: %w", err)
	}
	mainTables, err := readTableNames(db, "main")
	if err != nil {
		return 0, 0, fmt.Errorf("读取当前表清单失败: %w", err)
	}
	bakSet := make(map[string]bool, len(bakTables))
	for _, t := range bakTables {
		bakSet[t] = true
	}

	tx := db.Begin()
	if tx.Error != nil {
		return 0, 0, fmt.Errorf("开启恢复事务失败: %w", tx.Error)
	}
	committed := false
	defer func() {
		if !committed {
			tx.Rollback()
		}
	}()

	restoredTables := 0
	restoredRows := 0
	for _, table := range bakTables {
		// 备份里有、当前结构没有的表：以当前结构为准，忽略。
		if !containsString(mainTables, table) {
			continue
		}
		bakCols, err := readColumns(tx, "bak", table)
		if err != nil {
			return 0, 0, fmt.Errorf("读取备份表 %s 结构失败: %w", table, err)
		}
		mainCols, err := readColumns(tx, "main", table)
		if err != nil {
			return 0, 0, fmt.Errorf("读取当前表 %s 结构失败: %w", table, err)
		}
		common := commonColumns(mainCols, bakCols)
		if len(common) == 0 {
			continue
		}

		ident := quoteIdent(table)
		if err := tx.Exec("DELETE FROM main." + ident).Error; err != nil {
			return 0, 0, fmt.Errorf("清空表 %s 失败: %w", table, err)
		}
		colList := quotedNames(common)
		res := tx.Exec(fmt.Sprintf(
			"INSERT INTO main.%s (%s) SELECT %s FROM bak.%s", ident, colList, colList, ident))
		if res.Error != nil {
			return 0, 0, fmt.Errorf("拷贝表 %s 数据失败: %w", table, res.Error)
		}
		// 当前比备份新增的列（备份更旧）：按列类型回填零值，避免 NULL 扫描失败。
		if missing := missingColumns(mainCols, bakCols); len(missing) > 0 {
			sets := make([]string, 0, len(missing))
			for _, col := range missing {
				sets = append(sets, quoteIdent(col.name)+" = "+defaultLiteral(col.typ))
			}
			if err := tx.Exec(fmt.Sprintf("UPDATE main.%s SET %s",
				ident, strings.Join(sets, ", "))).Error; err != nil {
				return 0, 0, fmt.Errorf("回填表 %s 新增列失败: %w", table, err)
			}
		}
		restoredTables++
		restoredRows += int(res.RowsAffected)
	}

	// 当前有、备份没有的表：清空，使整体状态与备份一致。
	for _, table := range mainTables {
		if bakSet[table] {
			continue
		}
		if err := tx.Exec("DELETE FROM main." + quoteIdent(table)).Error; err != nil {
			return 0, 0, fmt.Errorf("清空表 %s 失败: %w", table, err)
		}
	}

	if err := tx.Commit().Error; err != nil {
		return 0, 0, fmt.Errorf("提交恢复事务失败: %w", err)
	}
	committed = true
	return restoredTables, restoredRows, nil
}

func readTableNames(db *gorm.DB, schema string) ([]string, error) {
	q := fmt.Sprintf(
		`SELECT name FROM %s.sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%%' ORDER BY name`,
		schema)
	rows, err := db.Raw(q).Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	names := make([]string, 0)
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		names = append(names, name)
	}
	return names, rows.Err()
}

func readColumns(db *gorm.DB, schema, table string) ([]colInfo, error) {
	rows, err := db.Raw(fmt.Sprintf("PRAGMA %s.table_info(%s)", schema, quoteLiteral(table))).Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	cols := make([]colInfo, 0)
	for rows.Next() {
		var cid, notnull, pk int
		var name, typ string
		var dflt any
		if err := rows.Scan(&cid, &name, &typ, &notnull, &dflt, &pk); err != nil {
			return nil, err
		}
		cols = append(cols, colInfo{name: name, typ: typ})
	}
	return cols, rows.Err()
}

// commonColumns 返回当前表与备份表共有的列，顺序以当前表为准。
func commonColumns(mainCols, bakCols []colInfo) []colInfo {
	bakHas := make(map[string]bool, len(bakCols))
	for _, c := range bakCols {
		bakHas[c.name] = true
	}
	out := make([]colInfo, 0, len(mainCols))
	for _, c := range mainCols {
		if bakHas[c.name] {
			out = append(out, c)
		}
	}
	return out
}

// missingColumns 返回当前表有、备份表没有的列。
func missingColumns(mainCols, bakCols []colInfo) []colInfo {
	bakHas := make(map[string]bool, len(bakCols))
	for _, c := range bakCols {
		bakHas[c.name] = true
	}
	out := make([]colInfo, 0)
	for _, c := range mainCols {
		if !bakHas[c.name] {
			out = append(out, c)
		}
	}
	return out
}

// defaultLiteral 按 SQLite 声明类型给出回填零值。
func defaultLiteral(typ string) string {
	u := strings.ToUpper(typ)
	switch {
	case strings.Contains(u, "INT"), strings.Contains(u, "REAL"),
		strings.Contains(u, "FLOA"), strings.Contains(u, "DOUB"),
		strings.Contains(u, "NUMER"), strings.Contains(u, "BOOL"):
		return "0"
	case strings.Contains(u, "DATE"), strings.Contains(u, "TIME"),
		strings.Contains(u, "STAMP"):
		return "'1970-01-01 00:00:00'"
	default:
		return "''"
	}
}

func quoteIdent(name string) string {
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}

func quotedNames(cols []colInfo) string {
	quoted := make([]string, 0, len(cols))
	for _, c := range cols {
		quoted = append(quoted, quoteIdent(c.name))
	}
	return strings.Join(quoted, ", ")
}

func quoteLiteral(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}

func containsString(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}
