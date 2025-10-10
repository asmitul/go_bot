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
					// 可选：发送错误消息给用户
					if task.Update.Message != nil {
						_, _ = task.BotInstance.SendMessage(task.Ctx, &bot.SendMessageParams{
							ChatID: task.Update.Message.Chat.ID,
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
		// 任务队列已满，记录警告
		logger.L().Warnf("Worker pool queue is full, task dropped")
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
