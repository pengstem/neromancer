# Neromancer — 随机灵感捕捉与自主孵化系统

> [!IMPORTANT]
> **人工审阅门禁（Human Review Gate）**  
> 本文件属于项目**对外展示性文档**。根据团队约定，在对外分享、提交大赛或更新内容前，请负责人在下方勾选确认已人工逐字通读：
> - [ ] 已核对产品定位与演示边界（无虚假功能夸大、未残留 AI 套话）
> - [ ] 已核对启动命令与运行前置条件（本机实测通过）
> - [ ] 已核对团队分工与文档链接指向正确

---

## 一句话理解 Neromancer

做一款 **“少阻力、低于平庸”** 的随机灵感记录与孵化原生应用——本质上是 **Notion（自主组织结构）+ ChatGPT（深度思维碰撞）**，但彻底去掉了传统备忘录“想标题、选目录、做排版”的记录阻力，也避免了一开始就把未成形想法丢给大模型换回千篇一律的“平庸正确废话”。

- **即出即记（少阻力）**：桌面常驻置顶悬浮球 + 全局快捷键（`Cmd/Ctrl+Shift+I`）+ 主界面右下角羽毛笔入口；先毫秒级把语音、短句、图片和当时场景（地点、心情、正在听的歌）原样落盘。
- **双层空间设计**：
  - **上层「展示页 · 记忆星海」**：拒绝手机端大段无组织纯文字刷屏，用《头脑特工队》风格的动态发光记忆球、场景标签与 AI 新火花提示直观浏览，支持输入关键词实时「回访联想」。
  - **下滑进入「思维空间」**：右下角按钮同步变形为「🧭 组织」，一键切换 **目录树 / 关联星图 / 场景轻卡片**。DeepSeek 会把同源碎片自动去重合并（例如把“买一盆郁金香”和“花瓶应该是蓝色的”自动归并为《用一个蓝色花瓶养一株郁金香》）。
- **右上角「🗂️ 原始证据」永久保真**：不管 AI 如何合并、重构摘要或推进生命周期，详情页右上角永远保留你当时最初的原录音、原图、原话和精确时间。
- **低于平庸的孵化**：不急着下结论，先用好读的口语摘要与启发式追问帮想法长出特色；当你真正想做出来时，再点击「✨ AI 帮忙」生成从第一步到成品的可执行方案，或者启用「🌙 睡眠孵化 Agent」在夜晚串联跨界暗线。
- **酷分享，不做论坛**：拒绝在工具软件里硬塞评论区和点赞流；支持导出高颜值记忆球 PNG 海报、`.orb.json` 便携胶囊，以及在只读「🌊 漫游切片」中看别人的灵感垃圾堆并与自己的想法一键碰撞。

---

## 技术选型

- **Android + 跨端原生 UI**：`github.com/egoist/mygo`（`v0.10.0`）+ `mygo/ui` + `mygo/ui/mobile`（Android OpenGL ES 原生 GPU 渲染 + Material 3 底部栏 / 抽屉 / 触觉反馈，桌面端同步打开 `412×892` 手机竖屏预览）。
- **本地存储**：`internal/store/vault.go` 纯 Go 原子存储（数据写入 `mygo.PathUserData` 沙盒目录，跨 Android / iOS / 桌面 / Web 零兼容障碍）。
- **原生硬件能力**：`internal/voicerec` 安卓 `AudioRecord` 16kHz WAV 录音插件、`mygo/audio` 原生播放、`mygo/plugins/imagepicker` 拍照/相册、`mygo.Share` 系统分享面板。
- **统一 AI 引擎**：直连真实 **DeepSeek API**（`deepseek-flash` 多模态文本与图片理解 + `/v1/tools/asr` 语音转写）。

---

## 快速开始（在哪里启动 & 如何打包成安卓 APK）

### 1. 环境检查与 DeepSeek Key 配置

```bash
cp .env.example .env
# 在 .env 中填入 DEEPSEEK_API_KEY=...（也可进入 App 后点右上角 ⚙️ 填写）

# 检查 Go 与 Android Studio / SDK / NDK 环境
go tool mygo doctor
```

### 2. 在哪里启动运行？（三种方式）

1. **直接在当前电脑预览安卓手机界面（无需装 Android Studio）**：
   ```bash
   go tool mygo dev
   # 会弹出 412×892 安卓 Material 竖屏开发预览窗口（改代码自动热重载）
   ```
   或直接运行编译好的桌面预览程序：
   ```bash
   go build -o bin/neromancer .
   ./bin/neromancer
   ```
2. **用浏览器直接预览（WASM 模式）**：
   ```bash
   go tool mygo dev -web
   ```
3. **运行到真实安卓手机或 Android 模拟器**（需先打开手机 USB 调试并连接 `adb`，或启动一个 Android Studio 模拟器）：
   ```bash
   go tool mygo devices                 # 查看已连接的安卓设备或模拟器
   go tool mygo dev -platform android   # 编译 Go 动态库 + Gradle 安装到手机运行
   ```

### 3. 如何打包成安卓 APK / AAB？

1. 安装 **Android Studio**，并在 `SDK Manager → SDK Tools` 中勾选安装 **NDK** 与 **Android SDK Command-line Tools**（`go tool mygo doctor` 会给出缺项提示）。
2. 在项目根目录执行打包命令：
   ```bash
   go tool mygo build -platform android
   ```
   构建完成后，会在 `dist/android/` 目录下生成可直接安装的 **`neromancer-0.1.0.apk`** 以及用于上架 Google Play 的 **`.aab`** 包。
   > 若需要打包正式签名版本，可在 [mygo.json](mygo.json) 的 `android` 字段配置 `keystore` 与 `keyAlias`，并设置环境变量 `MYGO_ANDROID_KEYSTORE_PASSWORD` / `MYGO_ANDROID_KEY_PASSWORD`。

### 4. 运行自动化测试

```bash
go vet ./...
go tool mygo vet
go test -count=1 -v -race ./...
```

---

## 仓库目录与协作入口

| 路径 | 用途 |
| :--- | :--- |
| [AGENTS.md](AGENTS.md) | 🔒 给团队每位成员和 AI 助手看的架构宪章与防“屎山”红线 |
| [docs/PRODUCT_CHARTER.md](docs/PRODUCT_CHARTER.md) | 🔒 产品规划书与演示视频脚本底稿（含郁金香完整 Demo 路径） |
| [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) | 📘 10 分钟上手架构指南：包分层、安卓原生插件与存储设计 |
| [docs/FEATURES_BACKLOG.md](docs/FEATURES_BACKLOG.md) | 📋 可同步飞书共享文档的带 Tag 功能认领池 |
| [docs/DOCUMENT_GOVERNANCE.md](docs/DOCUMENT_GOVERNANCE.md) | ⚖️ 团队合作规范四条制度化与好/坏设计自查清单 |
| `internal/domain` | 纯 Go 领域模型（`IdeaNode`、`RawEvidence`、`StageSnapshot` 等） |
| `internal/store` | 纯 Go 原子持久化 Vault 与首次启动演示数据 |
| `internal/ai` | 统一 DeepSeek API 客户端（Chat 多模态 + ASR）与口语化 Prompt |
| `internal/voicerec` | 安卓端原生 `AudioRecord` 16kHz PCM WAV 录音插件（Kotlin + Go） |
| `internal/media` | 基于 `mygo/audio` 与 `voicerec` 的跨端录音回放管理 |
| `internal/service` | 捕捉、去重组织、孵化、碰撞与 PNG 海报导出服务 |
| `internal/ui` | 模块化 `mygo/ui/mobile` 安卓竖屏适配界面与 `Painter` 记忆球 |
