package ui

import (
	"context"
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/mobile"
)

const (
	settingDeepSeekOnboardingSeen = "deepseek_onboarding_seen"
	settingsPromptFirstLaunch     = "first_launch"
	settingsPromptUnavailable     = "unavailable"
)

var deepSeekModelChoices = []string{
	"deepseek-flash",
	"deepseek-v4-pro",
	"deepseek-chat",
	"deepseek-reasoner",
}

func (a *App) renderSettingsModal(c *ui.Context) {
	pal := a.Palette()
	configured := a.Deps.AI != nil && a.Deps.AI.Configured()
	wasOpen := a.SettingsModalOpen

	subtitle := "填好后自动生效，之后就不用再管了"
	switch a.SettingsPromptReason {
	case settingsPromptFirstLaunch:
		subtitle = "填一次 API Key，AI 就能自动帮你整理灵感，之后就不用再管了"
	case settingsPromptUnavailable:
		subtitle = "检测到当前配置暂时不能用了，重新填一下就能恢复"
	}

	sheetBg := pal.Card
	mobile.Sheet(c, &a.SettingsModalOpen, mobile.SheetOptions{
		Detents:    []float32{0.76, 0.94},
		Background: &sheetBg,
		Label:      "DeepSeek 设置",
	}, func() {
		ui.Scroll(c).Fill().Children(func() {
			ui.Column(c).FillWidth().Padding(14, 18, 22, 18).Gap(14).Children(func() {
				RenderSheetHeader(c, pal, SheetHeaderProps{
					Icon:     IconSliders,
					Title:    "DeepSeek 设置",
					Subtitle: subtitle,
					ExtraBadge: func() {
						a.renderConnectionBadge(c, pal, configured)
					},
					CloseKey: "settings_close",
					OnClose: func() {
						a.SettingsModalOpen = false
						a.SettingsPromptReason = ""
						a.markSettingsOnboardingSeen()
					},
				})

				ui.Column(c).FillWidth().Gap(6).Children(func() {
					ui.Text(c, "服务地址").FontSize(FontMeta).FontWeight(600).TextColor(pal.InkSecondary)
					ui.TextInput(c, &a.SettingsBaseURL).
						Placeholder("https://api.deepseek.com").
						Label("服务地址").
						Background(pal.SoftPanel).
						FillWidth()
				})

				ui.Column(c).FillWidth().Gap(6).Children(func() {
					ui.Text(c, "API Key").FontSize(FontMeta).FontWeight(600).TextColor(pal.InkSecondary)
					ui.TextInput(c, &a.SettingsAPIKey).
						Password().
						Placeholder("sk-...").
						Label("API Key").
						Background(pal.SoftPanel).
						FillWidth()
				})

				ui.Column(c).FillWidth().Gap(6).Children(func() {
					ui.Text(c, "选用模型").FontSize(FontMeta).FontWeight(600).TextColor(pal.InkSecondary)
					modelOpts := make([]selectOption, 0, len(deepSeekModelChoices))
					for _, m := range deepSeekModelChoices {
						lbl := m
						if m == "deepseek-flash" {
							lbl = "deepseek-flash（推荐）"
						}
						modelOpts = append(modelOpts, selectOption{Value: m, Label: lbl})
					}
					ui.Row(c).FillWidth().Children(func() {
						atelierSelect(c, pal, "settings_model_select", "选用模型", &a.SettingsModel, modelOpts)
					})
				})

				if status := strings.TrimSpace(a.SettingsTestStatus); status != "" {
					isFail := strings.HasPrefix(status, "连接失败")
					stBg := pal.SoftPanel
					stBorder := pal.Border
					stFg := pal.Ink
					if isFail {
						stBg = c.Theme().Danger.Alpha(0.10)
						stBorder = c.Theme().Danger.Alpha(0.32)
						stFg = c.Theme().Danger
					}
					ui.Row(c).FillWidth().Padding(10, 12).Radius(RadiusControl).Gap(8).AlignItems(ui.Center).
						Background(stBg).Border(1, stBorder).
						Children(func() {
							ui.Text(c, status).FontSize(FontMeta).LineHeight(1.45).TextColor(stFg).Grow(1).MinWidth(0)
						})
				}

				ui.Row(c).FillWidth().Wrap().Justify(ui.End).Gap(8).AlignItems(ui.Center).Children(func() {
					if a.SettingsPromptReason == settingsPromptFirstLaunch {
						if GhostTextAction(c, "settings_skip", nil, "先跳过").Clicked() {
							mygo.Haptics.Selection()
							a.markSettingsOnboardingSeen()
							a.SettingsModalOpen = false
							a.SettingsPromptReason = ""
						}
					}
					if PillAction(c, "settings_test", IconRefresh, "测试连接", false).Clicked() {
						a.persistDeepSeekSettings()
						a.SettingsTestStatus = "正在测试连接…"
						a.RunAsync("正在测试连接…", func(ctx context.Context) error {
							msg, err := a.Deps.AI.TestConnection(ctx)
							a.Dispatch(func() {
								if err != nil {
									a.SettingsTestStatus = "连接失败：" + err.Error()
								} else {
									a.SettingsTestStatus = msg
								}
							})
							return nil
						})
					}
					if PillAction(c, "settings_save", IconCheck, "保存", true).Clicked() {
						mygo.Haptics.Success()
						a.persistDeepSeekSettings()
						a.markSettingsOnboardingSeen()
						a.SettingsModalOpen = false
						a.SettingsPromptReason = ""
						a.SettingsTestStatus = ""
						a.setBanner("设置已保存")
					}
				})
			})
		})
	})

	if wasOpen && !a.SettingsModalOpen {
		a.markSettingsOnboardingSeen()
		a.SettingsPromptReason = ""
	}
}

func (a *App) renderConnectionBadge(c *ui.Context, pal SurfacePalette, configured bool) {
	dotCol := pal.InkMuted
	bg := pal.SoftPanel
	fg := pal.InkSecondary
	text := "未填写 Key"
	if configured {
		dotCol = c.Theme().Success
		bg = c.Theme().Success.Alpha(0.14)
		fg = c.Theme().Success
		text = "已就绪"
	}
	ui.Row(c).Padding(3, 8).Gap(5).Radius(RadiusPill).AlignItems(ui.Center).Shrink(0).
		Background(bg).Children(func() {
		ui.Box(c).Size(6, 6).Radius(3).Background(dotCol).Shrink(0)
		ui.Text(c, text).FontSize(FontCaption).FontWeight(600).TextColor(fg).SingleLine()
	})
}

func (a *App) persistDeepSeekSettings() {
	if a.Deps.AI != nil {
		a.Deps.AI.UpdateCredentials(a.SettingsBaseURL, a.SettingsAPIKey, a.SettingsModel)
	}
	if a.Deps.Store != nil {
		_ = a.Deps.Store.SetSetting("deepseek_base_url", strings.TrimSpace(a.SettingsBaseURL))
		_ = a.Deps.Store.SetSetting("deepseek_api_key", strings.TrimSpace(a.SettingsAPIKey))
		_ = a.Deps.Store.SetSetting("deepseek_model", strings.TrimSpace(a.SettingsModel))
	}
}
