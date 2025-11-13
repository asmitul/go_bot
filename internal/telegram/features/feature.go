package features

import (
	"context"

	botModels "github.com/go-telegram/bot/models"
	"go_bot/internal/telegram/features/types"
	"go_bot/internal/telegram/models"
)

// Feature 消息处理功能插件接口
//
// 每个功能插件需实现此接口,例如:
// - 计算器: 检测数学表达式并计算结果
// - 汇率查询: 检测价格指令并返回行情
// - 天气查询: 检测 "天气 城市名" 并返回天气信息
// - AI 对话: 匹配任意文本并调用 AI API
type Feature interface {
	// Name 返回功能名称(用于日志和调试)
	Name() string

	// Enabled 检查功能是否启用
	// 参数:
	//   - ctx: 上下文
	//   - group: 群组信息(包含配置)
	// 返回:
	//   - true: 功能已启用
	//   - false: 功能已禁用,跳过处理
	Enabled(ctx context.Context, group *models.Group) bool

	// Match 检查消息是否匹配该功能
	// 参数:
	//   - ctx: 上下文
	//   - msg: Telegram 消息
	// 返回:
	//   - true: 消息匹配,应该由该功能处理
	//   - false: 消息不匹配,跳过
	//
	// 示例:
	//   - 计算器: 检测是否为数学表达式 "1+1"
	//   - 行情: 检测是否匹配指定价格指令
	Match(ctx context.Context, msg *botModels.Message) bool

	// Process 处理消息并返回响应
	// 参数:
	//   - ctx: 上下文
	//   - msg: Telegram 消息
	//   - group: 群组信息（包含配置）
	// 返回:
	//   - response: 响应内容（包含文本和可选的 ReplyMarkup），用于发送给用户
	//   - handled: 是否已完成处理(true 则停止后续功能的执行)
	//   - error: 处理过程中的错误
	//
	// 返回值说明:
	//   - (response, true, nil): 成功处理,发送响应,停止后续功能
	//   - (nil, false, nil): 不处理,继续执行下一个功能
	//   - (response, true, err): 处理失败,可选地发送错误消息,并停止后续功能
	Process(ctx context.Context, msg *botModels.Message, group *models.Group) (response *types.Response, handled bool, err error)

	// Priority 返回功能优先级(1-100)
	// 数值越小优先级越高,功能按优先级顺序执行
	//
	// 推荐优先级:
	//   - 1-20: 高优先级功能(如计算器、命令解析)
	//   - 21-50: 中优先级功能(如行情查询、天气查询)
	//   - 51-100: 低优先级功能(如 AI 对话、关键词回复)
	//
	// 优先级排序原因:
	//   - 避免低优先级功能抢占高优先级功能的消息
	//   - 例如: AI 对话(优先级 90)应在计算器(优先级 20)之后执行
	Priority() int
}

// Response 类型别名用于兼容旧引用，实际定义位于 types 包。
type Response = types.Response

// TierAwareFeature 可选接口：实现后可限制功能适用的群组等级
type TierAwareFeature interface {
	AllowedGroupTiers() []models.GroupTier
}
