# Neromancer 功能认领池（Feishu Backlog 同步模板）

> [!TIP]
> **对应团队合作规范第四条**：本清单设计为可直接粘贴进 **飞书共享文档 / 多维表格** 的任务池。  
> 每条 Feature 都带有清晰 Tag、当前状态与涉及文件范围：  
> - **觉得哪条实用且想做，直接把认领人改成自己，然后让你的 AI 助手按 [AGENTS.md](../AGENTS.md) 规范来实现。**  
> - **加入全新大功能前，建议先在群里和大家对齐，确认不违背“少阻力、克制不臃肿”原则再开工。**

---

## 一、已完成原型底座（可直接试玩与打磨）

| ID | 功能名称 | Tags | 状态 | 涉及代码 | 说明 |
| :--- | :--- | :--- | :--- | :--- | :--- |
| `F-01` | 安卓 Material 双层空间与变形悬浮按钮 | `[#visual-orb]` `[#mind-space]` | ✅ 原型已就绪 | `internal/ui/showcase_view.go`, `capture_fab.go` | 基于 `mygo v0.10.0 ui/mobile` 的底部栏 + 下滑/切换进入思维空间，右下角「🪶 记灵感」自动变形为「🧭 组织」 |
| `F-02` | 手机原生麦克风语音 + 相机/相册零阻力捕捉 | `[#capture]` `[#android]` | ✅ 原型已就绪 | `internal/voicerec/`, `internal/ui/capture_fab.go` | 安卓 Kotlin `AudioRecord` 直出 16kHz WAV 送 DeepSeek ASR，支持 `imagepicker` 拍照/选图与联想提醒 |
| `F-03` | Deep Link 快捷捕捉 + 桌面置顶悬浮球预览 | `[#capture]` `[#desktop-widget]` | ✅ 原型已就绪 | `main.go`, `internal/ui/floating_orb_view.go` | 安卓支持 `neromancer://capture` 与顶栏速记胶囊；桌面端保留置顶无边框悬浮球预览 |
| `F-04` | AI 去重归并 + 原始证据抽屉 & `mygo/audio` 回放 | `[#deepseek-ai]` `[#mind-space]` | ✅ 原型已就绪 | `internal/service/capture_service.go`, `detail_panel.go` | 实现郁金香双碎片合并与 `mobile.Sheet` 原始证据档案（支持 Android `AudioTrack` 原声回放） |
| `F-05` | 生命周期快照 + AI 帮忙落地方案 | `[#deepseek-ai]` `[#incubate]` | ✅ 原型已就绪 | `internal/service/incubate_service.go`, `detail_panel.go` | 阶段切换自动留快照，支持「低于平庸」追问与 Markdown 分步方案 |
| `F-06` | PNG 海报 + 安卓系统分享 + 只读漫游碰撞 | `[#share-roam]` `[#deepseek-ai]` | ✅ 原型已就绪 | `internal/service/share_card.go`, `roam_share_view.go` | 导出 PNG 海报调用 `mygo.Share` 系统分享面板，支持 `.orb.json` 胶囊与他人切片一键碰撞 |

---

## 二、待认领深化功能 Pool（欢迎 4 位成员自由认领）

| ID | Feature 名称 | Tags | 难度 | 认领人 | 涉及代码范围 | 验收标准（交给 AI 的 Prompt 目标） |
| :--- | :--- | :--- | :--- | :--- | :--- | :--- |
| `F-10` | **呼吸感记忆球动效与星图物理布局优化** | `[#visual-orb]` `[#ui-polish]` | ⭐⭐ | *待认领* | `internal/ui/theme.go`, `showcase_view.go`, `mind_space_view.go` | 利用 `ui.Element.Loop` / `Spring` 给展示页记忆球增加轻柔呼吸光晕，星图支持拖动节点和缩放，消除重叠 |
| `F-11` | **多模态图片与手绘板即时涂鸦** | `[#capture]` `[#visual-orb]` | ⭐⭐⭐ | *待认领* | `internal/ui/capture_fab.go`, `internal/media/` | 在捕捉弹层内增加一块轻量纯 Go 画板（`Pointer` 轨迹 -> 导出 PNG 存入原始证据），直接喂给 `deepseek-flash` 看图理解 |
| `F-12` | **语音波纹实时可视化与 Whisper/ASR 长录音分段** | `[#capture]` `[#deepseek-ai]` | ⭐⭐ | *待认领* | `internal/media/audio.go`, `internal/ai/client.go` | 录音时在 UI 绘制实时声波脉冲，自动检测/提示 8MiB/120s DeepSeek ASR 上限 |
| `F-13` | **主动式桌面“回访联想”轻量浮窗** | `[#recall]` `[#desktop-widget]` | ⭐⭐⭐ | *待认领* | `internal/service/organize_service.go`, `main.go` | 监听剪切板关键词或定时轻量提醒（可选开关），发现当前在聊的话题命中旧灵感时在托盘/悬浮球闪一下 |
| `F-14` | **睡眠孵化定时调度与“早安胶囊”通知** | `[#deepseek-ai]` `[#agent]` | ⭐⭐ | *待认领* | `internal/service/incubate_service.go`, `main.go` | 设定每晚固定时段或系统空闲时静默跑一轮 `SleepIncubate`，次日用 `mygo.NewNotification` 温柔提醒一次 |
| `F-15` | **目录树拖拽归并 & 父子二级目录编辑** | `[#mind-space]` `[#ux]` | ⭐⭐ | *待认领* | `internal/ui/mind_space_view.go`, `service/organize_service.go` | 支持在「思维空间」手动把两颗球拖到一起合并，或把碎片挂到主节点下作为二级子目录 |
| `F-16` | **多端协同 / 移动端 & 手表端语音按键入口** | `[#mobile-watch]` `[#infra]` | ⭐⭐⭐ | *待认领* | 新增 `cmd/mobile/` 或同步 HTTP API | 跟进 `egoist/mygo` 移动端合并分支，适配手机竖屏单手滑动与极简手表一键录音同步 |
| `F-17` | **项目规划书终稿打磨与演示视频录制** | `[#pitch-video]` `[#docs]` | ⭐ | *建议成员丙牵头* | `docs/PRODUCT_CHARTER.md`, `README.md` | 人工逐字通读并润色规划书，按“郁金香双记录合并 → 原始证据 → AI 帮忙 → 漫游碰撞”录制 3 分钟演示视频 |
