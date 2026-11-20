# Neromancer

一款方便、好用的移动灵感记录 App：随手语音/图文速记，AI 自动归并整理，且完整保留原始记录。

## 技术栈

- **跨端原生 UI**：`github.com/egoist/mygo`（`v0.10.0`，Android OpenGL ES / Material 3 + 桌面竖屏预览）
- **本地存储与硬件**：纯 Go 原子持久化 Vault、`AudioRecord` 录音、`mygo/audio` 播放、`imagepicker` 相机/相册
- **AI 引擎**：DeepSeek API（多模态文本/图像理解 + `/v1/tools/asr` 语音转写）

## 快速开始

### 1. 配置环境

```bash
cp .env.example .env
# 在 .env 中填入 DEEPSEEK_API_KEY（也可启动后在右上角「DeepSeek 设置」中填写）

go tool mygo doctor
```

### 2. 开发预览

```bash
# 桌面直接打开 412×892 手机竖屏预览（支持热重载，无需 Android Studio）
go tool mygo dev

# 运行到真实安卓手机或模拟器
go tool mygo devices
go tool mygo dev -platform android
```

### 3. 打包安卓 APK

```bash
go tool mygo build -platform android
```

产物位于 `dist/android/`（含 `neromancer-0.1.0.apk` 与 `.aab`）。正式签名可在 [mygo.json](mygo.json) 配置 `keystore` / `keyAlias`。

### 4. 自动化测试

```bash
go vet ./...
go tool mygo vet
go test -race ./...
```

---

架构分层与开发规范见 [AGENTS.md](AGENTS.md)。
