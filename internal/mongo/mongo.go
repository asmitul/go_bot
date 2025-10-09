package mongo

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
)

// Client 封装 MongoDB 客户端及其配置
type Client struct {
	*mongo.Client
	dbName string
}

// Config 定义 MongoDB 连接配置
type Config struct {
	URI      string        // MongoDB 连接 URI，例如 "mongodb://localhost:27017"
	Database string        // 数据库名称
	Timeout  time.Duration // 连接超时时间
}

// NewClient 初始化 MongoDB 客户端
func NewClient(cfg Config) (*Client, error) {
	// 验证配置参数
	if cfg.URI == "" {
		return nil, fmt.Errorf("MongoDB URI cannot be empty")
	}
	if cfg.Database == "" {
		return nil, fmt.Errorf("database name cannot be empty")
	}

	// 设置客户端选项
	clientOptions := options.Client().ApplyURI(cfg.URI)

	// 设置默认超时时间（如果未提供）
	if cfg.Timeout == 0 {
		cfg.Timeout = 10 * time.Second
	}

	// 创建上下文，设置超时
	ctx, cancel := context.WithTimeout(context.Background(), cfg.Timeout)
	defer cancel()

	// 连接到 MongoDB
	client, err := mongo.Connect(ctx, clientOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to MongoDB: %w", err)
	}

	// 验证连接
	err = client.Ping(ctx, readpref.Primary())
	if err != nil {
		return nil, fmt.Errorf("failed to ping MongoDB: %w", err)
	}

	return &Client{
		Client: client,
		dbName: cfg.Database,
	}, nil
}

// Close 关闭 MongoDB 客户端连接
func (c *Client) Close(ctx context.Context) error {
	if c.Client == nil {
		return nil
	}
	return c.Client.Disconnect(ctx)
}

// Database 返回指定数据库的句柄
func (c *Client) Database() *mongo.Database {
	if c.Client == nil {
		return nil
	}
	return c.Client.Database(c.dbName)
}

// Ping 验证与 MongoDB 的连接
func (c *Client) Ping(ctx context.Context) error {
	if c.Client == nil {
		return fmt.Errorf("MongoDB client is not initialized")
	}
	return c.Client.Ping(ctx, readpref.Primary())
}
