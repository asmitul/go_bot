package forward

import (
	"sync"
	"time"

	"go_bot/internal/logger"

	botModels "github.com/go-telegram/bot/models"
)

// MediaGroupBuffer 媒体组缓冲区
type MediaGroupBuffer struct {
	Messages []*botModels.Message
	Timer    *time.Timer
	Mutex    sync.Mutex
}

// MediaGroupCollector 媒体组收集器
type MediaGroupCollector struct {
	buffers   map[string]*MediaGroupBuffer
	mutex     sync.RWMutex
	timeout   time.Duration
	onCollect func(messages []*botModels.Message)
}

// NewMediaGroupCollector 创建媒体组收集器
func NewMediaGroupCollector(timeout time.Duration, onCollect func([]*botModels.Message)) *MediaGroupCollector {
	return &MediaGroupCollector{
		buffers:   make(map[string]*MediaGroupBuffer),
		timeout:   timeout,
		onCollect: onCollect,
	}
}

// Add 添加消息到收集器
func (c *MediaGroupCollector) Add(message *botModels.Message) {
	mediaGroupID := message.MediaGroupID

	c.mutex.Lock()
	buffer, exists := c.buffers[mediaGroupID]
	if !exists {
		buffer = &MediaGroupBuffer{
			Messages: make([]*botModels.Message, 0),
		}
		c.buffers[mediaGroupID] = buffer
		logger.L().Debugf("Created new media group buffer: media_group_id=%s", mediaGroupID)
	}
	c.mutex.Unlock()

	buffer.Mutex.Lock()
	buffer.Messages = append(buffer.Messages, message)
	logger.L().Debugf("Added message to media group: media_group_id=%s, total_messages=%d", mediaGroupID, len(buffer.Messages))

	// 重置定时器
	if buffer.Timer != nil {
		buffer.Timer.Stop()
	}
	buffer.Timer = time.AfterFunc(c.timeout, func() {
		c.collect(mediaGroupID)
	})
	buffer.Mutex.Unlock()
}

// collect 收集并处理媒体组
func (c *MediaGroupCollector) collect(mediaGroupID string) {
	c.mutex.Lock()
	buffer, exists := c.buffers[mediaGroupID]
	if !exists {
		c.mutex.Unlock()
		return
	}
	delete(c.buffers, mediaGroupID)
	c.mutex.Unlock()

	buffer.Mutex.Lock()
	messages := buffer.Messages
	buffer.Mutex.Unlock()

	if len(messages) > 0 {
		logger.L().Infof("Media group collection completed: media_group_id=%s, message_count=%d", mediaGroupID, len(messages))
		c.onCollect(messages)
	}
}
