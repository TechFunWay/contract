// Package contract 实现合同管家的业务模块：合同模板（富文本正文 + 自定义表单
// 字段）与合同（模板快照 + 填写值 + 服务端渲染结果）。
//
// 快照原则：合同创建时整体拷贝模板的正文与字段定义，此后模板的修改、删除均
// 不影响已生成的合同；打印页只读服务端渲染好的 rendered 快照。
package contract

import (
	"time"

	"smallgo/server/apps"
	"smallgo/server/database"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// ContentMaxBytes 限制模板正文（清洗前）与合同正文快照的大小，防止超大
// HTML 撑爆 SQLite 与前端编辑器。
const ContentMaxBytes = 512 << 10

// 合同状态枚举。
const (
	StatusDraft    = "draft"    // 草稿：可自由填写，不要求必填
	StatusActive   = "active"   // 生效
	StatusArchived = "archived" // 归档
)

// ContractTemplate 合同模板：正文富文本 HTML（含占位符）+ 字段定义 JSON。
type ContractTemplate struct {
	ID          uint      `gorm:"primarykey" json:"id"`
	UserID      uint      `gorm:"index;not null" json:"user_id"`
	Name        string    `gorm:"not null" json:"name"`
	Category    string    `gorm:"index;default:''" json:"category"`
	Description string    `json:"description"`
	Content     string    `gorm:"type:text" json:"content"`
	Fields      string    `gorm:"type:text" json:"-"`
	CreatedAt   time.Time `gorm:"index" json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// Contract 合同：模板时刻的快照 + 填写值 + 渲染结果。
type Contract struct {
	ID           uint       `gorm:"primarykey" json:"id"`
	UserID       uint       `gorm:"index;not null" json:"user_id"`
	TemplateID   uint       `gorm:"index" json:"template_id"`
	TemplateName string     `gorm:"not null" json:"template_name"`
	Title        string     `gorm:"not null" json:"title"`
	Content      string     `gorm:"type:text" json:"content"`
	Fields       string     `gorm:"type:text" json:"-"`
	Values       string     `gorm:"type:text" json:"-"`
	Rendered     string     `gorm:"type:text" json:"rendered"`
	Status       string     `gorm:"index;default:draft" json:"status"`
	Remark       string     `json:"remark"`
	SignDate     *time.Time `json:"sign_date"` // 签订日期（UTC 午夜）；nil 表示未签订
	CreatedAt    time.Time  `gorm:"index" json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

func init() {
	database.RegisterModels(&ContractTemplate{}, &Contract{})

	// 名下仍有模板或合同的用户不可删除，避免留下悬空的业务数据。
	database.RegisterUserDeleteGuard(func(db *gorm.DB, userID uint) (bool, error) {
		var templates, contracts int64
		if err := db.Model(&ContractTemplate{}).Where("user_id = ?", userID).Count(&templates).Error; err != nil {
			return false, err
		}
		if err := db.Model(&Contract{}).Where("user_id = ?", userID).Count(&contracts).Error; err != nil {
			return false, err
		}
		return templates+contracts > 0, nil
	})
	database.RegisterUserDeleteCleanup(func(db *gorm.DB, userID uint) error {
		if err := db.Where("user_id = ?", userID).Delete(&Contract{}).Error; err != nil {
			return err
		}
		return db.Where("user_id = ?", userID).Delete(&ContractTemplate{}).Error
	})

	apps.Register(apps.App{
		Name:        "contract",
		DisplayName: "合同管家",
		Icon:        "file-text",
		NavPosition: 10,
		SetupAuth:   setupRoutes,
		SetupAdmin:  setupAdminRoutes, // 备份恢复接口（见 restore.go）
	})
}

// setupRoutes 只挂业务路由，全部在 authGroup 下（登录可用 + 审计中间件），
// 数据按 user_id 隔离。
func setupRoutes(api *gin.RouterGroup, db *gorm.DB) {
	api.GET("/contract/templates", handleListTemplates(db))
	api.POST("/contract/templates", handleCreateTemplate(db))
	api.GET("/contract/templates/:id", handleGetTemplate(db))
	api.PUT("/contract/templates/:id", handleUpdateTemplate(db))
	api.DELETE("/contract/templates/:id", handleDeleteTemplate(db))
	api.POST("/contract/templates/:id/duplicate", handleDuplicateTemplate(db))

	api.GET("/contract/contracts", handleListContracts(db))
	api.POST("/contract/contracts", handleCreateContract(db))
	api.GET("/contract/contracts/:id", handleGetContract(db))
	api.PUT("/contract/contracts/:id", handleUpdateContract(db))
	api.DELETE("/contract/contracts/:id", handleDeleteContract(db))
	api.POST("/contract/contracts/:id/duplicate", handleDuplicateContract(db))
}
