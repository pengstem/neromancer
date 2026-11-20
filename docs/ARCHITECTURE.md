# Neromancer 工程架构指南（10 分钟上手）

本文档面向所有参与 Neromancer 后续开发的同学，帮助你快速看懂现有骨架并安全地向里面添加新功能。

---

## 1. 整体结构与四层边界

```text
neromancer/
├── main.go                    # 进程入口：在 WhenReady 中解析 PathUserData 沙盒路径，启动手机竖屏主窗（桌面附带悬浮球/托盘）
├── main_test.go               # 基于 mygo/ui.NewTester (412×892 安卓竖屏) 的原生 UI 端到端无头测试
├── mygo.json                  # MyGo v0.10.0 安卓权限 (RECORD_AUDIO)、AdaptiveIcon、Deep Link (neromancer://) 配置
├── resources/                 # App 图标与 Android Adaptive Icon 前景图
├── internal/
│   ├── domain/models.go       # 【第1层】纯数据结构与不变量（IdeaNode / RawEvidence / StageSnapshot / RoamSlice）
│   ├── config/config.go       # 【第1层】.env + 环境变量 + 运行时目录管理
│   ├── store/                 # 【第2层】纯 Go 原子写入 Vault (vault.go)、联想检索、种子数据
│   ├── ai/                    # 【第2层】统一 DeepSeek HTTP 客户端（Chat 多模态 + /v1/tools/asr）与文风 Prompt
│   ├── voicerec/              # 【第2层】MyGo 安卓原生录音 Kotlin 插件（AudioRecord 16kHz WAV）
│   ├── media/                 # 【第2层】基于 mygo/audio (Android AudioTrack) 的音频回放与录音管理
│   ├── service/               # 【第3层】核心业务编排（捕捉入库、AI 去重合并、孵化、PNG 分享卡绘制）
│   └── ui/                    # 【第4层】模块化 mygo/ui/mobile Material 3 原生视图（Showcase / MindSpace / Detail Sheet / FAB）
└── docs/                      # 团队协作规范、产品底稿、飞书认领池
```

---

## 2. 数据模型设计关键点

核心思想是把 **“整理后的目录状态”** 和 **“用户当时的真实记录”** 彻底解耦：

1. **`IdeaNode`（记忆球 / 目录条目）**：存放 AI 或用户归纳后的标题、口语摘要、标签、权重（决定展示页球体半径）、颜色主题（`coral/sky/amber/mint/lavender`）、视觉纹样 `VisualMotif`、生命周期阶段（`spark → sprouting → incubating → crystallized`）以及可按需生成的 `ActionPlanMD`。
2. **`RawEvidence`（原始证据）**：通过 `NodeID` 关联到某个 `IdeaNode`。当多条碎片被合并进同一条灵感时，仅修改 `RawEvidence.NodeID`，其 `RawText`、`AudioPath`、`ImagePath`、`Scene`（时间/地点/心情/音乐）与 `CapturedAt` **永远保持原样**。
3. **`StageSnapshot`（阶段快照）**：每次推进阶段或经过 AI 归并，自动追加一条时间轴快照，方便回头追溯一个点子是怎么长出来的。

---

## 3. 如何添加一个新功能（Vibe Coding 标准姿势）

以“为详情页加一个新按钮”为例：
1. **数据层（如需要新字段）**：在 `internal/domain/models.go` 补字段，并在 `internal/store/sqlite.go` 同步建表/读写，跑通 `go test ./internal/store/...`。
2. **AI 能力（如需要 DeepSeek 参与）**：在 `internal/ai/prompts.go` 新增带 `"json"` 和格式示例的 System Prompt，在 `internal/ai/client.go` 新增方法，并用 `net/http/httptest` 在 `client_test.go` 验证请求和解析。
3. **业务编排**：在 `internal/service/` 暴露清晰的方法。
4. **UI 展示**：
   - 在 `internal/ui/` 对应视图中构建声明式 Go UI；
   - 耗时操作一律包裹在 `a.RunAsync("提示文案...", func(ctx context.Context) error { ... })` 中，不要在 `Clicked()` 分支直接同步发 HTTP 请求；
   - 在 `main_test.go` 用 `ui.NewTester` 模拟点击和文本断言。

---

## 4. 安卓原生运行与硬件插件说明（`egoist/mygo v0.10.0`）

本项目已全面基于 **`github.com/egoist/mygo v0.10.0`** 的移动端架构构建：
- **为什么不使用 CGO SQLite 库？** 安卓以 NDK 编译 `libmygoapp.so` 动态库并由 Gradle 打包；`internal/store/vault.go` 采用纯 Go 原子写入持久化，配合 `mygo.App.Path(mygo.PathUserData)` 落盘到手机私有沙盒，无需任何第三方 C 交叉编译依赖。
- **安卓录音与相册/相机**：`internal/voicerec` 通过 `mygo-plugin.json` 把 Kotlin `AudioRecord` 自动编入 APK（直出 16kHz 单声道 WAV 给 DeepSeek `/v1/tools/asr`）；图像输入与系统分享分别调用 `mygo/plugins/imagepicker` 与 `mygo.Share`。
- **一处代码，双端运行**：在手机上直接运行在 Android OpenGL ES + Material 3（`mobile.SetLook(mobile.Material)`）；在开发电脑上执行 `go tool mygo dev` 即可打开 `412×892` 竖屏模拟器预览窗口快速调试。
