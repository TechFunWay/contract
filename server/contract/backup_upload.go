// 本地上传备份：把浏览器上传的 .db 备份文件落进备份目录，之后走与列表备份
// 完全相同的「恢复」流程（恢复前快照 → 事务恢复 → 结构迁移）。
//
// 与 restore.go 同理放在实例层：框架 backup 包要与 smallgo 基线保持一致，
// 上传接口经业务 App 的 SetupAdmin 挂到同一个 adminGroup（POST /backups/upload）。
//
// 安全要点：
//  1. 仅管理员（adminGroup）；请求体上限 maxUploadBytes
//  2. 只收 .db 后缀；先落到不匹配备份名规则的临时文件，校验通过后才改成
//     正式名——半截文件 / 校验失败的文件不会混进备份列表，也不会被恢复
//  3. 复用 verifyBackupFile：SQLite 文件头 + integrity_check + 必须含 users 表
//  4. 正式名由服务端生成（backup_YYYYMMDD_HHMMSS.db，冲突追加 -N），与
//     框架 backup.Create 同规则，天然满足列表/下载/恢复的文件名校验
package contract

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"smallgo/server/audit"
	"smallgo/server/backup"
	"smallgo/server/config"
	"smallgo/server/logger"
	"smallgo/server/response"
)

// maxUploadBytes 单个上传备份的大小上限。正常备份远小于此，上限只用于
// 兜底防御（防把接口当网盘、防内存被拖爆）。
const maxUploadBytes = 256 << 20

// handleUploadBackup 接收浏览器上传的备份文件，校验通过后存为一份备份。
// 上传本身不改变当前数据；真正恢复仍走列表上的「恢复」按钮（带确认弹窗
// 与恢复前快照）。
func handleUploadBackup(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxUploadBytes)
		fileHeader, err := c.FormFile("file")
		if err != nil {
			response.ErrorBadRequest(c, "请选择要上传的备份文件（.db）")
			return
		}
		if fileHeader.Size > maxUploadBytes {
			response.ErrorBadRequest(c, "备份文件过大（上限 256MB）")
			return
		}
		if ext := strings.ToLower(filepath.Ext(fileHeader.Filename)); ext != ".db" {
			response.ErrorBadRequest(c, "只支持 .db 备份文件")
			return
		}

		dir := backup.Dir(config.C.DataDir)
		tmp, err := os.CreateTemp(dir, ".upload-*.tmp")
		if err != nil {
			response.ErrorInternal(c, "保存上传文件失败")
			return
		}
		tmpName := tmp.Name()
		defer os.Remove(tmpName) // 成功改名后路径已不存在，此调用是空操作

		src, err := fileHeader.Open()
		if err != nil {
			tmp.Close()
			response.ErrorBadRequest(c, "读取上传文件失败")
			return
		}
		_, copyErr := io.Copy(tmp, src)
		src.Close()
		closeErr := tmp.Close()
		if copyErr != nil || closeErr != nil {
			response.ErrorInternal(c, "保存上传文件失败")
			return
		}

		if err := verifyBackupFile(tmpName); err != nil {
			response.ErrorBadRequest(c, "备份校验失败："+err.Error())
			return
		}

		name := uniqueBackupName(dir)
		if err := os.Rename(tmpName, filepath.Join(dir, name)); err != nil {
			response.ErrorInternal(c, "保存备份失败")
			return
		}

		logger.Info("上传备份 %s（原文件名 %s，%d 字节）", name, fileHeader.Filename, fileHeader.Size)
		audit.Log(db, c, "backup_upload", "backup", 0,
			fmt.Sprintf("%s（原文件 %s）", name, fileHeader.Filename))
		response.Success(c, gin.H{"name": name})
	}
}

// uniqueBackupName 生成 backup_YYYYMMDD_HHMMSS.db，同秒冲突追加 -N 序号，
// 与框架 backup.Create 的命名规则一致，保证能通过列表/恢复的文件名校验。
func uniqueBackupName(dir string) string {
	base := "backup_" + time.Now().Format("20060102_150405")
	target := base + ".db"
	for i := 1; ; i++ {
		if _, err := os.Stat(filepath.Join(dir, target)); os.IsNotExist(err) {
			return target
		}
		target = fmt.Sprintf("%s-%d.db", base, i)
	}
}
