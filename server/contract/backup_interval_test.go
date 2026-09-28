package contract

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"smallgo/server/scheduler"
	"smallgo/server/sysconfig"
)

// TestAutoBackupJobTakenOver 接管必须成立：框架注册的 auto_backup 应被改写成
// 5 分钟间隔任务（Daily 清空）；万一框架版本变化没找到同名任务，也必须注册
// 实例自己的任务兜底——总之存在一个 5 分钟一查的任务。
func TestAutoBackupJobTakenOver(t *testing.T) {
	var intervalJobs, dailyBackup int
	for _, j := range scheduler.All() {
		if j.Name == "auto_backup" && j.Daily != "" {
			dailyBackup++ // 仍是框架原样的每日任务 → 接管失败
		}
		if j.Interval == backupCheckInterval && j.Run != nil {
			intervalJobs++
		}
	}
	if dailyBackup != 0 {
		t.Fatalf("框架 auto_backup 仍是每日任务，接管未生效")
	}
	if intervalJobs != 1 {
		t.Fatalf("应恰好有一个 %v 间隔的备份任务，实际 %d", backupCheckInterval, intervalJobs)
	}
}

// TestBackupIntervalConfigRegistered 频率配置已注册且默认每天 03:00。
func TestBackupIntervalConfigRegistered(t *testing.T) {
	def, ok := sysconfig.GetConfigDef(backupIntervalKey)
	if !ok {
		t.Fatalf("配置 %s 未注册", backupIntervalKey)
	}
	if def.Default != "0" {
		t.Fatalf("默认值应为 0（每天 03:00），实际 %q", def.Default)
	}
	if err := sysconfig.Validate(def, "6"); err != nil {
		t.Fatalf("合法值 6 被拒: %v", err)
	}
	if err := sysconfig.Validate(def, "7"); err == nil {
		t.Fatal("非法值 7 应被拒绝")
	}
}

// TestBackupDueAt 每天模式的截止点是最近一个已过去的 03:00；
// 间隔模式是滚动窗口。
func TestBackupDueAt(t *testing.T) {
	loc := time.Local

	cases := []struct {
		name  string
		now   time.Time
		hours int
		want  time.Time
	}{
		{
			name:  "每天模式_已过03点_截止今天凌晨",
			now:   time.Date(2026, 3, 10, 14, 0, 0, 0, loc),
			hours: 0,
			want:  time.Date(2026, 3, 10, 3, 0, 0, 0, loc),
		},
		{
			name:  "每天模式_还没到03点_截止昨天凌晨",
			now:   time.Date(2026, 3, 10, 1, 0, 0, 0, loc),
			hours: 0,
			want:  time.Date(2026, 3, 9, 3, 0, 0, 0, loc),
		},
		{
			name:  "间隔模式_6小时滚动窗口",
			now:   time.Date(2026, 3, 10, 14, 0, 0, 0, loc),
			hours: 6,
			want:  time.Date(2026, 3, 10, 8, 0, 0, 0, loc),
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := backupDueAt(c.now, c.hours); !got.Equal(c.want) {
				t.Fatalf("got %v, want %v", got, c.want)
			}
		})
	}
}

// TestAutoBackupDue 没有备份 → 立即备份；有备份按最新文件时间判定点。
func TestAutoBackupDue(t *testing.T) {
	dir := t.TempDir()

	// 空目录：无论如何都该备份
	for _, hours := range []int{0, 1, 168} {
		due, err := autoBackupDue(dir, hours)
		if err != nil || !due {
			t.Fatalf("hours=%d 空目录应 due,err=%v,due=%v", hours, err, due)
		}
	}

	// 造一份"2 小时前"的备份文件（必须落在 dataDir/backups 下，名字符合
	// backup_YYYYMMDD_HHMMSS.db 规则，否则不会被 backup.List 看到）
	bakDir := filepath.Join(dir, "backups")
	if err := os.MkdirAll(bakDir, 0o755); err != nil {
		t.Fatal(err)
	}
	f := filepath.Join(bakDir, "backup_20260923_120000.db")
	if err := os.WriteFile(f, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-2 * time.Hour)
	if err := os.Chtimes(f, old, old); err != nil {
		t.Fatal(err)
	}

	check := func(hours int, want bool, msg string) {
		t.Helper()
		due, err := autoBackupDue(dir, hours)
		if err != nil {
			t.Fatalf("%s: err=%v", msg, err)
		}
		if due != want {
			t.Fatalf("%s: due=%v, want %v", msg, due, want)
		}
	}
	check(6, false, "2小时前的备份，6小时频率还没到点")
	check(1, true, "2小时前的备份，1小时频率已到点")
	check(168, false, "2小时前的备份，每7天频率没到点")
}
