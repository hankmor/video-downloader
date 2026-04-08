# 订阅扫描逻辑统一与 Context 管理重构计划

## 目标

统一 `core/subscription/manager.go` 和 `core/subscription/scan_manager.go` 中的扫描逻辑，消除代码冗余，确保所有扫描操作（包括 `yt-dlp` 进程）都通过 `context.Context` 进行管理，从而支持完美的取消操作和僵尸进程清理。

## 现状分析

1. **代码冗余**: `Manager.processSubscription` 和 `ScanManager.processSubscriptionWithOptimization` 包含大量重复的扫描、去重和任务创建逻辑。
2. **Context 缺失**: 原 `Manager.processSubscription` 内部使用 `context.Background()`，导致该方法启动的 `yt-dlp` 进程无法被外部取消。
3. **管理混乱**: 后台轮询 (`pollAll`) 和手动扫描 (`CheckNow`) 可能绕过 `ScanManager` 的状态管理。

## 重构计划

### 1. 重构 Manager (`core/subscription/manager.go`)

将核心扫描逻辑集中到 `Manager` 中，并强制要求传入 `Context`。

- **新增/修改方法**: `ProcessSubscription`
  - **签名**: `func (m *Manager) ProcessSubscription(ctx context.Context, sub *Subscription, onNewVideo func(entry *playlistEntry, i, total int)) (int, error)`
  - **职责**:
    - 接收外部传入的可取消 `Context`。
    - 执行 `fetchPlaylistEntries` (使用传入的 Context)。
    - 执行 LastVideoID 优化检查（从 `ScanManager` 迁移过来的逻辑）。
    - 遍历条目、去重、创建任务 (`createTaskFromEntry`)。
    - 通过 `onNewVideo` 回调通知进度的更新。
    - 返回新增视频数量。

- **清理**: 删除旧的 `processSubscription` 私有方法。

### 2. 重构 ScanManager (`core/subscription/scan_manager.go`)

`ScanManager` 将专注于任务调度、并发控制和状态通知，不再包含具体的扫描业务逻辑。

- **修改 `ScanOne`**:
  - 保持现有的 `context.WithCancel` 管理机制。
  - 移除内部对 `processSubscriptionWithOptimization` 的调用。
  - 改为调用 `m.manager.ProcessSubscription`。
  - 在 `ProcessSubscription` 的 `onNewVideo` 回调中执行：
    - 角标更新 (`DAO.IncrementBadgeCount`)。
    - 进度通知 (`notifyProgress`)。
    - 日志记录。
- **清理**: 删除 `ScanManager` 中的 `processSubscriptionWithOptimization` 方法。

### 3. 统一调用入口

确保所有触发扫描的地方都经过 `ScanManager`，以便统一管理状态和取消。

- **后台轮询 (`Manager.pollAll`)**:
  - 将原来的 `go m.processSubscription(sub)` 修改为调用 `m.scanManager.ScanOne(sub.ID)`。
  - **收益**: 后台自动扫描现在也会在 UI 上显示状态，并且可以被"停止所有"按钮一键取消。

- **立即检查 (`Manager.CheckNow`)**:
  - 确保调用 `m.scanManager.ScanOne(sub.ID)`。

## 兼容性与风险控制

- **现有功能**: 重构仅涉及内部实现，不改变对外的调用接口和业务行为。
- **取消机制**: 通过统一 Context 传递，确保 `CancelScan` 能真正终止底层的 `yt-dlp` 进程。
- **并发控制**: 依然由 `ScanManager` 的锁和 map 机制保证同一订阅不重复扫描。

## 执行步骤

1. 修改 `Manager.go`: 实现新的 `ProcessSubscription` 方法，迁移优化逻辑。
2. 修改 `ScanManager.go`: 更新 `ScanOne` 调用新方法，删除冗余代码。
3. 修改 `Manager.go`: 更新 `CheckNow` 和 `pollAll` 使用 `ScanManager`。
4. 验证: 编译通过，并验证添加、扫描、取消、删除等流程。
