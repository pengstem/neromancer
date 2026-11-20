package ui

import (
	"context"
	"fmt"
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/pengstem/neromancer/internal/domain"
	"github.com/pengstem/neromancer/internal/service"
)

// FloatingOrbView renders the frameless Always-On-Top desktop companion orb window.
func (a *App) FloatingOrbView(c *ui.Context) {
	a.FlushPending()
	pal := a.Palette()
	phase := a.Phase(c)

	ui.Themed(c, NeromancerThemeFor(a.DarkMode)).Fill().Children(func() {
		if !a.FloatExpanded {
			ui.Column(c).Fill().Center().Padding(8).
				Background(ui.Transparent).
				Children(func() {
					ui.ButtonBase(c).Size(76, 76).Radius(38).DragWindow().
						Tooltip("拖动移位 · 轻触展开悬浮速记").
						Label("悬浮速记").
						Cursor(ui.CursorPointer).
						Draw(func(p *ui.Painter, r ui.Rect) {
							DrawAnimatedMemoryOrb(p, r, domain.ThemeCoral, 0.92, phase, 0.85, "petals", true)
						}).
						OnClick(func() {
							a.FloatExpanded = true
							if a.FloatWin != nil {
								a.FloatWin.SetSize(360, 250)
							}
						})
				})
			return
		}

		ui.Column(c).Fill().Padding(14).Gap(10).Radius(18).
			Background(pal.Card).Border(1, pal.Border).
			Children(func() {
				ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Center).DragWindow().Children(func() {
					ui.Box(c).Size(22, 22).Shrink(0).Draw(func(p *ui.Painter, r ui.Rect) {
						DrawAnimatedMemoryOrb(p, r, domain.ThemeCoral, 0.9, phase, 0.78, "petals", false)
					})
					ui.Text(c, "悬浮速记").FontSize(13).Bold().TextColor(pal.Ink).Grow(1)
					if PillAction(c, "float_main_win", IconOrb, "主窗口", false).Clicked() && a.MainWin != nil {
						a.MainWin.Show()
						a.MainWin.Focus()
					}
					if PillAction(c, "float_collapse", IconChevronDown, "收起", false).Clicked() {
						a.FloatExpanded = false
						if a.FloatWin != nil {
							a.FloatWin.SetSize(96, 96)
						}
					}
				})

				ui.Box(c).FillWidth().Grow(1).Padding(10, 12).Radius(12).
					Background(pal.SoftPanel).Border(1, pal.Border.Alpha(0.75)).
					Children(func() {
						ui.TextAreaBase(c, &a.FloatQuickText).
							Placeholder("想起什么直接记下…").
							PlaceholderColor(pal.InkMuted).
							TextColor(pal.Ink).
							Label("悬浮速记输入").
							Fill().AutoFocus()
					})

				ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Center).Children(func() {
					if a.Deps.Media != nil && a.Deps.Media.IsRecording() {
						if PillAction(c, "float_stop_rec", IconStop, "停录", true).Clicked() {
							path, _, err := a.Deps.Media.StopRecording()
							if err == nil {
								a.submitFloatingCapture(path)
							} else {
								a.setErrorBanner(err.Error())
							}
						}
					} else {
						if PillAction(c, "float_start_rec", IconMic, "语音", false).Clicked() && a.Deps.Media != nil {
							if _, err := a.Deps.Media.StartRecording(); err != nil {
								a.setErrorBanner(err.Error())
							}
						}
					}

					ui.Spacer(c).Grow(1)

					if PillAction(c, "float_submit", IconCheck, "收入星海", true).Clicked() {
						a.submitFloatingCapture("")
					}
				})
			})
	})
}

func (a *App) submitFloatingCapture(audioPath string) {
	text := strings.TrimSpace(a.FloatQuickText)
	if text == "" && audioPath == "" {
		return
	}
	a.FloatQuickText = ""
	a.FloatExpanded = false
	if a.FloatWin != nil {
		a.FloatWin.SetSize(96, 96)
	}

	modality := "text"
	if audioPath != "" {
		modality = "voice"
	}

	localOut, err := a.Deps.Capture.CaptureQuick(context.Background(), service.QuickCaptureInput{
		RawText:      text,
		Modality:     modality,
		AudioSrcPath: audioPath,
		AutoOrganize: false,
	})
	if err != nil {
		a.Dispatch(func() { a.setErrorBanner(err.Error()) })
		return
	}
	a.Dispatch(func() {
		_ = a.ReloadVault()
		a.SelectNode(localOut.Node.ID)
		a.FocusedShowcaseID = localOut.Node.ID
		a.setBanner(fmt.Sprintf("已收进记忆星海：《%s》", localOut.Node.Title))
	})

	if a.Deps.AI != nil && a.Deps.AI.Configured() {
		ev := localOut.Evidence
		nodeID := localOut.Node.ID
		a.RunAsync("DeepSeek 正在整理悬浮速记的灵感…", func(ctx context.Context) error {
			organized, _, err := a.Deps.Capture.OrganizeEvidenceIntoCatalog(ctx, ev, nodeID)
			if err != nil {
				return err
			}
			a.Dispatch(func() {
				a.SelectNode(organized.ID)
				a.FocusedShowcaseID = organized.ID
				a.setBanner(fmt.Sprintf("悬浮灵感已归并亮起：《%s》", organized.Title))
			})
			return nil
		})
	}
}
