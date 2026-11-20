package ui

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/mobile"
	"github.com/pengstem/neromancer/internal/domain"
)

func (a *App) renderShowcaseView(c *ui.Context) {
	sc := ui.Scroll(c).Key("space_showcase").TrackScroll(&a.ShowcaseScroll).Fill().
		HandleInput(func(ev ui.InputEvent) bool {
			// Pull up past bottom transitions into Mind Space.
			if ev.Kind == ui.InputScroll && ev.DY > 28 && a.ShowcaseScroll.Y >= a.ShowcaseScroll.MaxY-8 {
				a.ActiveSpace = SpaceMind
				return true
			}
			return false
		})

	if sc.PullToRefresh(&a.ShowcaseRefresh) {
		mygo.Haptics.Light()
		a.RunAsync("正在换一条新启发…", func(ctx context.Context) error {
			defer a.Dispatch(func() { a.ShowcaseRefresh.Refreshing = false })
			if a.Deps.AI != nil && a.Deps.AI.Configured() {
				if spark, err := a.Deps.Incubate.SuggestShowcaseSpark(ctx); err == nil && spark != "" {
					a.Dispatch(func() { a.ShowcaseSpark = spark })
				}
			}
			return nil
		})
	}

	sc.Children(func() {
		padX := float32(16)
		if !a.isPhoneLayout(c) {
			padX = 28
		}
		ui.Column(c).FillWidth().Padding(12, padX, 112, padX).Gap(14).Children(func() {
			a.renderShowcaseHeaderAndRecall(c)
			a.renderTagFilterBar(c)
			a.renderMemoryOrbField(c)
			a.renderShowcaseSparkBanner(c)
			a.renderMemoryOrbsGrid(c)
			a.renderPullDownToMindSpace(c)
		})
	})
}

func (a *App) renderShowcaseHeaderAndRecall(c *ui.Context) {
	pal := a.Palette()

	ui.Row(c).FillWidth().Padding(8, 14).Gap(8).AlignItems(ui.Center).
		Radius(999).Background(pal.Card).Border(1, pal.Border).
		Role(ui.RoleSearch).
		Children(func() {
			ui.Icon(c, IconSearch).Size(16, 16).TextColor(pal.InkMuted).Shrink(0)
			if ui.TextInputBase(c, &a.SearchQuery).
				Placeholder("搜搜以前记过的想法…").
				PlaceholderColor(pal.InkMuted).
				TextColor(pal.Ink).
				FontSize(14).
				Label("搜索灵感").
				Search().
				Grow(1).MinWidth(0).
				Changed() {
				a.refreshRecallHits()
			}
			if strings.TrimSpace(a.SearchQuery) != "" {
				if IconAction(c, "search_clear_pill", IconClose, "清空搜索", false).
					Size(26, 26).Radius(999).Clicked() {
					a.SearchQuery = ""
					a.RecallHits = nil
				}
			}
		})

	if strings.TrimSpace(a.SearchQuery) != "" {
		a.renderRecallResults(c)
	}
}

func (a *App) refreshRecallHits() {
	q := strings.TrimSpace(a.SearchQuery)
	if q == "" {
		a.RecallHits = nil
		return
	}
	hits, err := a.Deps.Organize.SearchRecall(q, 4)
	if err == nil {
		a.RecallHits = hits
	}
}

func (a *App) renderRecallResults(c *ui.Context) {
	pal := a.Palette()
	phase := a.Phase(c)

	ui.Column(c).FillWidth().Padding(12).Gap(8).Radius(16).
		Background(pal.Card).Border(1, pal.Border).
		Children(func() {
			ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(8).Children(func() {
				ui.Icon(c, IconSearch).Size(14, 14).TextColor(pal.Accent).Shrink(0)
				ui.Textf(c, "找到 %d 条相关记录", len(a.RecallHits)).
					FontSize(12.5).Bold().TextColor(pal.Accent).Grow(1)
				if PillAction(c, "recall_clear", IconClose, "清除", false).Clicked() {
					a.SearchQuery = ""
					a.RecallHits = nil
				}
			})

			if len(a.RecallHits) == 0 {
				ui.Text(c, "还没有关于这个词的记录，随手记一条吧。").
					FontSize(12).TextColor(pal.InkSecondary)
				return
			}

			for i, hit := range a.RecallHits {
				h := hit
				row := ui.Row(c).Key(fmt.Sprintf("recall_%d_%s", i, h.Node.ID)).
					FillWidth().Padding(8, 10).Gap(10).AlignItems(ui.Center).
					Radius(12).Background(pal.SoftPanel)
				row.Children(func() {
					ui.Box(c).Size(28, 28).Shrink(0).Draw(func(p *ui.Painter, r ui.Rect) {
						DrawAnimatedMemoryOrb(p, r, h.Node.Theme, h.Node.Weight, phase+float32(i)*0.7, float32(h.Node.Visual.Energy), h.Node.Visual.MotifType, false)
					})
					ui.Column(c).Grow(1).MinWidth(0).Gap(2).Children(func() {
						ui.Text(c, h.Node.Title).FontSize(13).Bold().TextColor(pal.Ink).MaxLines(1)
						ui.Textf(c, "%s · %d 条原记录", h.MatchReason, h.EvidenceCount).
							FontSize(11).TextColor(pal.InkMuted).MaxLines(1)
					})
					if PillAction(c, "recall_open_"+h.Node.ID, IconChevronRight, "打开", true).Clicked() {
						a.openNodeDetail(c, h.Node.ID)
					}
				})
			}
		})
}

func (a *App) renderTagFilterBar(c *ui.Context) {
	pal := a.Palette()

	ui.ScrollHorizontal(c).FillWidth().Scrollbars(false).Children(func() {
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			allActive := a.ActiveTag == ""
			a.renderFilterChip(c, "tag_all", "全部", allActive, pal, func() {
				a.ActiveTag = ""
			})

			limit := min(len(a.Tags), 10)
			for i := 0; i < limit; i++ {
				tc := a.Tags[i]
				tagName := tc.Tag
				label := fmt.Sprintf("#%s %d", tc.Tag, tc.Count)
				active := a.ActiveTag == tagName
				a.renderFilterChip(c, "tag_"+tagName, label, active, pal, func() {
					if a.ActiveTag == tagName {
						a.ActiveTag = ""
					} else {
						a.ActiveTag = tagName
					}
				})
			}
		})
	})
}

func (a *App) renderFilterChip(c *ui.Context, key, label string, active bool, pal SurfacePalette, onTap func()) {
	bgTarget := pal.SoftPanel
	fgTarget := pal.InkSecondary
	if active {
		bgTarget = pal.Accent
		fgTarget = pal.AccentText
	}

	chip := ui.ButtonBase(c).Key(key).Padding(6, 12).Radius(999).
		Cursor(ui.CursorPointer).Shrink(0).Clip().Label(label)
	bg := chip.AnimateColor("bg", bgTarget, ui.Animation{Duration: 160 * time.Millisecond, Oklab: true})
	fg := chip.AnimateColor("fg", fgTarget, ui.Animation{Duration: 160 * time.Millisecond, Oklab: true})
	chip.Background(bg).TextColor(fg)
	mobile.Ink(chip)
	chip.Children(func() {
		ui.Text(c, label).FontSize(12).Bold().SingleLine()
	})
	if chip.Clicked() {
		mygo.Haptics.Selection()
		onTap()
	}
}

func (a *App) renderMemoryOrbField(c *ui.Context) {
	filtered := a.filteredShowcaseNodes()
	if len(filtered) == 0 {
		return
	}

	pal := a.Palette()
	phase := a.Phase(c)
	phone := a.isPhoneLayout(c)

	maxField := min(len(filtered), 6)
	fieldNodes := filtered[:maxField]
	anchors := showcaseOrbAnchors(maxField)

	posByID := make(map[string][2]float32, maxField)
	for i, n := range fieldNodes {
		posByID[n.ID] = anchors[i]
	}

	focusedID := a.FocusedShowcaseID
	var focusedNode domain.IdeaNode
	hasExplicitFocus := false
	for _, n := range fieldNodes {
		if n.ID == focusedID {
			focusedNode = n
			hasExplicitFocus = true
			break
		}
	}
	if !hasExplicitFocus {
		focusedNode = fieldNodes[0]
		focusedID = ""
	}

	ui.Column(c).FillWidth().Gap(10).Children(func() {
		stage := ui.Box(c).Key("showcase_orb_field").FillWidth().Height(280).
			Radius(20).Background(pal.Card.Alpha(0.55)).Border(1, pal.Border.Alpha(0.75)).Clip()

		stage.Draw(func(p *ui.Painter, r ui.Rect) {
			cx := r.X + r.W/2
			cy := r.Y + r.H/2
			base := min(r.W, r.H)

			// Concentric orbital rings.
			for _, frac := range []float32{0.18, 0.31, 0.44} {
				var ring ui.Path
				ring.Circle(cx, cy, base*frac)
				p.StrokePath(&ring, 1, pal.Border.Alpha(0.62))
			}

			// Cardinal observatory ticks on middle ring.
			ringR := base * 0.31
			for _, ang := range []float64{0, math.Pi / 2, math.Pi, 3 * math.Pi / 2} {
				tx := cx + ringR*float32(math.Cos(ang))
				ty := cy + ringR*float32(math.Sin(ang))
				var tick ui.Path
				tick.Circle(tx, ty, 2)
				p.FillPath(&tick, pal.InkMuted.Alpha(0.35))
			}

			// Ambient wash under focused orb.
			if hasExplicitFocus {
				if anchor, ok := posByID[focusedNode.ID]; ok {
					fx := r.X + anchor[0]*r.W
					fy := r.Y + anchor[1]*r.H
					orbPal := ColorsForTheme(focusedNode.Theme)
					haloR := float32(76)
					p.FillRadialGradient(ui.Rect{
						X: fx - haloR,
						Y: fy - haloR,
						W: haloR * 2,
						H: haloR * 2,
					}, ui.RadialGradient{
						Center: ui.Point{X: 0.5, Y: 0.5},
						Radius: haloR,
						Oklab:  true,
						Stops: []ui.GradientStop{
							{Color: orbPal.Aura.Alpha(0.35), At: 0},
							{Color: ui.Transparent, At: 1},
						},
					}, haloR)
				}
			}

			// Subtle Bezier filament lines connecting RelatedIDs.
			drawnEdges := make(map[string]struct{})
			for _, n := range fieldNodes {
				p1, ok1 := posByID[n.ID]
				if !ok1 {
					continue
				}
				x1, y1 := r.X+p1[0]*r.W, r.Y+p1[1]*r.H
				for _, relID := range n.RelatedIDs {
					p2, ok2 := posByID[relID]
					if !ok2 || relID == n.ID {
						continue
					}
					edgeKey := n.ID + "|" + relID
					if relID < n.ID {
						edgeKey = relID + "|" + n.ID
					}
					if _, seen := drawnEdges[edgeKey]; seen {
						continue
					}
					drawnEdges[edgeKey] = struct{}{}

					x2, y2 := r.X+p2[0]*r.W, r.Y+p2[1]*r.H
					mx, my := (x1+x2)/2, (y1+y2)/2
					ctrlX := mx + (cx-mx)*0.22
					ctrlY := my + (cy-my)*0.22 - 14

					var fil ui.Path
					fil.MoveTo(x1, y1)
					fil.QuadTo(ctrlX, ctrlY, x2, y2)

					lineW := float32(1.1)
					lineCol := pal.Border.Alpha(0.9)
					if focusedID != "" && (n.ID == focusedID || relID == focusedID) {
						lineW = 1.8
						lineCol = pal.Accent.Alpha(0.55)
					}
					p.StrokePath(&fil, lineW, lineCol)
				}
			}
		})

		stage.Children(func() {
			for idx, node := range fieldNodes {
				n := node
				pos := anchors[idx]
				orbDim := float32(56 + 36*clampOrbWeight(n.Weight))
				isFocused := n.ID == focusedID

				orbBtn := ui.ButtonBase(c).Key("field_orb_"+n.ID).
					Absolute().
					LeftPercent(pos[0]*100).
					TopPercent(pos[1]*100).
					TranslatePercent(-50, -50).
					Size(orbDim, orbDim).
					Radius(orbDim / 2).
					Cursor(ui.CursorPointer).
					Label("查看灵感 " + n.Title).
					Tooltip(n.Title)

				pose := ui.Pose{}
				if isFocused {
					pose = ui.Pose{Scale: 1.07, Y: -2}
				} else if orbBtn.Hovered() {
					pose = ui.Pose{Scale: 1.04}
				}
				orbBtn.AnimatePose(pose, ui.Snappy)

				orbBtn.Draw(func(p *ui.Painter, r ui.Rect) {
					DrawAnimatedMemoryOrb(p, r, n.Theme, n.Weight, phase+float32(idx)*0.85, float32(n.Visual.Energy), n.Visual.MotifType, isFocused)
				})

				orbBtn.OnClick(func() {
					mygo.Haptics.Selection()
					if a.FocusedShowcaseID == n.ID {
						a.SelectNode(n.ID)
						if phone {
							a.DetailSheetOpen = true
						} else {
							a.ActiveSpace = SpaceMind
						}
					} else {
						a.FocusedShowcaseID = n.ID
					}
				})

				pillBg := pal.Card.Alpha(0.90)
				pillBorder := pal.Border.Alpha(0.65)
				pillFg := pal.InkSecondary
				if isFocused {
					pillBg = pal.Card
					pillBorder = pal.Accent.Alpha(0.55)
					pillFg = pal.Ink
				}

				ui.Box(c).Key("field_pill_"+n.ID).
					Absolute().
					LeftPercent(pos[0]*100).
					TopPercent(pos[1]*100).
					Translate(0, orbDim/2+3).
					TranslatePercent(-50, 0).
					Padding(1.5, 6).
					Radius(999).
					Background(pillBg).
					Border(1, pillBorder).
					PassThrough().
					Children(func() {
						ui.Text(c, trimText(n.Title, 6)).
							FontSize(10.5).
							Bold().
							TextColor(pillFg).
							SingleLine()
					})
			}
		})

		// Sleek whisper bar for the focused (or lead) memory orb.
		orbPal := ColorsForTheme(focusedNode.Theme)
		ui.Row(c).Key("showcase_whisper_bar").FillWidth().Padding(10, 12).Gap(10).
			AlignItems(ui.Center).Radius(14).Background(pal.Card).Border(1, pal.Border).
			Children(func() {
				ui.Box(c).Size(8, 8).Radius(4).Background(orbPal.Mid).Shrink(0)
				infoBtn := ui.ButtonBase(c).Key("whisper_info_" + focusedNode.ID).
					Grow(1).MinWidth(0).Cursor(ui.CursorPointer).Label(focusedNode.Title)
				infoBtn.Children(func() {
					ui.Column(c).FillWidth().Gap(2).Children(func() {
						ui.Text(c, focusedNode.Title).FontSize(13.5).Bold().TextColor(pal.Ink).MaxLines(1)
						ui.Text(c, trimText(focusedNode.ConversationalSummary, 46)).
							FontSize(11.5).TextColor(pal.InkSecondary).MaxLines(1)
					})
				})
				infoBtn.OnClick(func() {
					mygo.Haptics.Selection()
					a.SelectNode(focusedNode.ID)
					if phone {
						a.DetailSheetOpen = true
					} else {
						a.ActiveSpace = SpaceMind
					}
				})

				if PillAction(c, "whisper_detail_"+focusedNode.ID, IconChevronRight, "查看详情", false).Clicked() {
					mygo.Haptics.Selection()
					a.openNodeDetail(c, focusedNode.ID)
				}
			})
	})
}

func showcaseOrbAnchors(count int) [][2]float32 {
	switch count {
	case 1:
		return [][2]float32{{0.50, 0.48}}
	case 2:
		return [][2]float32{{0.33, 0.46}, {0.69, 0.50}}
	case 3:
		return [][2]float32{{0.50, 0.30}, {0.24, 0.67}, {0.76, 0.65}}
	case 4:
		return [][2]float32{{0.33, 0.30}, {0.73, 0.27}, {0.23, 0.69}, {0.69, 0.70}}
	case 5:
		return [][2]float32{{0.50, 0.47}, {0.20, 0.24}, {0.80, 0.24}, {0.21, 0.73}, {0.79, 0.73}}
	default:
		return [][2]float32{
			{0.32, 0.23},
			{0.70, 0.22},
			{0.17, 0.57},
			{0.51, 0.52},
			{0.84, 0.56},
			{0.65, 0.81},
		}
	}
}

func (a *App) renderShowcaseSparkBanner(c *ui.Context) {
	pal := a.Palette()

	ui.Row(c).FillWidth().Wrap().Padding(10, 12).Gap(8).AlignItems(ui.Center).
		Justify(ui.SpaceBetween).Radius(12).Background(pal.SoftPanel.Alpha(0.7)).
		Children(func() {
			ui.Row(c).Gap(8).AlignItems(ui.Center).Grow(1).MinWidth(200).Children(func() {
				ui.Icon(c, IconSpark).Size(15, 15).TextColor(pal.Accent).Shrink(0)
				ui.Text(c, a.ShowcaseSpark).FontSize(12.5).LineHeight(1.45).
					TextColor(pal.InkSecondary).Grow(1).MinWidth(0)
			})
			ui.Row(c).Gap(6).Shrink(0).AlignItems(ui.Center).Children(func() {
				if PillAction(c, "spark_refresh", IconRefresh, "换一条", false).Clicked() {
					a.RunAsync("正在换一条新启发…", func(ctx context.Context) error {
						spark, err := a.Deps.Incubate.SuggestShowcaseSpark(ctx)
						if err != nil {
							return err
						}
						a.Dispatch(func() { a.ShowcaseSpark = spark })
						return nil
					})
				}
				if PillAction(c, "spark_capture", IconQuill, "顺着记", false).Clicked() {
					a.CaptureText = a.ShowcaseSpark
					a.CaptureModalityIdx = 1
					a.CaptureModalOpen = true
				}
			})
		})
}

func (a *App) renderMemoryOrbsGrid(c *ui.Context) {
	pal := a.Palette()
	filtered := a.filteredShowcaseNodes()
	if len(filtered) == 0 {
		ui.Column(c).FillWidth().Padding(28).Center().Gap(8).Radius(16).
			Background(pal.Card).Border(1, pal.Border).Children(func() {
			ui.Text(c, "还没有匹配的灵感记录").FontSize(15).Bold().TextColor(pal.Ink)
			ui.Text(c, "点右下角「记灵感」，随口说一句或写几个字就行。").
				FontSize(12.5).TextColor(pal.InkSecondary).TextAlign(ui.Center)
		})
		return
	}

	phone := a.isPhoneLayout(c)
	ui.Row(c).FillWidth().Wrap().Gap(12).Children(func() {
		for _, node := range filtered {
			n := node
			a.renderShowcaseOrbCard(c, n, phone)
		}
	})
}

func (a *App) renderShowcaseOrbCard(c *ui.Context, n domain.IdeaNode, fullWidth bool) {
	pal := a.Palette()
	orbPal := ColorsForTheme(n.Theme)
	phase := a.Phase(c)
	isSelected := n.ID == a.SelectedID || n.ID == a.FocusedShowcaseID
	evCount := a.EvCounts[n.ID]

	borderCol := pal.Border
	borderW := float32(1)
	if isSelected {
		borderCol = pal.Accent.Alpha(0.75)
		borderW = 1.5
	}

	card := ui.ButtonBase(c).Key("orb_card_"+n.ID).Padding(14, 16).Radius(16).
		Background(pal.Card).Border(borderW, borderCol)
	if fullWidth {
		card.FillWidth()
	} else {
		card.Width(332)
	}
	mobile.Ink(card)

	card.OnClick(func() {
		mygo.Haptics.Selection()
		a.FocusedShowcaseID = n.ID
		a.SelectNode(n.ID)
		if fullWidth {
			a.DetailSheetOpen = true
		} else {
			a.ActiveSpace = SpaceMind
		}
	}).Children(func() {
		ui.Column(c).FillWidth().Gap(8).Children(func() {
			ui.Row(c).FillWidth().Gap(12).AlignItems(ui.Center).Children(func() {
				ui.Box(c).Size(52, 52).Shrink(0).Draw(func(p *ui.Painter, r ui.Rect) {
					DrawAnimatedMemoryOrb(p, r, n.Theme, n.Weight, phase, float32(n.Visual.Energy), n.Visual.MotifType, isSelected)
				})
				ui.Column(c).Grow(1).MinWidth(0).Gap(4).Children(func() {
					ui.Row(c).FillWidth().Gap(6).AlignItems(ui.Center).Children(func() {
						ui.Box(c).Size(6, 6).Radius(3).Background(orbPal.Mid).Shrink(0)
						ui.Text(c, n.Stage.Label()).FontSize(11).Bold().TextColor(pal.InkSecondary)
						ui.Spacer(c).Grow(1)
						ui.Textf(c, "%d 条原记录", evCount).FontSize(11).TextColor(pal.InkMuted)
					})
					ui.Text(c, n.Title).FontSize(15).Bold().TextColor(pal.Ink).MaxLines(2)
				})
			})

			ui.Text(c, trimText(n.ConversationalSummary, 62)).
				FontSize(13).LineHeight(1.48).TextColor(pal.InkSecondary).MaxLines(2)

			if len(n.Tags) > 0 {
				ui.Row(c).FillWidth().Wrap().Gap(6).Children(func() {
					for _, tag := range n.Tags {
						ui.Text(c, "#"+tag).FontSize(11).Padding(3, 8).Radius(999).
							Background(pal.SoftPanel).TextColor(pal.InkSecondary)
					}
				})
			}
		})
	})
}

func (a *App) renderPullDownToMindSpace(c *ui.Context) {
	pal := a.Palette()
	ui.Column(c).FillWidth().Center().Padding(8, 0, 4, 0).Gap(8).Children(func() {
		ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
			ui.Icon(c, IconChevronDown).Size(14, 14).TextColor(pal.InkMuted)
			ui.Text(c, "继续上滑进入整理视图").FontSize(12).TextColor(pal.InkMuted)
		})
		if PillAction(c, "showcase_to_mind", IconMind, "去整理", false).Clicked() {
			mygo.Haptics.Selection()
			a.ActiveSpace = SpaceMind
		}
	})
}

func (a *App) openNodeDetail(c *ui.Context, id string) {
	a.SelectNode(id)
	phone := true
	if c != nil {
		phone = a.isPhoneLayout(c)
	}
	if phone {
		a.DetailSheetOpen = true
	} else {
		a.ActiveSpace = SpaceMind
	}
}

func (a *App) filteredShowcaseNodes() []domain.IdeaNode {
	if a.ActiveTag == "" {
		return a.Nodes
	}
	var out []domain.IdeaNode
	for _, n := range a.Nodes {
		for _, t := range n.Tags {
			if t == a.ActiveTag {
				out = append(out, n)
				break
			}
		}
	}
	return out
}

func trimText(s string, maxRunes int) string {
	r := []rune(strings.TrimSpace(s))
	if len(r) <= maxRunes {
		return string(r)
	}
	return string(r[:maxRunes]) + "…"
}

func clampOrbWeight(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}
