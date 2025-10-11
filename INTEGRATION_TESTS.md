# 集成测试说明

本项目为 Stage 3 新增的 Repository 层创建了完整的集成测试。

## 测试文件

- `internal/telegram/repository/inline_test.go` - 内联查询 Repository 测试（9 个测试用例）
- `internal/telegram/repository/poll_test.go` - 投票 Repository 测试（11 个测试用例）
- `internal/telegram/repository/reaction_test.go` - 反应 Repository 测试（11 个测试用例）

**总计：31 个集成测试用例**

## 测试覆盖范围

### InlineRepository (9 tests)
- ✅ LogQuery - 记录内联查询
- ✅ LogChosenResult - 记录内联结果选择
- ✅ GetPopularQueries - 获取热门查询（聚合查询）
- ✅ GetUserQueries - 获取用户查询历史
- ✅ GetUserQueries with limit - 查询历史限制
- ✅ EnsureIndexes - 索引创建
- ✅ EnsureIndexes idempotency - 重复调用索引创建

### PollRepository (11 tests)
- ✅ CreatePoll - 创建投票
- ✅ UpdatePoll - 更新投票状态
- ✅ RecordAnswer - 记录投票回答
- ✅ RecordAnswer upsert - 更新用户回答（upsert）
- ✅ GetPollAnswers - 获取投票回答
- ✅ GetUserPolls - 获取用户创建的投票
- ✅ GetUserPolls with limit - 投票列表限制
- ✅ GetPollByID not found - 查询不存在的投票
- ✅ UpdatePoll not found - 更新不存在的投票
- ✅ CreateQuizPoll - 创建测验类型投票
- ✅ EnsureIndexes - 索引创建

### ReactionRepository (11 tests)
- ✅ RecordReaction - 记录消息反应
- ✅ RecordReaction upsert - 更新用户反应（upsert）
- ✅ UpdateReactionCount - 更新反应统计
- ✅ GetMessageReactions - 获取消息反应
- ✅ GetTopReactedMessages - 获取反应最多的消息（排序查询）
- ✅ GetReactionCount not found - 查询不存在的统计
- ✅ RecordReaction with empty reactions - 记录空反应列表
- ✅ Multiple reaction types - 多种反应类型
- ✅ EnsureIndexes - 索引创建

## 运行前提

集成测试需要本地 MongoDB 服务运行：

```bash
# 使用 Docker 启动 MongoDB（推荐）
docker run -d -p 27017:27017 --name test-mongo mongo:latest

# 或使用已安装的 MongoDB 服务
mongod --dbpath /path/to/data
```

## 运行方法

### 运行所有集成测试
```bash
go test -tags=integration -v ./internal/telegram/repository/
```

### 运行特定 Repository 的测试
```bash
# Inline Repository
go test -tags=integration -v ./internal/telegram/repository/ -run TestInlineRepository

# Poll Repository
go test -tags=integration -v ./internal/telegram/repository/ -run TestPollRepository

# Reaction Repository
go test -tags=integration -v ./internal/telegram/repository/ -run TestReactionRepository
```

### 运行单个测试用例
```bash
go test -tags=integration -v ./internal/telegram/repository/ -run TestInlineRepository_LogQuery
```

### 查看测试覆盖率
```bash
go test -tags=integration -coverprofile=coverage.out ./internal/telegram/repository/
go tool cover -html=coverage.out
```

## CI/CD 集成

集成测试已配置在 GitHub Actions CI 流程中（`.github/workflows/ci.yml`）：

```yaml
- name: Run integration tests
  run: go test -tags=integration -v -race ./...
  env:
    MONGO_URI: mongodb://localhost:27017
```

CI 会自动启动 MongoDB service container 并运行所有集成测试。

## 测试数据库管理

每个测试会：
1. 创建临时测试数据库（格式：`test_<repo>_<timestamp>`）
2. 执行测试操作
3. 测试结束后自动删除数据库（cleanup）

测试数据库相互隔离，不会影响生产数据。

## 验证编译

如果没有 MongoDB 服务，可以验证测试代码编译正确：

```bash
go test -tags=integration -c ./internal/telegram/repository/
```

编译成功说明测试代码语法正确。

## 测试设计原则

1. **独立性** - 每个测试用例独立运行，不依赖其他测试
2. **清理** - 使用 defer cleanup() 确保资源释放
3. **隔离性** - 每个测试使用独立的临时数据库
4. **完整性** - 测试所有 Repository 方法和边界情况
5. **真实性** - 使用真实 MongoDB 而非 mock，测试实际数据库交互

## 故障排查

### 连接超时错误
```
server selection timeout, current topology: { Type: Unknown ... }
```
**解决方案**：确保 MongoDB 服务运行在 `localhost:27017`

### 权限错误
```
not authorized on admin to execute command
```
**解决方案**：使用无认证的测试 MongoDB 或配置正确的认证信息

### 索引冲突错误
```
E11000 duplicate key error
```
**解决方案**：测试数据库名称冲突，重新运行测试（会生成新的时间戳）
