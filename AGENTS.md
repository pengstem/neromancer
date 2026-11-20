# AGENTS.md — Neromancer 工程宪章与 AI 协作规范

> [!IMPORTANT]
> **🔒 受控核心文档（人工必审 / Human Review Gate）**  
> 根据团队合作规范第二条：本文件直接决定所有 AI 编程助手如何理解架构与编写代码。  
> **任何对本文件、`README.md` 或 `docs/PRODUCT_CHARTER.md` 的修改，必须由人类团队成员通读全文并确认签字后方可合入或对外展示**；禁止把 AI 生成的文字未经审阅直接放行。  
> AI 助手在实现单个 Feature 过程中生成的临时“左脚踩右脚”计划或思考草稿，请写入 `docs/dev-plans/`（该目录免人工逐字审阅）。

---

## 1. 产品灵魂与不可妥协的红线

Neromancer 的本质是 **“Notion + ChatGPT for Random Sparks”**，聚焦 **手机/桌面随机灵感捕捉、AI 自主组织与按需孵化**。所有后续开发必须守住以下四条红线：

1. **少阻力（Zero-Friction Capture）**  
   - 任何新录入入口（主窗口右下角悬浮按钮、桌面置顶悬浮球、全局快捷键、未来手表端/移动端）必须在 1~2 次点击或一句话内完成落盘。
   - **禁止**要求用户先选文件夹、先写标题、先做排版。捕捉动作必须先本地毫秒级入库，再异步交给 AI 整理。
2. **低于平庸（Beyond Mediocrity）**  
   - **禁止**在用户刚记下一句半成品闪念时，直接用大模型百科全书式的平庸结论把原本生猛甚至“带点有趣错误”的念头覆盖掉。
   - AI 的首要职责是提炼**像朋友聊天一样好读的口语化摘要**与**启发式追问**，仅当用户主动点击「✨ AI 帮忙」时才展开分步可执行方案。
3. **原始证据神圣不可侵犯（Immutable Raw Evidence）**  
   - 当 AI 将多条同源闪念去重归并进同一个灵感节点时（经典案例：`买一盆郁金香放在阳台上养` + `郁金香的花瓶应该是蓝色的` → 合并为节点 `《用一个蓝色花瓶养一株郁金香》`），**只允许调整 `IdeaNode` 并更新 `RawEvidence.NodeID` 的归属外键**。
   - **严禁**修改、删除或覆盖 `raw_evidences` 表中的 `RawText`、`AudioPath`、`ImagePath`、`Scene` 与 `CapturedAt`。详情页右上角的「🗂️ 原始证据」永远可回溯到最初的语音、图片与时间场景。
4. **绝对克制（不做“美团里面的游戏”）**  
   - **禁止**添加论坛帖子流、评论区、点赞数、关注粉丝等重社交功能。
   - 分享能力只保留轻量、有品位的形态：高颜值记忆球 PNG 海报、`.orb.json` 灵感胶囊导入导出、以及只读的「他人灵感切片漫游 + 跨界碰撞」。

---

## 2. 技术栈与单向分层铁律（防止滑向“屎山”）

全项目坚持 **100% 纯 Go 核心逻辑 + `mygo v0.10.0` 安卓原生打包**：
- **GUI 渲染引擎**：`github.com/egoist/mygo`（`v0.10.0`）、`github.com/egoist/mygo/ui` 与 `github.com/egoist/mygo/ui/mobile`（Android OpenGL ES / 桌面 GPU 原生绘图与 Material 移动端组件）
- **本地存储**：纯 Go 原子写入 JSON Vault（`internal/store/vault.go`，零第三方 CGO/libc 依赖，Android/iOS/Desktop/WASM 全端通用，路径优先取 `mygo.PathUserData` 沙盒目录）
- **原生硬件桥接**：音频回放统一走 `github.com/egoist/mygo/audio`（Android `AudioTrack`）；安卓麦克风录音通过 `internal/voicerec` Kotlin 插件（16kHz 16-bit PCM WAV）；相机/相册通过 `github.com/egoist/mygo/plugins/imagepicker`
- **大模型服务**：统一对接 **DeepSeek API**（`POST /v1/chat/completions` 多模态文本/图像输入 + `POST /v1/tools/asr` 语音识别）

### 严格包依赖方向（禁止循环依赖与跨层越权）

```text
main.go
  └── internal/ui        (纯 Go 原生移动端 Material 视图、状态派发、Painter 绘图)
        └── internal/service   (业务编排：捕捉、去重归并、孵化、PNG 分享卡)
              ├── internal/store   (纯 Go 原子持久化 Vault 与种子数据)
              ├── internal/ai      (统一 DeepSeek HTTP 客户端与 Prompt 规范)
              ├── internal/media   (mygo/audio 回放 + internal/voicerec 安卓原生录音)
              └── internal/config  (.env 与运行时配置)
                    └── internal/domain (纯领域模型：IdeaNode / RawEvidence / StageSnapshot)
```

**编码硬约束：**
1. `internal/domain` 不得导入任何其他内部包。
2. `internal/ui` **不得**直接写 SQL 查询，**不得**直接发起 `http.Request` 调用 DeepSeek，所有动作必须经由 `internal/service` 或 `internal/media`。
3. `internal/service` **不得**导入 `internal/ui`（PNG 海报若需使用 `ui.Render`，通过 `ShareService.SetRasterizer(...)` 回调注入）。
4. 禁止编写重复逻辑：颜色主题统一调用 `ui.ColorsForTheme(...)`，记忆球绘制统一调用 `ui.DrawMemoryOrb(...)`，时间/场景格式化复用 `domain.SceneContext.SummaryLine()`。

---

## 3. `mygo/ui` 原生界面与并发安全规范

1. **禁止跨帧保留临时句柄**：`ui.Element` 与 `*ui.Context` 只在单帧 `view(c *ui.Context)` 构建期间有效，**绝对禁止**将其保存到 `App` 结构体字段或传入后台 goroutine。每次修改 UI 代码后必须运行 `go tool mygo vet` 检查生命周期安全。
2. **后台更新主线程安全**：所有网络请求（DeepSeek Chat / ASR）与文件弹窗（`mygo.Dialog`）必须在 goroutine 中执行，严禁阻塞 UI 主线程。完成后统一通过 `app.Dispatch(func() { ... })` 安全刷新状态与重绘。
3. **动态列表必须加 Key**：凡是在 `for` 循环中构建的卡片、按钮或证据条目，必须链式调用 `.Key("前缀_" + item.ID)`，防止滚动、焦点或点击状态串位。
4. **保持组件小而独立**：单个视图文件只负责一个清晰区域（参考 `showcase_view.go`、`mind_space_view.go`、`detail_panel.go`、`capture_fab.go`）。新增独立功能时优先在 `internal/ui/` 新增小文件并附带 `ui.NewTester` 测试用例。

---

## 4. 统一 DeepSeek API 与“说人话”文风约束

1. **禁止假 Mock AI**：不允许在运行时伪造假的大模型回复。如果未检测到 `DEEPSEEK_API_KEY`，`internal/ai.Client` 必须返回 `ai.ErrMissingAPIKey`，由界面弹出清晰引导前往「⚙️ DeepSeek 设置」。
2. **JSON Mode 必须带示例**：调用 DeepSeek `response_format: {"type": "json_object"}` 时，System Prompt 中**必须包含 `"json"` 字样与结构示例**，并在解析层保留空白输出检测与代码围栏剥离逻辑（见 `internal/ai/client.go`）。
3. **音频限制**：DeepSeek `/v1/tools/asr` 仅接受 16-bit PCM WAV 或 Ogg Opus、大小 `<= 8 MiB`、时长 `<= 120s`。
4. **口语化好读文风标准**（写入 `internal/ai/prompts.go`）：
   - ✅ 好示例：*“常听人说有模型学习和免模型学习，这两者究竟差在哪一步？一句话答案：差在智能体手里有没有转移概率。有就解方程或求和，没有就只能采样或求平均。”*
   - ❌ 坏示例：*“拿不到 PSMIDSA 也不许解方程，还能不能算出某个奇怪的符号？样本均值就是期望的估计。”*（严禁生造缩写、严禁公文 AI 腔）

---

## 5. 新增 Feature 与提交前验证标准流程

每位成员准备往仓库增加新功能时，请遵循：
1. **先看认领池**：在 [FEATURES_BACKLOG.md](docs/FEATURES_BACKLOG.md) 找到或新增带 Tag 的条目；重大架构改动先在群里拉齐确认，防止功能杂乱叠加。
2. **先写单测再改 UI**：核心逻辑放进 `internal/store` / `internal/ai` / `internal/service`，配好单元测试（AI 请求用 `net/http/httptest` 测试协议，无需真实烧额度）。
3. **端到端验证必过命令**（提交前全部绿灯）：
   ```bash
   go vet ./...
   go tool mygo vet
   go test -v -race ./...
   go build -o bin/neromancer .
   ```
4. **文档审阅红线**：如果你改动了 `README.md`、`AGENTS.md` 或 `docs/PRODUCT_CHARTER.md`，请务必自己在 PR / 群消息里逐字核对后再提交！
