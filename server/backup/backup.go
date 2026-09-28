// Package backup 提供数据库备份管理：手动/自动创建 SQLite 快照、列表、
// 下载与删除，并按保留数量自动清理。
//
// 备份文件放在 <data-dir>/backups 下，命名为 backup_YYYYMMDD_HHMMSS.db，
// 由 SQLite 的 VACUUM INTO 生成一致性快照（WAL 模式下也安全）。
package backup

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"smallgo/server/audit"
	"smallgo/server/logger"
	"smallgo/server/response"
	"smallgo/server/scheduler"
	"smallgo/server/sysconfig"
)

const dirName = "backups"

// backup_YYYYMMDD_HHMMSS.db；同秒多次备份时追加 -N 序号（见 Create）。
var namePattern = regexp.MustCompile(`^backup_\d{8}_\d{6}(?:-\d+)?\.db$`)

var (
	dbRef   *gorm.DB
	dataRef string
)

// Dir 返回备份目录路径并确保存在。
func Dir(dataDir string) string {
	dir := filepath.Join(dataDir, dirName)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		logger.Error("创建备份目录失败: %v", err)
	}
	return dir
}

// Create 生成一个一致性数据库快照，返回文件名。
// 时间戳精确到秒，同秒内再次备份时自动追加 -1/-2 序号避免覆盖。
func Create(db *gorm.DB, dataDir string) (string, error) {
	dir := Dir(dataDir)
	base := "backup_" + time.Now().Format("20060102_150405")
	target := filepath.Join(dir, base+".db")
	if _, err := os.Stat(target); err == nil {
		for i := 1; ; i++ {
			name := fmt.Sprintf("%s-%d.db", base, i)
			p := filepath.Join(dir, name)
			if _, err := os.Stat(p); os.IsNotExist(err) {
				target = p
				break
			}
			if i >= 99 {
				return "", fmt.Errorf("备份文件名冲突过多")
			}
		}
	}
	if err := db.Exec("VACUUM INTO ?", target).Error; err != nil {
		return "", fmt.Errorf("VACUUM INTO 失败: %w", err)
	}
	return filepath.Base(target), nil
}

// Item 是备份文件的列表视图。
type Item struct {
	Name      string    `json:"name"`
	Size      int64     `json:"size"`
	CreatedAt time.Time `json:"created_at"`
}

// List 返回全部备份，新在前。
func List(dataDir string) ([]Item, error) {
	entries, err := os.ReadDir(Dir(dataDir))
	if err != nil {
		return nil, err
	}
	items := make([]Item, 0)
	for _, e := range entries {
		if e.IsDir() || !namePattern.MatchString(e.Name()) {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		items = append(items, Item{
			Name:      e.Name(),
			Size:      info.Size(),
			CreatedAt: info.ModTime(),
		})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Name > items[j].Name })
	return items, nil
}

// Prune 按保留数量清理最旧的备份，返回删除的文件数。
func Prune(dataDir string, keep int) (int, error) {
	if keep < 1 {
		keep = 1
	}
	items, err := List(dataDir)
	if err != nil {
		return 0, err
	}
	removed := 0
	for i := keep; i < len(items); i++ {
		if err := os.Remove(filepath.Join(Dir(dataDir), items[i].Name)); err != nil {
			return removed, err
		}
		removed++
	}
	return removed, nil
}

// keepCount 读取保留数量配置，非法值回退默认 7。
func keepCount(db *gorm.DB) int {
	keep := 7
	if db == nil {
		return keep
	}
	if v, err := sysconfig.GetConfig(db, "backup_keep_count", 0); err == nil && v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 1 {
			keep = n
		}
	}
	return keep
}

// autoEnabled 读取自动备份开关（默认开启）。
func autoEnabled(db *gorm.DB) bool {
	if db == nil {
		return false
	}
	v, err := sysconfig.GetConfig(db, "backup_auto_enabled", 0)
	if err != nil || v == "" {
		return true
	}
	return v != "false"
}

func runAutoBackup() {
	db, dataDir := dbRef, dataRef
	if db == nil || dataDir == "" {
		return
	}
	if !autoEnabled(db) {
		return
	}
	name, err := Create(db, dataDir)
	if err != nil {
		logger.Error("自动备份失败: %v", err)
		return
	}
	if n, err := Prune(dataDir, keepCount(db)); err != nil {
		logger.Error("备份清理失败: %v", err)
	} else if n > 0 {
		logger.Info("自动备份 %s 完成，清理了 %d 个过期备份", name, n)
	}
}

// RegisterRoutes 挂载管理员备份接口，并记住运行时依赖供自动备份任务使用。
func RegisterRoutes(admin *gin.RouterGroup, db *gorm.DB, dataDir string) {
	dbRef, dataRef = db, dataDir

	admin.GET("/backups", func(c *gin.Context) {
		items, err := List(dataDir)
		if err != nil {
			response.ErrorInternal(c, "读取备份列表失败")
			return
		}
		response.Success(c, gin.H{
			"items":         items,
			"dir":           Dir(dataDir),
			"auto_enabled":  autoEnabled(db),
			"keep_count":    keepCount(db),
		})
	})

	admin.POST("/backups", func(c *gin.Context) {
		name, err := Create(db, dataDir)
		if err != nil {
			logger.Error("手动备份失败: %v", err)
			response.ErrorInternal(c, "备份失败："+err.Error())
			return
		}
		if n, err := Prune(dataDir, keepCount(db)); err == nil && n > 0 {
			logger.Info("备份 %s 完成，清理了 %d 个过期备份", name, n)
		}
		audit.Log(db, c, "backup_create", "backup", 0, name)
		response.Success(c, gin.H{"name": name})
	})

	admin.GET("/backups/:name/download", func(c *gin.Context) {
		name := c.Param("name")
		if !namePattern.MatchString(name) {
			response.ErrorBadRequest(c, "无效的备份文件名")
			return
		}
		path := filepath.Join(Dir(dataDir), name)
		if _, err := os.Stat(path); err != nil {
			response.ErrorNotFound(c, "备份不存在")
			return
		}
		c.FileAttachment(path, name)
	})

	admin.DELETE("/backups/:name", func(c *gin.Context) {
		name := c.Param("name")
		if !namePattern.MatchString(name) {
			response.ErrorBadRequest(c, "无效的备份文件名")
			return
		}
		path := filepath.Join(Dir(dataDir), name)
		if _, err := os.Stat(path); err != nil {
			response.ErrorNotFound(c, "备份不存在")
			return
		}
		if err := os.Remove(path); err != nil {
			response.ErrorInternal(c, "删除备份失败")
			return
		}
		audit.Log(db, c, "backup_delete", "backup", 0, name)
		response.Success(c, gin.H{"ok": true})
	})
}

func init() {
	scheduler.Register(scheduler.Job{
		Name: "auto_backup",
		// 每天低峰期执行一次；是否真正备份由 backup_auto_enabled 决定
		Daily: "03:00",
		Run:   runAutoBackup,
	})

	sysconfig.RegisterConfig(sysconfig.ConfigDef{
		Key:     "backup_auto_enabled",
		Scope:   sysconfig.ScopeSystem,
		Type:    sysconfig.TypeBool,
		Default: "true",
		Group:   "backup",
		Label:   "自动备份",
		Description: "每天 03:00 自动创建一次数据库快照，存放于数据目录 backups/ 下",
	})
	sysconfig.RegisterConfig(sysconfig.ConfigDef{
		Key:     "backup_keep_count",
		Scope:   sysconfig.ScopeSystem,
		Type:    sysconfig.TypeInt,
		Default: "7",
		Group:   "backup",
		Label:   "备份保留数量",
		Description: "自动/手动备份超出该数量时自动清理最旧的备份",
	})
}
