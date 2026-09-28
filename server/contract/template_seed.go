package contract

import (
	"gorm.io/gorm"
)

// seedTemplateContent 示例模板正文：演示标题、编号占位符、甲乙方、金额与
// 日期字段的用法，用户可随意修改或删除。
const seedTemplateContent = `<h1 style="text-align: center">服务合同</h1>
<p style="text-align: center">合同编号：<span class="ct-field" data-field="contract_no" contenteditable="false">【合同编号】</span></p>
<p><span class="ct-field" data-field="party_a" contenteditable="false">【甲方名称】</span>（以下简称"甲方"）与 <span class="ct-field" data-field="party_b" contenteditable="false">【乙方名称】</span>（以下简称"乙方"），经友好协商，就甲方委托乙方提供相关服务事宜达成如下协议：</p>
<h3>一、服务内容</h3>
<p>乙方向甲方提供 <span class="ct-field" data-field="service_content" contenteditable="false">【服务内容】</span> 等服务，具体以双方确认的服务方案为准。</p>
<h3>二、合同金额与支付</h3>
<p>本合同总金额为人民币 <span class="ct-field" data-field="amount" contenteditable="false">【合同金额】</span> 元（大写：<span class="ct-field" data-field="amount_cn" contenteditable="false">【金额大写】</span>）。甲方应于 <span class="ct-field" data-field="pay_date" contenteditable="false">【付款日期】</span> 前支付至乙方指定账户。</p>
<h3>三、履行期限</h3>
<p>本合同履行期限自 <span class="ct-field" data-field="start_date" contenteditable="false">【开始日期】</span> 至 <span class="ct-field" data-field="end_date" contenteditable="false">【结束日期】</span> 止。</p>
<h3>四、其他</h3>
<p><span class="ct-field" data-field="extra_terms" contenteditable="false">【补充条款】</span></p>
<p>本合同一式两份，甲乙双方各执一份，自双方签字（盖章）之日起生效。</p>
<table>
<tbody>
<tr>
<td>甲方（盖章）：<span class="ct-blank"></span></td>
<td>乙方（盖章）：<span class="ct-blank"></span></td>
</tr>
<tr>
<td>签订日期：<span class="ct-field" data-field="sign_date_a" contenteditable="false">【甲方签订日期】</span></td>
<td>签订日期：<span class="ct-field" data-field="sign_date_b" contenteditable="false">【乙方签订日期】</span></td>
</tr>
</tbody>
</table>`

// seedTemplateFields 示例模板的字段定义，与正文占位符一一对应。
var seedTemplateFields = []Field{
	{Key: "contract_no", Label: "合同编号", Type: "text", Width: "half"},
	{Key: "party_a", Label: "甲方名称", Type: "text", Required: true, Width: "half"},
	{Key: "party_b", Label: "乙方名称", Type: "text", Required: true, Width: "half"},
	{Key: "service_content", Label: "服务内容", Type: "textarea", Width: "full"},
	{Key: "amount", Label: "合同金额（元）", Type: "number", Required: true, Width: "half"},
	{Key: "amount_cn", Label: "金额大写", Type: "text", Width: "half"},
	{Key: "pay_date", Label: "付款日期", Type: "date", Width: "half"},
	{Key: "start_date", Label: "开始日期", Type: "date", Width: "half"},
	{Key: "end_date", Label: "结束日期", Type: "date", Width: "half"},
	{Key: "extra_terms", Label: "补充条款", Type: "textarea", Width: "full"},
	{Key: "sign_date_a", Label: "甲方签订日期", Type: "date", Width: "half"},
	{Key: "sign_date_b", Label: "乙方签订日期", Type: "date", Width: "half"},
}

// ensureSeedTemplate 在用户没有任何模板时创建一份示例模板（每用户一次性）。
// 并发下极小概率重复创建，无业务影响（多一份可删的示例）。
func ensureSeedTemplate(db *gorm.DB, userID uint) error {
	fieldsJSON, err := marshalFields(seedTemplateFields)
	if err != nil {
		return err
	}
	seed := ContractTemplate{
		UserID:      userID,
		Name:        "通用服务合同（示例）",
		Category:    "示例",
		Description: "内置示例模板，演示正文占位符与字段定义的用法，可随意修改或删除。",
		Content:     seedTemplateContent,
		Fields:      fieldsJSON,
	}
	if err := db.Create(&seed).Error; err != nil {
		return err
	}
	return nil
}
