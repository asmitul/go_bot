package telegram

import (
	"context"
	"fmt"
	"time"

	"go_bot/internal/config"
	"go_bot/internal/logger"
	"go_bot/internal/telegram/models"
	"go_bot/internal/telegram/repository"

	"github.com/go-telegram/bot"
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
	bot       *bot.Bot
	db        *mongo.Database
	ownerIDs  []int64
	userRepo  *repository.UserRepository
	groupRepo *repository.GroupRepository
}

// New 创建 Telegram Bot 实例
func New(cfg Config, db *mongo.Database) (*Bot, error) {
	// 验证配置
	if cfg.Token == "" {
		return nil, fmt.Errorf("telegram token cannot be empty")
	}

	// 创建 repositories
	userRepo := repository.NewUserRepository(db)
	groupRepo := repository.NewGroupRepository(db)

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
		bot:       b,
		db:        db,
		ownerIDs:  cfg.OwnerIDs,
		userRepo:  userRepo,
		groupRepo: groupRepo,
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
func (b *Bot) Start(ctx context.Context) error {
	logger.L().Info("Starting Telegram bot...")
	b.bot.Start(ctx)
	logger.L().Info("Telegram bot stopped")
	return nil
}

// Stop 停止 Bot
func (b *Bot) Stop(ctx context.Context) error {
	logger.L().Info("Stopping Telegram bot...")
	// bot.Stop() 通过 context 取消实现，这里只是记录日志
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
				CreatedAt:    time.Now(),
				UpdatedAt:    time.Now(),
				LastActiveAt: time.Now(),
			}
			if err := b.userRepo.CreateOrUpdate(ctx, user); err != nil {
				logger.L().Warnf("Failed to create owner %d: %v", ownerID, err)
				continue
			}
			logger.L().Infof("Initialized owner: %d", ownerID)
		} else if user.Role != models.RoleOwner {
			// 用户存在但角色不是 owner，更新为 owner
			user.Role = models.RoleOwner
			user.UpdatedAt = time.Now()
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

	return nil
}
