// 自动备份频率：让备份间隔可配置（每天 03:00 / 每 N 小时）。
//
// 框架层（scheduler/backup/sysconfig 包）保持与 smallgo 基线一致、一行不改，
// 实现完全在实例层：
//
//   - 调度接管：backup 包在 init() 里注册了名为 auto_backup 的每日 03:00 任务；
//     scheduler.All() 返回的就是注册表本体切片，这里把同名任务原位改写成
//     5 分钟一查的间隔任务，框架的 runAutoBackup 不再执行。若哪天没找到同名
//     任务（框架版本变化），降级为注册一个自己的任务并告警，避免悄悄失灵。
//   - 频率配置：新增系统配置 backup_interval_hours（select）：
//     0 = 每天 03:00（与框架原语义一致），其余 = 每 N 小时滚动一次。
//     到点未备份则在下次检查（≤5 分钟）时补上，服务重启不丢节奏
//     （截止点由最新备份文件的时间推算，不依赖进程内计时器）。
//   - 总开关与保留数量沿用框架的 backup_auto_enabled / backup_keep_count，
//     界面徽章与「系统配置」里还是同一把开关。
//
// 备份的创建（backup.Create 的 VACUUM INTO）与清理（backup.Prune）直接复用
// 框架导出的能力。该接管建议后续上游到 smallgo 框架，成为家族通用能力。
package contract

import (
	"strconv"
	"time"

	"gorm.io/gorm"

	"smallgo/server/backup"
	"smallgo/server/config"
	"smallgo/server/logger"
	"smallgo/server/scheduler"
	"smallgo/server/sysconfig"
)

// backupIntervalKey 自动备份频率配置键（小时数字符串）。
const backupIntervalKey = "backup_interval_hours"

// backupCheckInterval 到点检查的粒度：配置只改存储，最迟 5 分钟内生效。
const backupCheckInterval = 5 * time.Minute

// dailyBackupHour 每天模式的备份时刻（本地时间，沿用框架的 03:00）。
const dailyBackupHour = 3

// autoBackupDB 运行时依赖：路由装配时写入（首个检查在启动 5 分钟后，无竞态）。
var autoBackupDB *gorm.DB

// autoBackupTakenOver init() 里的接管结果，延迟到 logger 就绪后在
// setAutoBackupDB 打日志（init 阶段 config/logger 尚未初始化，直接打会丢）。
var autoBackupTakenOver bool

func init() {
	sysconfig.RegisterConfig(sysconfig.ConfigDef{
		Key:     backupIntervalKey,
		Scope:   sysconfig.ScopeSystem,
		Type:    sysconfig.TypeSelect,
		Default: "0",
		Options: []string{"0", "1", "3", "6", "12", "24", "72", "168"},
		Group:   "backup",
		Label:   "自动备份频率",
		Description: "多久自动备份一次：0 为每天 03:00（原默认），" +
			"其余为每 N 小时（含每 3 天 72、每 7 天 168），到点未备会在 5 分钟内自动补上",
	})

	// 接管框架的 auto_backup 任务：同名原位改写为间隔任务。
	replaced := false
	jobs := scheduler.All()
	for i := range jobs {
		if jobs[i].Name == "auto_backup" {
			jobs[i] = scheduler.Job{
				Name:     "auto_backup",
				Interval: backupCheckInterval,
				Run:      runAutoBackupInterval,
			}
			replaced = true
		}
	}
	if replaced {
		autoBackupTakenOver = true
	} else {
		logger.Warn("scheduler: 未找到框架 auto_backup 任务，注册实例自带的备份任务")
		scheduler.Register(scheduler.Job{
			Name:     "contract_auto_backup",
			Interval: backupCheckInterval,
			Run:      runAutoBackupInterval,
		})
	}
}

// setAutoBackupDB 供路由装配注入运行时依赖（见 setupAdminRoutes），
// 顺带在 logger 就绪后报告接管结果。
func setAutoBackupDB(db *gorm.DB) {
	autoBackupDB = db
	if autoBackupTakenOver {
		logger.Info("scheduler: 已接管框架 auto_backup 任务，备份频率由 %s 控制", backupIntervalKey)
	} else {
		logger.Warn("scheduler: 未找到框架 auto_backup 任务，已注册实例自带的备份任务")
	}
}

// runAutoBackupInterval 每 5 分钟检查一次是否到了该备份的时间点。
func runAutoBackupInterval() {
	db := autoBackupDB
	dataDir := config.C.DataDir
	if db == nil || dataDir == "" {
		return
	}
	// 总开关沿用框架的 backup_auto_enabled（默认开启）。
	enabled := true
	if v, err := sysconfig.GetConfig(db, "backup_auto_enabled", 0); err == nil && v != "" {
		enabled = v != "false"
	}
	if !enabled {
		return
	}

	hours := backupIntervalHours(db)
	due, err := autoBackupDue(dataDir, hours)
	if err != nil {
		logger.Error("自动备份检查失败: %v", err)
		return
	}
	if !due {
		return
	}

	name, err := backup.Create(db, dataDir)
	if err != nil {
		logger.Error("自动备份失败: %v", err)
		return
	}
	if n, err := backup.Prune(dataDir, backupKeepCount(db)); err != nil {
		logger.Error("备份清理失败: %v", err)
	} else if n > 0 {
		logger.Info("自动备份 %s 完成，清理了 %d 个过期备份", name, n)
	} else {
		logger.Info("自动备份 %s 完成", name)
	}
}

// backupIntervalHours 读取频率配置（小时），非法值回退 0（每天 03:00）。
func backupIntervalHours(db *gorm.DB) int {
	v, err := sysconfig.GetConfig(db, backupIntervalKey, 0)
	if err != nil || v == "" {
		return 0
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 0 {
		return 0
	}
	return n
}

// backupKeepCount 读取保留数量（沿用框架键，非法值回退 7）。
func backupKeepCount(db *gorm.DB) int {
	v, err := sysconfig.GetConfig(db, "backup_keep_count", 0)
	if err != nil || v == "" {
		return 7
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 1 {
		return 7
	}
	return n
}

// backupDueAt 返回"该备份了"的时间截止点：最新备份早于它就该备份。
//   - hours > 0：滚动窗口 now-hours
//   - hours <= 0：最近一个已过去的 03:00（当前还没到 03:00 时取昨天的），
//     与框架原每日 03:00 语义一致
func backupDueAt(now time.Time, hours int) time.Time {
	if hours > 0 {
		return now.Add(-time.Duration(hours) * time.Hour)
	}
	t := time.Date(now.Year(), now.Month(), now.Day(), dailyBackupHour, 0, 0, 0, now.Location())
	if !now.After(t) {
		t = t.Add(-24 * time.Hour)
	}
	return t
}

// autoBackupDue 判断是否到了备份时间：没有任何备份 → 立即备份；
// 否则比较最新备份文件（VACUUM INTO 的创建时间即 mtime）与截止点。
func autoBackupDue(dataDir string, hours int) (bool, error) {
	items, err := backup.List(dataDir)
	if err != nil {
		return false, err
	}
	if len(items) == 0 {
		return true, nil
	}
	return items[0].CreatedAt.Before(backupDueAt(time.Now(), hours)), nil
}
