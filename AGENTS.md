# AGENTS.md — 架构与开发规范

## 1. 产品原则

1. **少阻力记录（Zero-Friction Capture）**：让用户在 1~2 次点击内随手记下语音、图文或短句，先毫秒级存入本地，再由 AI 在后台自动归类整理。
2. **启发式陪伴（Beyond Mediocrity）**：保留用户原始闪念的锋芒与个性，默认提供像朋友聊天一样的口语化摘要与启发式追问，并在用户点击「AI 帮忙」时按需生成可执行方案。
3. **原始记录原样留存（Authentic Raw Evidence）**：AI 归并多条同源碎片时，通过更新 `IdeaNode` 与 `RawEvidence.NodeID` 建立关联，始终完整保留 `RawEvidences` 中的 `RawText`、`AudioPath`、`ImagePath`、`Scene` 与 `CapturedAt`，让灵感随时可回溯最初的原话、录音、图片与场景。

## 2. 包分层与职责边界

```text
main.go
  └── internal/ui            (mygo/ui/mobile 原生视图、状态派发、Painter 绘图)
        └── internal/service       (业务编排：捕捉、去重归并、孵化、PNG 分享卡)
              ├── internal/store       (纯 Go 原子持久化 JSON Vault 与种子数据)
              ├── internal/ai          (DeepSeek Chat 多模态 + /v1/tools/asr 客户端与 Prompt)
              ├── internal/media       (mygo/audio 回放与跨端录音管理)
              │     └── internal/voicerec  (安卓 AudioRecord 16kHz WAV 录音插件)
              └── internal/config      (.env 与沙盒路径配置)
                    └── internal/domain    (纯领域模型：IdeaNode / RawEvidence / StageSnapshot)
```

1. `internal/domain` 保持零内部依赖，仅定义领域模型与纯方法。
2. 核心业务写操作（捕捉、归并、孵化、导入导出）统一收敛在 `internal/service`；`internal/ui` 专注视图渲染与状态派发，不直接操作本地文件或拼装 `net/http` 请求。
3. `internal/service` 保持对 `internal/ui` 的解耦（PNG 海报渲染通过 `ShareService.SetRasterizer` 回调注入）。
4. 复用公共实现：颜色主题用 `ui.ColorsForTheme`，记忆球绘制用 `ui.DrawMemoryOrb`，场景摘要用 `domain.SceneContext.SummaryLine`。

## 3. `mygo/ui` 与并发规范

1. **临时句柄单帧即用**：`ui.Element` 与 `*ui.Context` 仅在当前 `view(c)` 帧内构建使用，跨帧状态统一存放在 `App` 结构体字段中；改 UI 后运行 `go tool mygo vet` 校验。
2. **耗时任务后台执行**：网络请求与系统弹窗统一放入 `a.RunAsync` 后台执行，完成后通过 `app.Dispatch` 回到主线程刷新状态。
3. **动态列表固定 Key**：循环渲染的元素统一链式调用 `.Key("前缀_" + item.ID)`，保证滚动与点击状态稳定对应。
4. **视图文件职责单一**：新增独立区域拆分为 `internal/ui/` 下的小文件，并配合 `ui.NewTester` 补充无头 UI 测试。

## 4. DeepSeek API 与文案规范

1. **直连真实接口**：运行时直接请求真实 DeepSeek 接口；未配置 `DEEPSEEK_API_KEY` 时返回 `ai.ErrMissingAPIKey`，引导用户在「DeepSeek 设置」中填写。
2. **JSON Mode 带示例**：启用 `json_object` 时，System Prompt 包含 `"json"` 字样与字段示例，解析层保留代码围栏剥离与空值校验（见 `internal/ai/client.go`）。
3. **ASR 音频规格**：`/v1/tools/asr` 使用 16-bit PCM WAV 或 Ogg Opus、大小 `<= 8 MiB`、时长 `<= 120s`。
4. **全链路说人话**：无论是界面按钮、提示文案还是 AI 生成内容，一律用自然直白的口语表达，避免技术黑话与生造概念（Prompt 规范见 `internal/ai/prompts.go`）。

## 5. 验证与提交流程

1. 核心逻辑先用 `httptest` 与单元测试覆盖，提交前确保以下命令全部通过：

```bash
go vet ./...
go tool mygo vet
go test -race ./...
go build -o bin/neromancer .
```

2. **自动提交**：当进行相应的改动并通过上述验证之后，自行执行 `git commit` 提交对应变更。
