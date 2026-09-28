package contract

import (
	"strconv"
	"strings"
	"time"

	"smallgo/server/audit"
	"smallgo/server/response"
	"smallgo/server/utils"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type templatePayload struct {
	Name        string  `json:"name" binding:"required"`
	Category    string  `json:"category"`
	Description string  `json:"description"`
	Content     string  `json:"content"`
	Fields      []Field `json:"fields"`
}

// templateSummary 列表条目：不含正文大字段，带字段数与关联合同数。
type templateSummary struct {
	ID            uint      `json:"id"`
	Name          string    `json:"name"`
	Category      string    `json:"category"`
	Description   string    `json:"description"`
	Fields        string    `json:"-"`
	FieldCount    int       `json:"field_count"`
	ContractCount int64     `json:"contract_count"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

func currentUserID(c *gin.Context) uint {
	return c.GetUint("userID")
}

// handleListTemplates 分页返回模板列表；该用户还没有任何模板且未带筛选时，
// 自动创建一份示例模板（每用户一次性）。
func handleListTemplates(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		page := utils.Atoi(c.Query("page"), 1)
		pageSize := utils.Atoi(c.Query("pageSize"), 20)
		keyword := strings.TrimSpace(c.Query("keyword"))
		category := strings.TrimSpace(c.Query("category"))
		uid := currentUserID(c)

		query := db.Model(&ContractTemplate{}).Where("user_id = ?", uid)
		if keyword != "" {
			like := "%" + keyword + "%"
			query = query.Where("name LIKE ? OR description LIKE ? OR category LIKE ?", like, like, like)
		}
		if category != "" {
			query = query.Where("category = ?", category)
		}

		var total int64
		if err := query.Count(&total).Error; err != nil {
			response.ErrorInternal(c, "查询模板失败")
			return
		}
		if total == 0 && keyword == "" && category == "" {
			if err := ensureSeedTemplate(db, uid); err != nil {
				response.ErrorInternal(c, "初始化示例模板失败")
				return
			}
			if err := query.Count(&total).Error; err != nil {
				response.ErrorInternal(c, "查询模板失败")
				return
			}
		}

		items := make([]templateSummary, 0)
		if err := query.
			Select("contract_templates.id, contract_templates.name, contract_templates.category, " +
				"contract_templates.description, contract_templates.fields, contract_templates.created_at, " +
				"contract_templates.updated_at, (SELECT COUNT(*) FROM contracts AS cc WHERE cc.template_id = contract_templates.id) AS contract_count").
			Order("contract_templates.id DESC").
			Offset(utils.Offset(page, pageSize)).
			Limit(pageSize).
			Scan(&items).Error; err != nil {
			response.ErrorInternal(c, "查询模板失败")
			return
		}
		for i := range items {
			if fields, err := parseFields(items[i].Fields); err == nil {
				items[i].FieldCount = len(fields)
			}
			items[i].Fields = ""
		}

		response.SuccessPage(c, items, total, page, pageSize)
	}
}

func handleCreateTemplate(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req templatePayload
		if err := c.ShouldBindJSON(&req); err != nil {
			response.ErrorBadRequest(c, "模板名称不能为空")
			return
		}

		template := &ContractTemplate{UserID: currentUserID(c)}
		created, ok := saveTemplate(db, c, template, &req)
		if !ok {
			return // 响应已写出
		}
		audit.Log(db, c, "contract_template_create", "contract_template", created.ID, "创建模板 "+created.Name)
		response.Success(c, templateDetail(created))
	}
}

func handleUpdateTemplate(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.Atoi(c.Param("id"))
		if err != nil || id <= 0 {
			response.ErrorBadRequest(c, "无效的 ID")
			return
		}

		var template ContractTemplate
		if err := db.Where("id = ? AND user_id = ?", id, currentUserID(c)).First(&template).Error; err != nil {
			response.ErrorNotFound(c, "模板不存在")
			return
		}

		var req templatePayload
		if err := c.ShouldBindJSON(&req); err != nil {
			response.ErrorBadRequest(c, "模板名称不能为空")
			return
		}

		updated, ok := saveTemplate(db, c, &template, &req)
		if !ok {
			return
		}
		audit.Log(db, c, "contract_template_update", "contract_template", template.ID, "更新模板 "+template.Name)
		response.Success(c, templateDetail(updated))
	}
}

// saveTemplate 校验载荷并落库（create 与 update 共用）。校验失败时已写出响应。
func saveTemplate(db *gorm.DB, c *gin.Context, template *ContractTemplate, req *templatePayload) (*ContractTemplate, bool) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		response.ErrorBadRequest(c, "模板名称不能为空")
		return nil, false
	}
	if len(req.Content) > ContentMaxBytes {
		response.ErrorBadRequest(c, "正文过长（上限 512KB）")
		return nil, false
	}

	content, err := SanitizeHTML(req.Content)
	if err != nil {
		response.ErrorBadRequest(c, "正文格式无法解析")
		return nil, false
	}

	fields, err := normalizeFields(req.Fields)
	if err != nil {
		response.ErrorBadRequest(c, errInvalidFields.Error())
		return nil, false
	}
	fieldsJSON, err := marshalFields(fields)
	if err != nil {
		response.ErrorInternal(c, "保存模板失败")
		return nil, false
	}

	template.Name = name
	template.Category = strings.TrimSpace(req.Category)
	template.Description = strings.TrimSpace(req.Description)
	template.Content = content
	template.Fields = fieldsJSON

	if err := db.Save(template).Error; err != nil {
		response.ErrorInternal(c, "保存模板失败")
		return nil, false
	}
	return template, true
}

func handleGetTemplate(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.Atoi(c.Param("id"))
		if err != nil || id <= 0 {
			response.ErrorBadRequest(c, "无效的 ID")
			return
		}

		var template ContractTemplate
		if err := db.Where("id = ? AND user_id = ?", id, currentUserID(c)).First(&template).Error; err != nil {
			response.ErrorNotFound(c, "模板不存在")
			return
		}
		response.Success(c, templateDetail(&template))
	}
}

func handleDeleteTemplate(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.Atoi(c.Param("id"))
		if err != nil || id <= 0 {
			response.ErrorBadRequest(c, "无效的 ID")
			return
		}

		var template ContractTemplate
		if err := db.Where("id = ? AND user_id = ?", id, currentUserID(c)).First(&template).Error; err != nil {
			response.ErrorNotFound(c, "模板不存在")
			return
		}
		if err := db.Delete(&template).Error; err != nil {
			response.ErrorInternal(c, "删除模板失败")
			return
		}
		// 已生成的合同持有快照，随模板删除一并保留，不级联删除。
		audit.Log(db, c, "contract_template_delete", "contract_template", template.ID, "删除模板 "+template.Name)
		response.Success(c, nil)
	}
}

func handleDuplicateTemplate(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.Atoi(c.Param("id"))
		if err != nil || id <= 0 {
			response.ErrorBadRequest(c, "无效的 ID")
			return
		}

		var template ContractTemplate
		if err := db.Where("id = ? AND user_id = ?", id, currentUserID(c)).First(&template).Error; err != nil {
			response.ErrorNotFound(c, "模板不存在")
			return
		}

		copy := template
		copy.ID = 0
		copy.CreatedAt = time.Now()
		copy.UpdatedAt = time.Now()
		copy.Name = template.Name + "（副本）"
		if err := db.Create(&copy).Error; err != nil {
			response.ErrorInternal(c, "复制模板失败")
			return
		}
		audit.Log(db, c, "contract_template_duplicate", "contract_template", copy.ID, "复制模板 "+template.Name)
		response.Success(c, templateDetail(&copy))
	}
}

// templateDetail 补充 fields 数组后的完整模板响应。
func templateDetail(template *ContractTemplate) gin.H {
	fields, _ := parseFields(template.Fields)
	if fields == nil {
		fields = []Field{}
	}
	return gin.H{
		"id":          template.ID,
		"name":        template.Name,
		"category":    template.Category,
		"description": template.Description,
		"content":     template.Content,
		"fields":      fields,
		"created_at":  template.CreatedAt,
		"updated_at":  template.UpdatedAt,
	}
}
