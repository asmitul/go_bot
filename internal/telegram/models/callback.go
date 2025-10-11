package models

import (
	"fmt"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// 回调动作类型常量
const (
	CallbackActionAdminPage      = "admin_page"       // 管理员列表翻页
	CallbackActionConfirmDelete  = "confirm_delete"   // 确认删除对话框
	CallbackActionGroupSettings  = "group_settings"   // 群组设置面板
	CallbackActionWelcomeToggle  = "welcome_toggle"   // 切换欢迎消息开关
	CallbackActionApproveJoin    = "approve_join"     // 批准入群申请
	CallbackActionRejectJoin     = "reject_join"      // 拒绝入群申请
	CallbackActionPagination     = "pagination"       // 通用翻页
)

// CallbackLog 回调查询日志
type CallbackLog struct {
	ID              primitive.ObjectID `bson:"_id,omitempty"`
	CallbackQueryID string             `bson:"callback_query_id"`        // Telegram Callback Query ID（唯一）
	UserID          int64              `bson:"user_id"`                  // 操作用户 ID
	Username        string             `bson:"username,omitempty"`       // 操作用户名
	ChatID          int64              `bson:"chat_id,omitempty"`        // 消息所属聊天 ID
	MessageID       int64              `bson:"message_id,omitempty"`     // 消息 ID
	Data            string             `bson:"data"`                     // 原始 callback_data
	Action          string             `bson:"action"`                   // 解析后的操作类型
	Params          []string           `bson:"params,omitempty"`         // 解析后的参数列表
	Answered        bool               `bson:"answered"`                 // 是否已应答
	Error           string             `bson:"error,omitempty"`          // 处理错误信息
	ProcessingTime  int64              `bson:"processing_time,omitempty"` // 处理耗时（毫秒）
	CreatedAt       time.Time          `bson:"created_at"`               // 创建时间
}

// CallbackData 回调数据解析结果
type CallbackData struct {
	Action string   // 操作类型
	Params []string // 参数列表
}

// ParseCallbackData 解析 callback_data
// 格式：action:param1:param2:param3
// 示例：admin_page:2 → action=admin_page, params=[2]
// 示例：confirm_delete:user:123456 → action=confirm_delete, params=[user, 123456]
func ParseCallbackData(data string) (*CallbackData, error) {
	if data == "" {
		return nil, fmt.Errorf("callback data cannot be empty")
	}

	parts := strings.Split(data, ":")
	if len(parts) == 0 {
		return nil, fmt.Errorf("invalid callback data format")
	}

	action := parts[0]
	var params []string
	if len(parts) > 1 {
		params = parts[1:]
	}

	return &CallbackData{
		Action: action,
		Params: params,
	}, nil
}

// BuildCallbackData 构建 callback_data 字符串
// 示例：BuildCallbackData("admin_page", "2") → "admin_page:2"
func BuildCallbackData(action string, params ...string) string {
	if len(params) == 0 {
		return action
	}
	return action + ":" + strings.Join(params, ":")
}

// GetParam 获取指定索引的参数（安全）
func (c *CallbackData) GetParam(index int) string {
	if index < 0 || index >= len(c.Params) {
		return ""
	}
	return c.Params[index]
}

// HasParams 是否有参数
func (c *CallbackData) HasParams() bool {
	return len(c.Params) > 0
}

// ParamCount 参数数量
func (c *CallbackData) ParamCount() int {
	return len(c.Params)
}
