package telegram

import (
	"context"
	"fmt"
	"time"

	"go_bot/internal/config"
	"go_bot/internal/logger"
	"go_bot/internal/telegram/models"
	"go_bot/internal/telegram/repository"
	"go_bot/internal/telegram/service"

	"github.com/go-telegram/bot"
	botModels "github.com/go-telegram/bot/models"
	"go.mongodb.org/mongo-driver/mongo"
)

// Config Telegram Bot 配置
type Config struct {
	Token    string  // Bot Token
	OwnerIDs []int64 // Owner 用户 IDs
	Debug    bool    // 是否开启调试模式
}

// Bot Telegram Bot 服务
type Bot struct {
	bot        *bot.Bot
	db         *mongo.Database
	ownerIDs   []int64
	workerPool *WorkerPool

	// Service 层（业务逻辑）
	userService     service.UserService
	groupService    service.GroupService
	messageService  service.MessageService
	callbackService service.CallbackService
	memberService   service.MemberService
	inlineService   service.InlineService
	pollService     service.PollService
	reactionService service.ReactionService

	// Repository 层（仅用于初始化）
	userRepo     repository.UserRepository
	groupRepo    repository.GroupRepository
	messageRepo  repository.MessageRepository
	callbackRepo repository.CallbackRepository
	memberRepo   repository.MemberRepository
	inlineRepo   repository.InlineRepository
	pollRepo     repository.PollRepository
	reactionRepo repository.ReactionRepository
}

// New 创建 Telegram Bot 实例
func New(cfg Config, db *mongo.Database) (*Bot, error) {
	// 验证配置
	if cfg.Token == "" {
		return nil, fmt.Errorf("telegram token cannot be empty")
	}

	// 创建 repositories
	userRepo := repository.NewMongoUserRepository(db)
	groupRepo := repository.NewMongoGroupRepository(db)
	messageRepo := repository.NewMongoMessageRepository(db)
	callbackRepo := repository.NewMongoCallbackRepository(db)
	memberRepo := repository.NewMongoMemberRepository(db)
	inlineRepo := repository.NewMongoInlineRepository(db)
	pollRepo := repository.NewMongoPollRepository(db)
	reactionRepo := repository.NewMongoReactionRepository(db)

	// 创建 services
	userService := service.NewUserService(userRepo)
	groupService := service.NewGroupService(groupRepo)
	messageService := service.NewMessageService(messageRepo)
	callbackService := service.NewCallbackService(callbackRepo)
	memberService := service.NewMemberService(memberRepo, groupRepo)
	inlineService := service.NewInlineService(inlineRepo)
	pollService := service.NewPollService(pollRepo)
	reactionService := service.NewReactionService(reactionRepo)

	// 创建 worker pool (10 workers, 100 queue size)
	workerPool := NewWorkerPool(10, 100)

	// 创建 bot 实例
	opts := []bot.Option{}
	if cfg.Debug {
		opts = append(opts, bot.WithDebug())
	}

	b, err := bot.New(cfg.Token, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create bot: %w", err)
	}

	telegramBot := &Bot{
		bot:             b,
		db:              db,
		ownerIDs:        cfg.OwnerIDs,
		workerPool:      workerPool,
		userService:     userService,
		groupService:    groupService,
		messageService:  messageService,
		callbackService: callbackService,
		memberService:   memberService,
		inlineService:   inlineService,
		pollService:     pollService,
		reactionService: reactionService,
		userRepo:        userRepo,
		groupRepo:       groupRepo,
		messageRepo:     messageRepo,
		callbackRepo:    callbackRepo,
		memberRepo:      memberRepo,
		inlineRepo:      inlineRepo,
		pollRepo:        pollRepo,
		reactionRepo:    reactionRepo,
	}

	// 初始化 owners
	if err := telegramBot.initOwners(context.Background()); err != nil {
		logger.L().Warnf("Failed to initialize owners: %v", err)
	}

	// 注册 handlers
	telegramBot.registerHandlers()

	// 初始化数据库索引
	if err := telegramBot.ensureIndexes(context.Background()); err != nil {
		return nil, fmt.Errorf("failed to ensure indexes: %w", err)
	}

	logger.L().Info("Telegram bot initialized successfully")
	return telegramBot, nil
}

// asyncHandler 异步 handler 包装器
// 将 handler 提交到 worker pool 异步执行
func (b *Bot) asyncHandler(handler bot.HandlerFunc) bot.HandlerFunc {
	return func(ctx context.Context, botInstance *bot.Bot, update *botModels.Update) {
		// 提交到 worker pool
		b.workerPool.Submit(HandlerTask{
			Ctx:         ctx,
			BotInstance: botInstance,
			Update:      update,
			Handler:     handler,
		})
	}
}

// InitFromConfig 从应用配置初始化 Telegram Bot
func InitFromConfig(cfg *config.Config, db *mongo.Database) (*Bot, error) {
	telegramCfg := Config{
		Token:    cfg.TelegramToken,
		OwnerIDs: cfg.BotOwnerIDs,
		Debug:    false, // 可根据需要从环境变量读取
	}
	return New(telegramCfg, db)
}

// Start 启动 Bot（阻塞式，应在 goroutine 中运行）
// 该方法会阻塞直到 context 被取消
func (b *Bot) Start(ctx context.Context) error {
	logger.L().Info("Starting Telegram bot...")
	b.bot.Start(ctx) // bot.Start() 是阻塞的，通过 context 取消来停止
	logger.L().Info("Telegram bot stopped gracefully")
	return nil
}

// Stop 停止 Bot
func (b *Bot) Stop(ctx context.Context) error {
	logger.L().Info("Stopping Telegram bot...")

	// 关闭 worker pool
	if b.workerPool != nil {
		b.workerPool.Shutdown()
	}

	// bot.Stop() 通过 context 取消实现
	return nil
}

// initOwners 初始化 owner 角色
func (b *Bot) initOwners(ctx context.Context) error {
	for _, ownerID := range b.ownerIDs {
		user, err := b.userRepo.GetByTelegramID(ctx, ownerID)
		if err != nil {
			// 用户不存在，创建 owner 记录
			user = &models.User{
				TelegramID:   ownerID,
				Role:         models.RoleOwner,
				CreatedAt:    time.Now().UTC(),
				UpdatedAt:    time.Now().UTC(),
				LastActiveAt: time.Now().UTC(),
			}
			if err := b.userRepo.CreateOrUpdate(ctx, user); err != nil {
				logger.L().Warnf("Failed to create owner %d: %v", ownerID, err)
				continue
			}
			logger.L().Infof("Initialized owner: %d", ownerID)
		} else if user.Role != models.RoleOwner {
			// 用户存在但角色不是 owner，更新为 owner
			user.Role = models.RoleOwner
			user.UpdatedAt = time.Now().UTC()
			if err := b.userRepo.CreateOrUpdate(ctx, user); err != nil {
				logger.L().Warnf("Failed to update owner role for %d: %v", ownerID, err)
				continue
			}
			logger.L().Infof("Updated user %d to owner", ownerID)
		}
	}
	return nil
}

// ensureIndexes 确保所有数据库索引存在
func (b *Bot) ensureIndexes(ctx context.Context) error {
	if err := b.userRepo.EnsureIndexes(ctx); err != nil {
		return fmt.Errorf("failed to ensure user indexes: %w", err)
	}
	logger.L().Debug("User indexes ensured")

	if err := b.groupRepo.EnsureIndexes(ctx); err != nil {
		return fmt.Errorf("failed to ensure group indexes: %w", err)
	}
	logger.L().Debug("Group indexes ensured")

	if err := b.messageRepo.EnsureIndexes(ctx); err != nil {
		return fmt.Errorf("failed to ensure message indexes: %w", err)
	}
	logger.L().Debug("Message indexes ensured")

	if err := b.callbackRepo.EnsureIndexes(ctx); err != nil {
		return fmt.Errorf("failed to ensure callback indexes: %w", err)
	}
	logger.L().Debug("Callback indexes ensured")

	if err := b.memberRepo.EnsureIndexes(ctx); err != nil {
		return fmt.Errorf("failed to ensure member indexes: %w", err)
	}
	logger.L().Debug("Member indexes ensured")

	if err := b.inlineRepo.EnsureIndexes(ctx); err != nil {
		return fmt.Errorf("failed to ensure inline indexes: %w", err)
	}
	logger.L().Debug("Inline indexes ensured")

	if err := b.pollRepo.EnsureIndexes(ctx); err != nil {
		return fmt.Errorf("failed to ensure poll indexes: %w", err)
	}
	logger.L().Debug("Poll indexes ensured")

	if err := b.reactionRepo.EnsureIndexes(ctx); err != nil {
		return fmt.Errorf("failed to ensure reaction indexes: %w", err)
	}
	logger.L().Debug("Reaction indexes ensured")

	return nil
}
