package telegram

import (
	"context"
	"sync"

	"github.com/go-telegram/bot"
	botModels "github.com/go-telegram/bot/models"

	"go_bot/internal/logger"
)

// HandlerTask Handler 任务
type HandlerTask struct {
	Ctx         context.Context
	BotInstance *bot.Bot
	Update      *botModels.Update
	Handler     bot.HandlerFunc
}

// extractChatIDForError 从 update 中提取 chatID 用于错误消息发送
func extractChatIDForError(update *botModels.Update) int64 {
	switch {
	case update.Message != nil:
		return update.Message.Chat.ID
	case update.CallbackQuery != nil && update.CallbackQuery.Message.Message != nil:
		return update.CallbackQuery.Message.Message.Chat.ID
	case update.EditedMessage != nil:
		return update.EditedMessage.Chat.ID
	case update.ChannelPost != nil:
		return update.ChannelPost.Chat.ID
	case update.EditedChannelPost != nil:
		return update.EditedChannelPost.Chat.ID
	case update.MyChatMember != nil:
		return update.MyChatMember.Chat.ID
	case update.ChatMember != nil:
		return update.ChatMember.Chat.ID
	default:
		return 0
	}
}

// WorkerPool Handler 工作池
type WorkerPool struct {
	taskQueue chan HandlerTask
	wg        sync.WaitGroup
	workers   int
}

// NewWorkerPool 创建工作池
// workers: worker 协程数量
// queueSize: 任务队列大小
func NewWorkerPool(workers int, queueSize int) *WorkerPool {
	pool := &WorkerPool{
		taskQueue: make(chan HandlerTask, queueSize),
		workers:   workers,
	}

	// 启动 worker goroutines
	for i := 0; i < workers; i++ {
		pool.wg.Add(1)
		go pool.worker(i)
	}

	logger.L().Infof("Worker pool started with %d workers, queue size %d", workers, queueSize)
	return pool
}

// worker 工作协程
func (p *WorkerPool) worker(id int) {
	defer p.wg.Done()

	logger.L().Debugf("Worker %d started", id)

	for task := range p.taskQueue {
		// 执行 handler，带 panic recovery
		func() {
			defer func() {
				if r := recover(); r != nil {
					logger.L().Errorf("Worker %d: handler panic recovered: %v", id, r)

					// 尝试从不同类型的 update 中提取 chatID，发送错误消息给用户
					chatID := extractChatIDForError(task.Update)
					if chatID != 0 {
						_, _ = task.BotInstance.SendMessage(task.Ctx, &bot.SendMessageParams{
							ChatID: chatID,
							Text:   "❌ 服务器内部错误，请稍后重试",
						})
					}
				}
			}()

			// 执行实际的 handler
			task.Handler(task.Ctx, task.BotInstance, task.Update)
		}()
	}

	logger.L().Debugf("Worker %d stopped", id)
}

// Submit 提交任务到工作池
func (p *WorkerPool) Submit(task HandlerTask) {
	select {
	case p.taskQueue <- task:
		// 任务成功提交
	default:
		// 任务队列已满，记录详细的警告信息
		var chatID int64
		var messageID int
		if task.Update.Message != nil {
			chatID = task.Update.Message.Chat.ID
			messageID = task.Update.Message.ID
		} else if task.Update.CallbackQuery != nil && task.Update.CallbackQuery.Message.Message != nil {
			chatID = task.Update.CallbackQuery.Message.Message.Chat.ID
			messageID = task.Update.CallbackQuery.Message.Message.ID
		}

		logger.L().Warnf("Worker pool queue is full, task dropped: update_id=%d, chat_id=%d, message_id=%d",
			task.Update.ID, chatID, messageID)
	}
}

// Shutdown 优雅关闭工作池
// 等待所有正在执行的任务完成
func (p *WorkerPool) Shutdown() {
	logger.L().Info("Shutting down worker pool...")

	// 关闭任务队列，不再接受新任务
	close(p.taskQueue)

	// 等待所有 worker 完成
	p.wg.Wait()

	logger.L().Info("Worker pool shut down successfully")
}
