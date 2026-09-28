package contract

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"smallgo/server/audit"
	"smallgo/server/response"
	"smallgo/server/utils"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

var contractStatuses = map[string]bool{
	StatusDraft: true, StatusActive: true, StatusArchived: true,
}

// contractSummary 列表条目：不含正文/渲染大字段。
type contractSummary struct {
	ID           uint       `json:"id"`
	TemplateID   uint       `json:"template_id"`
	TemplateName string     `json:"template_name"`
	Title        string     `json:"title"`
	Status       string     `json:"status"`
	Remark       string     `json:"remark"`
	SignDate     *time.Time `json:"sign_date"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

// handleListContracts 分页返回合同列表，支持状态/模板/关键字筛选。
func handleListContracts(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		page := utils.Atoi(c.Query("page"), 1)
		pageSize := utils.Atoi(c.Query("pageSize"), 20)
		keyword := strings.TrimSpace(c.Query("keyword"))
		status := strings.TrimSpace(c.Query("status"))
		templateID := utils.Atoi(c.Query("template_id"), 0)
		uid := currentUserID(c)

		query := db.Model(&Contract{}).Where("user_id = ?", uid)
		if status != "" {
			if !contractStatuses[status] {
				response.ErrorBadRequest(c, "无效的状态筛选")
				return
			}
			query = query.Where("status = ?", status)
		}
		if templateID > 0 {
			query = query.Where("template_id = ?", templateID)
		}
		if keyword != "" {
			like := "%" + keyword + "%"
			query = query.Where("title LIKE ? OR template_name LIKE ? OR remark LIKE ?", like, like, like)
		}

		var total int64
		if err := query.Count(&total).Error; err != nil {
			response.ErrorInternal(c, "查询合同失败")
			return
		}

		items := make([]contractSummary, 0)
		if err := query.
			Select("id, template_id, template_name, title, status, remark, sign_date, created_at, updated_at").
			Order("id DESC").
			Offset(utils.Offset(page, pageSize)).
			Limit(pageSize).
			Scan(&items).Error; err != nil {
			response.ErrorInternal(c, "查询合同失败")
			return
		}

		response.SuccessPage(c, items, total, page, pageSize)
	}
}

type contractCreatePayload struct {
	TemplateID uint              `json:"template_id" binding:"required"`
	Title      string            `json:"title"`
	Values     map[string]string `json:"values"`
}

// handleCreateContract 从模板生成合同：拷贝正文与字段定义快照，按提交的
// 填写值（可空）渲染 rendered，状态置为草稿。
func handleCreateContract(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req contractCreatePayload
		if err := c.ShouldBindJSON(&req); err != nil {
			response.ErrorBadRequest(c, "请选择要使用的模板")
			return
		}

		var template ContractTemplate
		if err := db.Where("id = ? AND user_id = ?", req.TemplateID, currentUserID(c)).First(&template).Error; err != nil {
			response.ErrorNotFound(c, "模板不存在")
			return
		}

		fields, err := parseFields(template.Fields)
		if err != nil {
			response.ErrorInternal(c, "模板字段定义损坏")
			return
		}

		contract := Contract{
			UserID:       currentUserID(c),
			TemplateID:   template.ID,
			TemplateName: template.Name,
			Title:        strings.TrimSpace(req.Title),
			Content:      template.Content,
			Fields:       template.Fields, // 字段定义快照，与正文一起固化
			Status:       StatusDraft,
		}
		if contract.Title == "" {
			contract.Title = fmt.Sprintf("%s %s", template.Name, time.Now().Format("2006-01-02"))
		}
		if contract.Content == "" {
			response.ErrorBadRequest(c, "该模板没有正文，无法生成合同")
			return
		}

		values := cleanValues(req.Values)
		if contract.Values, err = marshalValues(values); err != nil {
			response.ErrorInternal(c, "创建合同失败")
			return
		}
		contract.Rendered = RenderContent(contract.Content, fields, values)

		if err := db.Create(&contract).Error; err != nil {
			response.ErrorInternal(c, "创建合同失败")
			return
		}
		audit.Log(db, c, "contract_create", "contract", contract.ID, "从模板生成合同 "+contract.Title)
		response.Success(c, contractDetail(&contract))
	}
}

type contractUpdatePayload struct {
	Title    *string            `json:"title"`
	Values   *map[string]string `json:"values"`
	Status   *string            `json:"status"`
	Remark   *string            `json:"remark"`
	SignDate *string            `json:"sign_date"` // YYYY-MM-DD；空串清除
}

// handleUpdateContract 更新标题/填写值/状态/备注/签订日期；values 变化时重渲染。
func handleUpdateContract(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.Atoi(c.Param("id"))
		if err != nil || id <= 0 {
			response.ErrorBadRequest(c, "无效的 ID")
			return
		}

		var contract Contract
		if err := db.Where("id = ? AND user_id = ?", id, currentUserID(c)).First(&contract).Error; err != nil {
			response.ErrorNotFound(c, "合同不存在")
			return
		}

		var req contractUpdatePayload
		if err := c.ShouldBindJSON(&req); err != nil {
			response.ErrorBadRequest(c, "请求参数不合法")
			return
		}

		if req.Title != nil {
			title := strings.TrimSpace(*req.Title)
			if title == "" {
				response.ErrorBadRequest(c, "合同标题不能为空")
				return
			}
			contract.Title = title
		}
		if req.Status != nil {
			status := strings.TrimSpace(*req.Status)
			if !contractStatuses[status] {
				response.ErrorBadRequest(c, "无效的合同状态")
				return
			}
			contract.Status = status
		}
		if req.Remark != nil {
			contract.Remark = strings.TrimSpace(*req.Remark)
		}
		if req.SignDate != nil {
			signDate, err := parseDate(*req.SignDate)
			if err != nil {
				response.ErrorBadRequest(c, "签订日期格式应为 YYYY-MM-DD")
				return
			}
			contract.SignDate = signDate
		}

		if req.Values != nil {
			fields, err := parseFields(contract.Fields)
			if err != nil {
				response.ErrorInternal(c, "合同字段定义损坏")
				return
			}
			values := cleanValues(*req.Values)
			if contract.Values, err = marshalValues(values); err != nil {
				response.ErrorInternal(c, "更新合同失败")
				return
			}
			contract.Rendered = RenderContent(contract.Content, fields, values)
		}

		if err := db.Save(&contract).Error; err != nil {
			response.ErrorInternal(c, "更新合同失败")
			return
		}
		audit.Log(db, c, "contract_update", "contract", contract.ID, "更新合同 "+contract.Title)
		response.Success(c, contractDetail(&contract))
	}
}

func handleGetContract(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.Atoi(c.Param("id"))
		if err != nil || id <= 0 {
			response.ErrorBadRequest(c, "无效的 ID")
			return
		}

		var contract Contract
		if err := db.Where("id = ? AND user_id = ?", id, currentUserID(c)).First(&contract).Error; err != nil {
			response.ErrorNotFound(c, "合同不存在")
			return
		}
		response.Success(c, contractDetail(&contract))
	}
}

func handleDeleteContract(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.Atoi(c.Param("id"))
		if err != nil || id <= 0 {
			response.ErrorBadRequest(c, "无效的 ID")
			return
		}

		var contract Contract
		if err := db.Where("id = ? AND user_id = ?", id, currentUserID(c)).First(&contract).Error; err != nil {
			response.ErrorNotFound(c, "合同不存在")
			return
		}
		if err := db.Delete(&contract).Error; err != nil {
			response.ErrorInternal(c, "删除合同失败")
			return
		}
		audit.Log(db, c, "contract_delete", "contract", contract.ID, "删除合同 "+contract.Title)
		response.Success(c, nil)
	}
}

// handleDuplicateContract 连值复制为草稿，便于换主体再签一份。
func handleDuplicateContract(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.Atoi(c.Param("id"))
		if err != nil || id <= 0 {
			response.ErrorBadRequest(c, "无效的 ID")
			return
		}

		var contract Contract
		if err := db.Where("id = ? AND user_id = ?", id, currentUserID(c)).First(&contract).Error; err != nil {
			response.ErrorNotFound(c, "合同不存在")
			return
		}

		copy := contract
		copy.ID = 0
		copy.CreatedAt = time.Now()
		copy.UpdatedAt = time.Now()
		copy.Title = contract.Title + "（副本）"
		copy.Status = StatusDraft
		if err := db.Create(&copy).Error; err != nil {
			response.ErrorInternal(c, "复制合同失败")
			return
		}
		audit.Log(db, c, "contract_duplicate", "contract", copy.ID, "复制合同 "+contract.Title)
		response.Success(c, contractDetail(&copy))
	}
}

// contractDetail 完整合同响应：fields/values 反序列化为结构化数组/对象。
func contractDetail(contract *Contract) gin.H {
	fields, _ := parseFields(contract.Fields)
	if fields == nil {
		fields = []Field{}
	}
	values := map[string]string{}
	if contract.Values != "" {
		_ = json.Unmarshal([]byte(contract.Values), &values)
	}
	return gin.H{
		"id":            contract.ID,
		"template_id":   contract.TemplateID,
		"template_name": contract.TemplateName,
		"title":         contract.Title,
		"content":       contract.Content,
		"fields":        fields,
		"values":        values,
		"rendered":      contract.Rendered,
		"status":        contract.Status,
		"remark":        contract.Remark,
		"sign_date":     contract.SignDate,
		"created_at":    contract.CreatedAt,
		"updated_at":    contract.UpdatedAt,
	}
}

// cleanValues 只保留非空键并去除首尾空白；键即字段 key，未知键不影响渲染
// （RenderContent 按 fields 的已知键集合判断）。
func cleanValues(values map[string]string) map[string]string {
	cleaned := make(map[string]string, len(values))
	for key, value := range values {
		if v := strings.TrimSpace(value); v != "" {
			cleaned[key] = v
		}
	}
	return cleaned
}

func marshalValues(values map[string]string) (string, error) {
	if len(values) == 0 {
		return "{}", nil
	}
	data, err := json.Marshal(values)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// parseDate 解析 YYYY-MM-DD；空串返回 nil（清除签订日期）。
func parseDate(s string) (*time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, nil
	}
	t, err := time.ParseInLocation("2006-01-02", s, time.UTC)
	if err != nil {
		return nil, err
	}
	return &t, nil
}
