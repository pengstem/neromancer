package ui

import (
	"context"
	"fmt"
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/mobile"
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

	sheetBg := pal.Card
	mobile.Sheet(c, &a.SettingsModalOpen, mobile.SheetOptions{
		Detents:    []float32{0.76, 0.94},
		Background: &sheetBg,
		Label:      "DeepSeek 设置",
	}, func() {
		ui.Scroll(c).Fill().Children(func() {
			ui.Column(c).FillWidth().Padding(18).Gap(14).Children(func() {
				ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(10).Children(func() {
					ui.Column(c).Grow(1).MinWidth(0).Gap(5).Children(func() {
						ui.Row(c).Gap(8).AlignItems(ui.Center).Wrap().Children(func() {
							ui.Icon(c, IconSliders).Size(17, 17).TextColor(pal.Accent).Shrink(0)
							ui.Text(c, "DeepSeek 设置").FontSize(18).Bold().TextColor(pal.Ink)
							a.renderConnectionBadge(c, pal, configured)
						})
						ui.Text(c, "填好 API Key 即可使用语音转文字、自动整理与追问").
							FontSize(11.5).TextColor(pal.InkSecondary)
					})
					if GhostIconAction(c, "settings_close", IconClose, "关闭", false).Clicked() {
						a.SettingsModalOpen = false
					}
				})

				ui.Box(c).FillWidth().Height(1).Background(pal.Border)

				ui.Column(c).FillWidth().Gap(6).Children(func() {
					ui.Text(c, "服务地址").FontSize(12).FontWeight(600).TextColor(pal.InkSecondary)
					ui.TextInput(c, &a.SettingsBaseURL).
						Placeholder("https://api.deepseek.com").
						Label("服务地址").
						Background(pal.SoftPanel).
						FillWidth()
				})

				ui.Column(c).FillWidth().Gap(6).Children(func() {
					ui.Text(c, "API Key").FontSize(12).FontWeight(600).TextColor(pal.InkSecondary)
					ui.TextInput(c, &a.SettingsAPIKey).
						Password().
						Placeholder("sk-...").
						Label("API Key").
						Background(pal.SoftPanel).
						FillWidth()
				})

				ui.Column(c).FillWidth().Gap(6).Children(func() {
					ui.Text(c, "选用模型").FontSize(12).FontWeight(600).TextColor(pal.InkSecondary)
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
					ui.Row(c).FillWidth().Padding(10, 12).Radius(10).Gap(8).AlignItems(ui.Center).
						Background(stBg).Border(1, stBorder).
						Children(func() {
							ui.Text(c, status).FontSize(12).LineHeight(1.45).TextColor(stFg).Grow(1).MinWidth(0)
						})
				}

				ui.Row(c).FillWidth().Wrap().Justify(ui.End).Gap(8).AlignItems(ui.Center).Children(func() {
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
							return err
						})
					}
					if PillAction(c, "settings_save", IconCheck, "保存", true).Clicked() {
						mygo.Haptics.Success()
						a.persistDeepSeekSettings()
						a.SettingsModalOpen = false
						a.setBanner("设置已保存")
					}
				})
			})
		})
	})
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
	ui.Row(c).Padding(3, 8).Gap(5).Radius(999).AlignItems(ui.Center).Shrink(0).
		Background(bg).Children(func() {
		ui.Box(c).Size(6, 6).Radius(3).Background(dotCol).Shrink(0)
		ui.Text(c, text).FontSize(10.5).FontWeight(600).TextColor(fg).SingleLine()
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

func (a *App) renderSleepModal(c *ui.Context) {
	pal := a.Palette()

	sheetBg := pal.Card
	mobile.Sheet(c, &a.SleepModalOpen, mobile.SheetOptions{
		Detents:    []float32{0.76, 0.94},
		Background: &sheetBg,
		Label:      "灵感串联",
	}, func() {
		ui.Column(c).Grow(1).Basis(0).MinHeight(0).FillWidth().Padding(18).Gap(14).Children(func() {
			ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(10).Children(func() {
				ui.Row(c).Grow(1).MinWidth(0).Gap(8).AlignItems(ui.Center).Children(func() {
					ui.Icon(c, IconMoon).Size(17, 17).TextColor(pal.Accent).Shrink(0)
					ui.Text(c, "灵感串联").FontSize(18).Bold().TextColor(pal.Ink)
					if len(a.SleepInsights) > 0 {
						ui.Text(c, fmt.Sprintf("%d", len(a.SleepInsights))).
							FontSize(11).FontWeight(600).Padding(2, 7).Radius(999).
							Background(pal.SoftPanel).TextColor(pal.InkSecondary).Shrink(0)
					}
				})
				if GhostIconAction(c, "sleep_close", IconClose, "关闭", false).Clicked() {
					a.SleepModalOpen = false
				}
			})

			if PillAction(c, "sleep_wake", IconSpark, "找找灵感之间的联系", true).FillWidth().Clicked() {
				a.RunAsync("正在翻看你的灵感，寻找有趣的联系…", func(ctx context.Context) error {
					_, err := a.Deps.Incubate.RunSleepIncubation(ctx)
					return err
				})
			}

			ui.Box(c).FillWidth().Height(1).Background(pal.Border)

			ui.Scroll(c).Grow(1).Basis(0).MinHeight(0).FillWidth().Children(func() {
				ui.Column(c).FillWidth().Gap(10).Children(func() {
					if len(a.SleepInsights) == 0 {
						ui.Column(c).FillWidth().Padding(28).Center().Gap(8).Radius(14).
							Background(pal.SoftPanel).Border(1, pal.Border).
							Children(func() {
								ui.Icon(c, IconMoon).Size(20, 20).TextColor(pal.InkMuted)
								ui.Text(c, "暂时还没有新的串联发现，点上方按钮试试看").
									FontSize(13).TextColor(pal.InkSecondary).TextAlign(ui.Center)
							})
						return
					}
					for _, ins := range a.SleepInsights {
						item := ins
						ui.Column(c).Key("sleep_"+item.ID).FillWidth().Padding(15).Gap(10).Radius(14).
							Background(pal.SoftPanel).Border(1, pal.Border).
							Children(func() {
								ui.Text(c, item.Title).FontSize(15.5).Bold().LineHeight(1.35).TextColor(pal.Ink)
								ui.Text(c, item.SparkSummary).FontSize(13).LineHeight(1.58).TextColor(pal.InkSecondary)

								if strings.TrimSpace(item.SuggestedAction) != "" {
									ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Stretch).
										Padding(8, 10).Radius(10).Background(pal.Card).
										Children(func() {
											ui.Box(c).Width(2.5).Radius(2).Background(pal.Accent).Shrink(0)
											ui.Text(c, item.SuggestedAction).FontSize(12.5).LineHeight(1.48).
												FontWeight(500).TextColor(pal.Accent).Grow(1).MinWidth(0)
										})
								}

								ui.Row(c).FillWidth().Gap(8).Justify(ui.End).AlignItems(ui.Center).Children(func() {
									if len(item.ConnectedNodeIDs) > 0 {
										firstID := item.ConnectedNodeIDs[0]
										if PillAction(c, "jump_"+item.ID, IconChevronRight, "去看这条灵感", false).Clicked() {
											a.SleepModalOpen = false
											a.ActiveSpace = SpaceArchive
											a.openNodeDetail(c, firstID)
										}
									}
									if PillAction(c, "dis_"+item.ID, IconCheck, "收起", false).Clicked() {
										_ = a.Deps.Store.DismissSleepInsight(item.ID)
										_ = a.ReloadVault()
									}
								})
							})
					}
				})
			})
		})
	})
}
