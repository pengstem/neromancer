package ui

import (
	"math"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/mobile"
	"github.com/pengstem/neromancer/internal/domain"
)

func (a *App) renderMindSpaceView(c *ui.Context) {
	pal := a.Palette()
	phone := a.isPhoneLayout(c)

	ui.Column(c).Key("space_mind").Fill().Background(pal.Canvas).Children(func() {
		a.renderMindSubBar(c, pal, phone)

		if phone {
			ui.Column(c).Grow(1).FillWidth().Background(pal.Canvas).Children(func() {
				a.renderMindOrgModeBody(c)
			})
			return
		}

		// Tablet / Wide Desktop: Split master-detail view
		ui.Row(c).Grow(1).FillWidth().Children(func() {
			ui.Column(c).Width(410).FillHeight().
				Background(pal.Canvas).BorderWidth(0, 1, 0, 0).BorderColor(pal.Border).
				Children(func() {
					a.renderMindOrgModeBody(c)
				})

			ui.Box(c).Grow(1).FillHeight().Background(pal.Card).Children(func() {
				a.renderDetailContent(c)
			})
		})
	})
}

func (a *App) renderMindSubBar(c *ui.Context, pal SurfacePalette, phone bool) {
	ui.Row(c).FillWidth().Padding(8, 12).Gap(8).AlignItems(ui.Center).
		BorderWidth(0, 0, 1, 0).BorderColor(pal.Border.Alpha(0.55)).
		Children(func() {
			if PillAction(c, "mind_back_showcase", IconChevronUp, "星海", false).Shrink(0).Clicked() {
				mygo.Haptics.Selection()
				a.ActiveSpace = SpaceShowcase
			}

			ui.Spacer(c).Grow(1)

			// Tactile 3-lens switcher capsule
			ui.Row(c).Padding(3).Gap(2).Radius(999).Shrink(0).
				Background(pal.SoftPanel.Alpha(0.85)).
				Border(1, pal.Border.Alpha(0.6)).
				Children(func() {
					a.renderMindLensPill(c, pal, "lens_outline", IconOutline, "目录", OrgOutline)
					a.renderMindLensPill(c, pal, "lens_graph", IconConstellation, "星图", OrgGraph)
					a.renderMindLensPill(c, pal, "lens_cards", IconFilmstrip, "场景", OrgCards)
				})

			if phone {
				if n, ok := a.SelectedNode(); ok {
					detailBtn := ui.ButtonBase(c).Key("mind_open_detail").
						Padding(6, 10).Radius(999).Gap(3).AlignItems(ui.Center).Shrink(0).
						Background(pal.Card).Border(1, pal.Border).
						Cursor(ui.CursorPointer).Label("详情")
					mobile.Ink(detailBtn)
					detailBtn.OnClick(func() {
						mygo.Haptics.Selection()
						a.SelectNode(n.ID)
						a.DetailSheetOpen = true
					}).Children(func() {
						ui.Text(c, "详情").FontSize(12).FontWeight(600).TextColor(pal.Ink).SingleLine()
						ui.Icon(c, IconChevronRight).FontSize(13).TextColor(pal.InkSecondary)
					})
				}
			}
		})
}

func (a *App) renderMindLensPill(c *ui.Context, pal SurfacePalette, key string, ic *ui.SVG, label string, mode OrgMode) {
	active := a.MindOrgMode == mode
	bg := ui.Transparent
	fg := pal.InkSecondary
	weight := 500
	if active {
		bg = pal.Card
		fg = pal.Ink
		weight = 600
	}

	pill := ui.ButtonBase(c).Key(key).
		Padding(5, 9).Radius(999).Gap(4).AlignItems(ui.Center).Shrink(0).
		Background(bg).Cursor(ui.CursorPointer).Label(label)
	if active {
		pill.Shadow(0, 1, 4, 0, ui.RGBA(0, 0, 0, 0.08))
	}
	mobile.Ink(pill)
	pill.OnClick(func() {
		if a.MindOrgMode != mode {
			mygo.Haptics.Selection()
			a.MindOrgMode = mode
		}
	}).Children(func() {
		ui.Icon(c, ic).FontSize(13).TextColor(fg)
		ui.Text(c, label).FontSize(12).FontWeight(weight).TextColor(fg).SingleLine()
	})
}

func (a *App) renderMindOrgModeBody(c *ui.Context) {
	switch a.MindOrgMode {
	case OrgOutline:
		a.renderOutlineTree(c)
	case OrgGraph:
		a.renderConstellationGraph(c)
	case OrgCards:
		a.renderSceneLightCards(c)
	}
}

func (a *App) renderOutlineTree(c *ui.Context) {
	pal := a.Palette()
	phase := a.Phase(c)

	a.OutlineChoice = a.SelectedID
	if len(a.Roots) == 0 {
		ui.Column(c).Fill().Center().Padding(24, 24, 112, 24).Gap(6).Children(func() {
			ui.Icon(c, IconOutline).FontSize(26).TextColor(pal.InkMuted)
			ui.Text(c, "暂无记忆条目").FontSize(14).FontWeight(600).TextColor(pal.InkSecondary)
		})
		return
	}

	// Custom Prefix uses pure data (no *ui.Context / Element capture across frames).
	a.OutlineState.Prefix = outlineTreePrefix(pal.InkMuted)

	phone := a.isPhoneLayout(c)
	el := ui.Outline(c, &a.OutlineState, a.Roots, func(id string) []string {
		return a.Children[id]
	}, func(id string) {
		n, ok := a.NodeByID[id]
		if !ok {
			return
		}
		orbPal := ColorsForTheme(n.Theme)
		selTint, _, _ := mindOrbSurface(pal, a.DarkMode, n.Theme)
		evCount := a.EvCounts[n.ID]
		isSel := n.ID == a.SelectedID

		row := ui.ButtonBase(c).Key("out_"+n.ID).Grow(1).MinWidth(0).
			Padding(13, 10).Cursor(ui.CursorPointer)
		if isSel {
			row.Radius(12).Background(selTint)
		} else {
			row.Background(ui.Transparent).BorderWidth(0, 0, 1, 0).BorderColor(pal.Border.Alpha(0.55))
		}
		mobile.Ink(row)
		row.OnClick(func() {
			mygo.Haptics.Selection()
			a.SelectNode(n.ID)
			if phone {
				a.DetailSheetOpen = true
			}
		}).Children(func() {
			ui.Row(c).FillWidth().Gap(12).AlignItems(ui.Center).Children(func() {
				ui.Box(c).Size(28, 28).Shrink(0).Draw(func(p *ui.Painter, r ui.Rect) {
					DrawAnimatedMemoryOrb(p, r, n.Theme, n.Weight, phase, float32(n.Visual.Energy), n.Visual.MotifType, isSel)
				})
				ui.Column(c).Grow(1).MinWidth(0).Gap(4).Children(func() {
					ui.Text(c, n.Title).FontSize(15).FontWeight(600).TextColor(pal.Ink).MaxLines(2)
					ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
						ui.Box(c).Size(6, 6).Radius(99).Background(orbPal.Rim).Shrink(0)
						ui.Text(c, n.Stage.Label()).FontSize(12).TextColor(pal.InkSecondary)
						ui.Text(c, "·").FontSize(12).TextColor(pal.InkMuted)
						ui.Textf(c, "%d 段原记录", evCount).FontSize(12).TextColor(pal.InkMuted)
					})
				})
				chevronCol := pal.InkMuted
				if isSel {
					chevronCol = pal.Accent
				}
				ui.Icon(c, IconChevronRight).FontSize(14).TextColor(chevronCol).Shrink(0)
			})
		})
	}).Padding(6, 14, 112, 14).Gap(2).Grow(1).FillWidth()

	if el.Changed() {
		if row := a.OutlineState.List.Selected; row != nil && *row >= 0 && *row < a.OutlineState.Rows() {
			a.SelectNode(a.OutlineState.Item(*row))
		}
	}
}

func outlineTreePrefix(inkMuted ui.Color) func(ui.OutlineRow) {
	return func(r ui.OutlineRow) {
		if r.Depth == 0 && !r.Branch {
			return
		}
		w := float32(r.Depth)*18 + 18
		arrow := r.Arrow().Size(w, 24)
		if r.Branch {
			open := r.Open
			arrow.Draw(func(p *ui.Painter, rect ui.Rect) {
				cx := rect.X + rect.W - 9
				cy := rect.Y + rect.H/2
				var path ui.Path
				if open {
					path.MoveTo(cx-4, cy-2).LineTo(cx, cy+2.5).LineTo(cx+4, cy-2)
				} else {
					path.MoveTo(cx-2, cy-4).LineTo(cx+2.5, cy).LineTo(cx-2, cy+4)
				}
				p.StrokePath(&path, 1.6, inkMuted)
			})
		}
	}
}

func (a *App) renderConstellationGraph(c *ui.Context) {
	pal := a.Palette()
	phase := a.Phase(c)
	phone := a.isPhoneLayout(c)

	viewport := ui.Box(c).Key("graph_viewport").Grow(1).FillWidth().Clip().Background(pal.Canvas)
	b := viewport.Bounds()
	canvasW, canvasH := b.W, b.H
	if canvasW < 180 || canvasH < 240 {
		winW, winH := c.Size()
		if canvasW < 180 {
			if phone {
				canvasW = max(float32(360), winW)
			} else {
				canvasW = 410
			}
		}
		if canvasH < 240 {
			canvasH = max(float32(480), winH-104)
		}
	}
	// Reserve bottom clearance (>= 104px) so orbs and labels never sink behind the frosted dock.
	safeH := max(float32(260), canvasH-112)
	positions := computeOrbGraphPositions(a.Nodes, canvasW, safeH)

	viewport.Children(func() {
		stage := ui.Box(c).Key("graph_stage").Size(canvasW, canvasH)
		stage.Draggable(&a.GraphOffset, ui.DraggableOptions{
			Bounds: &ui.Rect{X: -110, Y: -90, W: 220, H: 180},
		})

		stage.Draw(func(p *ui.Painter, r ui.Rect) {
			a.drawConstellationBackdropAndEdges(p, r, pal, positions, safeH)
		}).Children(func() {
			for _, n := range a.Nodes {
				node := n
				pt := positions[node.ID]
				isSel := node.ID == a.SelectedID
				orbPal := ColorsForTheme(node.Theme)

				orbSize := float32(48 + 18*clamp01(node.Weight))
				colW := float32(152)

				btn := ui.ButtonBase(c).Key("graph_node_" + node.ID).
					Absolute().Left(pt.X - colW/2).Top(pt.Y - orbSize/2).Width(colW).
					Cursor(ui.CursorPointer).Label(node.Title)
				btn.OnClick(func() {
					mygo.Haptics.Selection()
					a.SelectNode(node.ID)
					if phone {
						a.DetailSheetOpen = true
					}
				}).Children(func() {
					ui.Column(c).FillWidth().AlignItems(ui.Center).Gap(6).Children(func() {
						ui.Box(c).Size(orbSize, orbSize).Draw(func(p *ui.Painter, r ui.Rect) {
							DrawAnimatedMemoryOrb(p, r, node.Theme, node.Weight, phase, float32(node.Visual.Energy), node.Visual.MotifType, isSel)
						})

						pillBg := pal.Card.Alpha(0.92)
						pillBorder := pal.Border.Alpha(0.85)
						pillText := pal.Ink
						dotColor := orbPal.Rim
						if isSel {
							pillBg = pal.Accent
							pillBorder = pal.Accent
							pillText = pal.AccentText
							dotColor = orbPal.Core
						}

						ui.Row(c).Padding(4, 10).Radius(999).Gap(5).AlignItems(ui.Center).
							Background(pillBg).Border(1, pillBorder).
							Shadow(0, 2, 8, 0, ui.RGBA(0, 0, 0, 0.08)).
							Children(func() {
								ui.Box(c).Size(5, 5).Radius(99).Background(dotColor).Shrink(0)
								ui.Text(c, trimText(node.Title, 11)).FontSize(11).FontWeight(600).
									TextColor(pillText).SingleLine()
							})
					})
				})
			}
		})

		if a.GraphOffset.X != 0 || a.GraphOffset.Y != 0 {
			ui.Box(c).Absolute().Top(12).Right(14).Children(func() {
				if PillAction(c, "graph_reset_pan", IconRefresh, "居中", false).Clicked() {
					a.GraphOffset = ui.Point{}
				}
			})
		}
	})
}

func (a *App) drawConstellationBackdropAndEdges(p *ui.Painter, r ui.Rect, pal SurfacePalette, positions map[string]ui.Point, safeH float32) {
	cx := r.X + r.W/2
	cy := r.Y + safeH*0.50
	minDim := min(r.W, safeH)

	// Subtle atmospheric central nebula glow
	glowR := minDim * 0.62
	p.FillRadialGradient(ui.Rect{
		X: cx - glowR,
		Y: cy - glowR,
		W: glowR * 2,
		H: glowR * 2,
	}, ui.RadialGradient{
		Center: ui.Point{X: 0.5, Y: 0.5},
		Radius: glowR,
		Oklab:  true,
		Stops: []ui.GradientStop{
			{Color: pal.Accent.Alpha(0.06), At: 0},
			{Color: pal.Accent.Alpha(0.02), At: 0.55},
			{Color: ui.Transparent, At: 1},
		},
	}, glowR)

	// Concentric orbital reference rings
	ringAlpha := float32(0.55)
	if a.DarkMode {
		ringAlpha = 0.75
	}
	ringColor := pal.Border.Alpha(ringAlpha)
	for _, ratio := range []float32{0.24, 0.39, 0.54} {
		rad := minDim * ratio
		var ring ui.Path
		ring.Circle(cx, cy, rad)
		p.StrokePath(&ring, 1.0, ringColor)
	}

	// Cardinal tick marks on middle orbit
	midR := minDim * 0.39
	dotCol := pal.InkMuted.Alpha(0.32)
	for i := 0; i < 4; i++ {
		ang := float64(i) * math.Pi / 2
		dx := cx + float32(math.Cos(ang))*midR
		dy := cy + float32(math.Sin(ang))*midR
		var dot ui.Path
		dot.Circle(dx, dy, 2.0)
		p.FillPath(&dot, dotCol)
	}

	if len(a.Nodes) == 0 {
		return
	}

	// Curved glowing Bezier filaments connecting RelatedIDs (deduplicated undirected edges)
	drawn := make(map[string]bool)
	for _, n := range a.Nodes {
		from, ok1 := positions[n.ID]
		if !ok1 {
			continue
		}
		for _, targetID := range n.RelatedIDs {
			edgeKey := n.ID + "->" + targetID
			if targetID < n.ID {
				edgeKey = targetID + "->" + n.ID
			}
			if drawn[edgeKey] {
				continue
			}
			to, ok2 := positions[targetID]
			if !ok2 {
				continue
			}
			drawn[edgeKey] = true

			x0, y0 := r.X+from.X, r.Y+from.Y
			x1, y1 := r.X+to.X, r.Y+to.Y
			mx, my := (x0+x1)/2, (y0+y1)/2
			dx, dy := x1-x0, y1-y0
			dist := float32(math.Hypot(float64(dx), float64(dy)))
			// Gentle arc bending outward from constellation center
			bend := float32(22)
			if dist > 1 {
				nx, ny := -dy/dist, dx/dist
				// Point normal away from (cx, cy)
				if (mx+nx-cx)*(mx+nx-cx)+(my+ny-cy)*(my+ny-cy) < (mx-nx-cx)*(mx-nx-cx)+(my-ny-cy)*(my-ny-cy) {
					nx, ny = -nx, -ny
				}
				mx += nx * bend
				my += ny * bend
			} else {
				my -= 20
			}

			var edge ui.Path
			edge.MoveTo(x0, y0)
			edge.QuadTo(mx, my, x1, y1)

			isHighlighted := n.ID == a.SelectedID || targetID == a.SelectedID
			glowAlpha := float32(0.11)
			lineAlpha := float32(0.32)
			if isHighlighted {
				glowAlpha = 0.22
				lineAlpha = 0.62
			}
			p.StrokePath(&edge, 4.5, pal.Accent.Alpha(glowAlpha))
			p.StrokePath(&edge, 1.4, pal.Accent.Alpha(lineAlpha))
		}
	}
}

func computeOrbGraphPositions(nodes []domain.IdeaNode, w, h float32) map[string]ui.Point {
	out := make(map[string]ui.Point, len(nodes))
	n := len(nodes)
	if n == 0 {
		return out
	}
	cx := w / 2
	cy := h * 0.50
	rx := max(float32(68), min(w*0.29, (w-144)*0.48))
	ry := max(float32(78), min(h*0.31, (h-120)*0.46))

	for i, node := range nodes {
		if n == 1 {
			out[node.ID] = ui.Point{X: cx, Y: cy}
			continue
		}
		angle := -math.Pi*0.38 + (2*math.Pi*float64(i))/float64(n)
		// Slightly weight-modulated radial distance so heavy core orbs gravitate gently inward
		radialScale := float32(0.86 + 0.22*(1.0-clamp01(node.Weight)))
		out[node.ID] = ui.Point{
			X: cx + float32(math.Cos(angle))*rx*radialScale,
			Y: cy + float32(math.Sin(angle))*ry*radialScale,
		}
	}
	return out
}

func (a *App) renderSceneLightCards(c *ui.Context) {
	pal := a.Palette()
	phase := a.Phase(c)
	phone := a.isPhoneLayout(c)

	ui.Scroll(c).Grow(1).FillWidth().Children(func() {
		ui.Column(c).FillWidth().Padding(8, 16, 112, 16).Gap(14).Children(func() {
			if len(a.Nodes) == 0 {
				ui.Column(c).FillWidth().Padding(32, 16).Center().Gap(6).Children(func() {
					ui.Icon(c, IconFilmstrip).FontSize(26).TextColor(pal.InkMuted)
					ui.Text(c, "暂无场景记录").FontSize(14).FontWeight(600).TextColor(pal.InkSecondary)
				})
				return
			}

			for _, node := range a.Nodes {
				n := node
				orbPal := ColorsForTheme(n.Theme)
				_, badgeBg, badgeText := mindOrbSurface(pal, a.DarkMode, n.Theme)
				isSel := n.ID == a.SelectedID

				evs, _ := a.Deps.Store.ListEvidencesByNode(n.ID)
				evCount := len(evs)
				if evCount == 0 {
					evCount = a.EvCounts[n.ID]
				}
				sceneText := "随手记下"
				capturedAt := n.CreatedAt
				if len(evs) > 0 {
					sceneText = evs[0].Scene.SummaryLine()
					capturedAt = evs[0].CapturedAt
				}
				timeText := formatShortTime(capturedAt)

				border := pal.Border
				borderW := float32(1)
				if isSel {
					border = pal.Accent
					borderW = 1.5
				}

				card := ui.ButtonBase(c).Key("scene_card_"+n.ID).FillWidth().
					Padding(15, 16).Radius(16).
					Background(pal.Card).Border(borderW, border).
					Cursor(ui.CursorPointer)
				if !a.DarkMode {
					card.Shadow(0, 3, 12, 0, ui.RGBA(0, 0, 0, 0.04))
				} else {
					card.InsetShadow(0, 1, 0, 0, pal.GlassBorder)
				}
				mobile.Ink(card)

				card.OnClick(func() {
					mygo.Haptics.Selection()
					a.SelectNode(n.ID)
					if phone {
						a.DetailSheetOpen = true
					}
				}).Children(func() {
					ui.Column(c).FillWidth().Gap(10).Children(func() {
						// Anchored Time & Scene Metadata Header Row
						ui.Row(c).FillWidth().Gap(6).AlignItems(ui.Center).Children(func() {
							ui.Icon(c, IconClock).FontSize(12).TextColor(pal.InkMuted).Shrink(0)
							ui.Text(c, timeText).FontSize(11).TextColor(pal.InkMuted).SingleLine().Shrink(0)
							ui.Text(c, "·").FontSize(11).TextColor(pal.InkMuted).Shrink(0)
							ui.Icon(c, IconMapPin).FontSize(12).TextColor(orbPal.Rim).Shrink(0)
							ui.Text(c, sceneText).FontSize(11).TextColor(pal.InkSecondary).Grow(1).MinWidth(0).SingleLine()
							ui.Textf(c, "%d 段原记录", evCount).FontSize(11).TextColor(pal.InkMuted).Shrink(0)
						})

						// Title + Animated Orb + Stage
						ui.Row(c).FillWidth().Gap(10).AlignItems(ui.Center).Children(func() {
							ui.Box(c).Size(28, 28).Shrink(0).Draw(func(p *ui.Painter, r ui.Rect) {
								DrawAnimatedMemoryOrb(p, r, n.Theme, n.Weight, phase, float32(n.Visual.Energy), n.Visual.MotifType, isSel)
							})
							ui.Text(c, n.Title).FontSize(15).Bold().TextColor(pal.Ink).Grow(1).MinWidth(0).MaxLines(2)
							ui.Text(c, n.Stage.Label()).FontSize(11).FontWeight(600).
								Padding(3, 8).Radius(999).
								Background(badgeBg).TextColor(badgeText).Shrink(0)
						})

						// 2-line Spoken Excerpt with thin accent rule
						ui.Row(c).FillWidth().Gap(10).AlignItems(ui.Stretch).Children(func() {
							ui.Box(c).Width(2.5).Radius(2).Background(orbPal.Rim.Alpha(0.65)).Shrink(0)
							ui.Text(c, trimText(n.ConversationalSummary, 68)).
								FontSize(13).LineHeight(1.55).TextColor(pal.InkSecondary).MaxLines(2).Grow(1).MinWidth(0)
						})
					})
				})
			}
		})
	})
}

func mindOrbSurface(pal SurfacePalette, dark bool, theme domain.OrbTheme) (tintBg, badgeBg, badgeText ui.Color) {
	orbPal := ColorsForTheme(theme)
	if dark {
		tintBg = pal.Card.MixOklab(orbPal.Mid, 0.14)
		badgeBg = orbPal.Mid.Alpha(0.16)
		badgeText = orbPal.Core.MixOklab(pal.Ink, 0.25)
		return tintBg, badgeBg, badgeText
	}
	tintBg = pal.Card.MixOklab(orbPal.Mid, 0.08)
	badgeBg = orbPal.Rim.Alpha(0.11)
	badgeText = orbPal.BadgeText
	return tintBg, badgeBg, badgeText
}

func formatShortTime(t time.Time) string {
	if t.IsZero() {
		return "刚刚"
	}
	return t.Format("01-02 15:04")
}
