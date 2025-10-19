package features

import (
	"context"
	"sort"

	"go_bot/internal/logger"
	"go_bot/internal/telegram/service"
	botModels "github.com/go-telegram/bot/models"
)

// Manager 功能管理器
// 负责注册、管理和执行所有功能插件
type Manager struct {
	features     []Feature
	groupService service.GroupService
}

// NewManager 创建功能管理器
func NewManager(groupService service.GroupService) *Manager {
	return &Manager{
		features:     make([]Feature, 0),
		groupService: groupService,
	}
}

// Register 注册功能插件
// 功能会按优先级自动排序(优先级低的数字先执行)
func (m *Manager) Register(feature Feature) {
	m.features = append(m.features, feature)

	// 按优先级排序(Priority 越小越优先)
	sort.Slice(m.features, func(i, j int) bool {
		return m.features[i].Priority() < m.features[j].Priority()
	})

	logger.L().Infof("Registered feature: %s (priority: %d)", feature.Name(), feature.Priority())
}

// Process 处理消息
// 按优先级顺序执行所有已启用且匹配的功能
// 返回值:
//   - responseText: 响应文本(如果有功能处理)
//   - handled: 是否已被某个功能处理
//   - error: 处理过程中的错误
func (m *Manager) Process(ctx context.Context, msg *botModels.Message) (responseText string, handled bool, err error) {
	// 获取群组配置
	group, err := m.groupService.GetGroupInfo(ctx, msg.Chat.ID)
	if err != nil {
		// 群组不存在或获取失败,跳过功能处理
		logger.L().Debugf("Skip feature processing: group not found or error, chat_id=%d", msg.Chat.ID)
		return "", false, nil
	}

	// 按优先级顺序执行功能
	for _, feature := range m.features {
		// 1. 检查功能是否启用
		if !feature.Enabled(ctx, group) {
			logger.L().Debugf("Feature %s disabled, skipping", feature.Name())
			continue
		}

		// 2. 检查消息是否匹配
		if !feature.Match(ctx, msg) {
			continue
		}

		logger.L().Debugf("Feature %s matched message, processing...", feature.Name())

		// 3. 执行功能处理（传递 group 参数）
		response, handled, err := feature.Process(ctx, msg, group)

		// 4. 如果功能已处理(handled=true)或发生错误,停止后续功能执行
		if handled || err != nil {
			logger.L().Infof("Feature %s processed message (handled=%v, error=%v)", feature.Name(), handled, err)
			return response, handled, err
		}
	}

	// 没有任何功能处理该消息
	return "", false, nil
}

// ListFeatures 列出所有已注册的功能(用于调试)
func (m *Manager) ListFeatures() []string {
	names := make([]string, len(m.features))
	for i, f := range m.features {
		names[i] = f.Name()
	}
	return names
}
