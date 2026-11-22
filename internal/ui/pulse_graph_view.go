package ui

import (
	"context"
	"fmt"
	"math"
	"strings"
	"unicode"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/mobile"
	"github.com/pengstem/neromancer/internal/domain"
	"github.com/pengstem/neromancer/internal/service"
)

// Obsidian Dark Workspace semantic color tokens (Option A).
var (
	obsBgGraph     = ui.Hex("#161618")
	obsBgTree      = ui.Hex("#1E1E22")
	obsBgElevated  = ui.Hex("#25252B")
	obsBgActiveRow = ui.Hex("#2A2A32")
	obsBorder      = ui.Hex("#2C2C32")
	obsGuideLine   = ui.Hex("#323238")
	obsTextBright  = ui.Hex("#F4F4F6")
	obsTextPrimary = ui.Hex("#E4E4E7")
	obsTextMuted   = ui.Hex("#9E9EA8")
	obsTextDim     = ui.Hex("#6E6E77")
	obsTextSubtle  = ui.Hex("#52525B")
	obsAccent      = ui.Hex("#A882FF") // Obsidian lavender focus & active bar
	obsEmerald     = ui.Hex("#3ECF6E") // Obsidian tag node green
	obsViolet      = ui.Hex("#9B8AFB") // Secondary hub cluster tint
)

// PulseViewState holds pure frame-persistent state for the Obsidian Graph + AI File Explorer view.
type PulseViewState struct {
	ShowTags          bool
	ShowEvidences     bool
	FilterPopoverOpen bool

	PanX       float32
	PanY       float32
	Zoom       float32
	SplitRatio float32 // fraction of body height given to Graph stage (0.26 .. 0.68)

	FocusedTag       string
	CollapsedFolders map[string]bool
	TreeScroll       ui.ScrollState

	// Pointer gesture tracking on Graph stage & Split handle
	DragDummy     ui.Point
	SplitDummy    ui.Point
	Dragging      bool
	DragMoved     bool
	DownX         float32
	DownY         float32
	StartPanX     float32
	StartPanY     float32
	HitVertexIdx  int
	SplitDragging bool
	SplitDownY    float32
	SplitStartRat float32

	// Memoized force-directed graph cache
	CachedVaultGen      int
	CachedQuery         string
	CachedTag           string
	CachedShowTags      bool
	CachedShowEvidences bool
	Graph               service.PulseGraph
}

func newPulseViewState() PulseViewState {
	return PulseViewState{
		ShowTags:         true,
		ShowEvidences:    true,
		Zoom:             1.0,
		SplitRatio:       0.45,
		HitVertexIdx:     -1,
		CollapsedFolders: make(map[string]bool),
	}
}

func (a *App) ensurePulseGraph() service.PulseGraph {
	q := strings.TrimSpace(a.SearchQuery)
	tag := cleanTagLabel(a.ActiveTag)
	p := &a.Pulse
	if p.CollapsedFolders == nil {
		p.CollapsedFolders = make(map[string]bool)
	}
	if p.Zoom <= 0 {
		p.Zoom = 1.0
	}
	if p.SplitRatio <= 0 {
		p.SplitRatio = 0.45
	}

	if p.CachedVaultGen == a.VaultGen &&
		p.CachedQuery == q &&
		p.CachedTag == tag &&
		p.CachedShowTags == p.ShowTags &&
		p.CachedShowEvidences == p.ShowEvidences {
		return p.Graph
	}

	filteredNodes := a.filteredArchiveNodes()
	nodeKeep := make(map[string]bool, len(filteredNodes))
	for _, n := range filteredNodes {
		nodeKeep[n.ID] = true
	}
	var filteredEvs []domain.RawEvidence
	for _, ev := range a.AllEvidences {
		if nodeKeep[ev.NodeID] {
			filteredEvs = append(filteredEvs, ev)
		}
	}

	p.Graph = a.Deps.Organize.BuildPulseGraph(filteredNodes, filteredEvs, a.SleepInsights, service.PulseGraphOptions{
		ShowTags:      p.ShowTags,
		ShowEvidences: p.ShowEvidences,
	})
	p.CachedVaultGen = a.VaultGen
	p.CachedQuery = q
	p.CachedTag = tag
	p.CachedShowTags = p.ShowTags
	p.CachedShowEvidences = p.ShowEvidences
	return p.Graph
}

func (a *App) renderArchivePulseSplit(c *ui.Context, phone bool) {
	pg := a.ensurePulseGraph()
	winW, winH := c.Size()
	if winW <= 0 {
		winW = 412
	}
	if winH <= 0 {
		winH = 860
	}

	paneW := winW
	if !phone {
		paneW = 440
	}
	availH := max(float32(420), winH-118)
	ratio := min(float32(0.68), max(float32(0.26), a.Pulse.SplitRatio))
	stageH := max(float32(190), min(availH-170, availH*ratio))

	ui.Column(c).Key("pulse_obsidian_split").Grow(1).FillWidth().MinHeight(0).Background(obsBgTree).Children(func() {
		if strings.TrimSpace(a.SearchQuery) != "" || a.ArchiveSearchOpen {
			a.renderDarkPulseSearchBar(c)
		}

		a.renderObsidianGraphStage(c, pg, paneW, stageH, phone)
		a.renderPulseSplitGrabber(c, availH)
		a.renderObsidianFileExplorer(c, pg, phone)
	})
}

func (a *App) renderDarkPulseSearchBar(c *ui.Context) {
	q := strings.TrimSpace(a.SearchQuery)
	ui.Box(c).FillWidth().Padding(6, 16, 8, 16).Background(obsBgGraph).
		BorderWidth(0, 0, 1, 0).BorderColor(obsBorder).
		Children(func() {
			ui.Row(c).FillWidth().Padding(7, 11).Gap(8).Radius(8).AlignItems(ui.Center).
				Background(obsBgElevated).Border(1, ui.Hex("#34343D")).
				Role(ui.RoleSearch).
				Children(func() {
					ui.Icon(c, IconSearch).Size(13.5, 13.5).TextColor(obsTextDim).Shrink(0)
					ui.TextInputBase(c, &a.SearchQuery).
						Placeholder("筛选脉络节点与文件夹…").
						PlaceholderColor(obsTextDim).
						TextColor(obsTextBright).
						FontSize(12.5).
						Label("搜索脉络").
						Search().
						Grow(1).MinWidth(0)
					if q != "" {
						clr := ui.ButtonBase(c).Key("pulse_search_clear").
							Size(20, 20).Radius(5).Center().
							Background(ui.Transparent).Cursor(ui.CursorPointer).Label("清空搜索")
						clr.OnClick(func() {
							a.SearchQuery = ""
						}).Children(func() {
							ui.Icon(c, IconClose).Size(12, 12).TextColor(obsTextMuted)
						})
					}
				})
		})
}

type projectedPulseVertex struct {
	Idx       int
	Node      service.PulseGraphNode
	SX, SY    float32
	Radius    float32
	IsFocus   bool
	IsNeigh   bool
	InBounds  bool
}

func (a *App) renderObsidianGraphStage(c *ui.Context, pg service.PulseGraph, estW, stageH float32, phone bool) {
	stage := ui.Box(c).Key("pulse_graph_stage").FillWidth().Height(stageH).
		Background(obsBgGraph).Clip().
		BorderWidth(0, 0, 1, 0).BorderColor(obsBorder)

	bounds := stage.Bounds()
	stageW := bounds.W
	if stageW <= 0 {
		stageW = estW
	}

	if len(pg.Nodes) == 0 {
		stage.Center().Children(func() {
			ui.Column(c).Center().Gap(8).Padding(24).Children(func() {
				ui.Icon(c, IconConstellation).Size(22, 22).TextColor(obsTextDim)
				ui.Text(c, "还没有匹配的脉络节点").FontSize(13).FontWeight(500).TextColor(obsTextMuted)
			})
		})
		return
	}

	// Resolve focused vertex index (either a focused Tag or the selected IdeaNode).
	focusIdx := -1
	focusAccent := obsAccent
	if a.Pulse.FocusedTag != "" {
		tagID := "tag:" + a.Pulse.FocusedTag
		for i, nd := range pg.Nodes {
			if nd.ID == tagID {
				focusIdx = i
				focusAccent = obsEmerald
				break
			}
		}
	}
	if focusIdx < 0 && a.SelectedID != "" {
		if idx, ok := pg.NoteIndexByID[a.SelectedID]; ok {
			focusIdx = idx
		}
	}
	neighMap := pg.NeighborsByIdx[focusIdx]

	zoom := min(float32(2.6), max(float32(0.55), a.Pulse.Zoom))
	panX := a.Pulse.PanX
	panY := a.Pulse.PanY
	cx := stageW * 0.5
	cy := stageH * 0.48
	usableW := stageW * 0.86
	usableH := stageH * 0.84

	proj := make([]projectedPulseVertex, len(pg.Nodes))
	for i, nd := range pg.Nodes {
		sx := cx + (float32(nd.X)-0.5)*usableW*zoom + panX
		sy := cy + (float32(nd.Y)-0.49)*usableH*zoom + panY
		isFoc := i == focusIdx
		isNei := neighMap[i]
		rad := pulseVertexRadius(nd, isFoc)
		proj[i] = projectedPulseVertex{
			Idx:      i,
			Node:     nd,
			SX:       sx,
			SY:       sy,
			Radius:   rad,
			IsFocus:  isFoc,
			IsNeigh:  isNei,
			InBounds: sx >= -30 && sx <= stageW+30 && sy >= -30 && sy <= stageH+30,
		}
	}

	edges := pg.Edges
	hasFocus := focusIdx >= 0

	stage.Draggable(&a.Pulse.DragDummy, ui.DraggableOptions{
		Axis:   ui.Horizontal | ui.Vertical,
		Bounds: &ui.Rect{},
		Rigid:  true,
	}).HandleInput(func(ev ui.InputEvent) bool {
		return a.handlePulseGraphInput(ev, pg, proj, stageW, stageH, phone)
	}).Draw(func(p *ui.Painter, r ui.Rect) {
		drawPulseObsidianMesh(p, r, stageW, stageH, proj, edges, focusIdx, hasFocus, focusAccent)
	}).Children(func() {
		// Zoom-aware plain-text labels (zero background pill boxes, automatic collision culling)
		renderPulsePlainLabels(c, stageW, stageH, zoom, proj)

		// Top-right discreet filter button (Obsidian Graph Settings style)
		fltBtn := ui.ButtonBase(c).Key("pulse_filter_btn").
			Absolute().Right(10).Top(8).Size(28, 28).Radius(6).Center().
			Background(ui.Transparent).Cursor(ui.CursorPointer).Label("图谱显示设置")
		mobile.Ink(fltBtn)
		fltBtn.OnClick(func() {
			mygo.Haptics.Selection()
			a.Pulse.FilterPopoverOpen = !a.Pulse.FilterPopoverOpen
		}).Children(func() {
			icCol := obsTextDim
			if a.Pulse.FilterPopoverOpen {
				icCol = obsTextBright
			}
			ui.Icon(c, IconSliders).Size(14, 14).TextColor(icCol)
		})

		if a.Pulse.FilterPopoverOpen {
			a.renderPulseFilterOverlay(c)
		}

		// Bottom-left quick open action on phone when a note is focused
		if phone && a.SelectedID != "" {
			if selNode, ok := a.NodeByID[a.SelectedID]; ok {
				openBtn := ui.ButtonBase(c).Key("pulse_open_detail_btn").
					Absolute().Left(12).Bottom(7).Padding(3, 7).Radius(5).
					Background(ui.Transparent).Cursor(ui.CursorPointer).
					Label("查看选中灵感详情")
				openBtn.OnClick(func() {
					mygo.Haptics.Selection()
					a.DetailSheetOpen = true
				}).Children(func() {
					ui.Row(c).Gap(3).AlignItems(ui.Center).Children(func() {
						ui.Text(c, trimText(selNode.Title, 9)).FontSize(10.5).FontWeight(500).TextColor(obsAccent).SingleLine()
						ui.Icon(c, IconChevronRight).Size(11, 11).TextColor(obsAccent)
					})
				})
			}
		}

		// Bottom-right quiet vertex & edge count
		ui.Box(c).Absolute().Right(13).Bottom(8).PassThrough().Children(func() {
			ui.Textf(c, "%d 节点 · %d 连线", len(pg.Nodes), len(pg.Edges)).
				FontSize(10).FontFeatures("tnum").TextColor(obsTextSubtle).SingleLine()
		})
	})
}

func (a *App) renderPulseFilterOverlay(c *ui.Context) {
	ui.Column(c).Key("pulse_filter_panel").
		Absolute().Right(10).Top(40).Width(178).Padding(10, 12).Gap(8).Radius(10).
		Background(obsBgElevated).Border(1, ui.Hex("#383842")).
		Shadow(0, 8, 24, 0, ui.RGBA(0, 0, 0, 0.42)).
		ZIndex(50).
		Children(func() {
			a.renderDarkToggleRow(c, "pf_tags", "显示标签节点", a.Pulse.ShowTags, func() {
				a.Pulse.ShowTags = !a.Pulse.ShowTags
				if !a.Pulse.ShowTags {
					a.Pulse.FocusedTag = ""
				}
			})
			a.renderDarkToggleRow(c, "pf_evs", "显示原记录碎点", a.Pulse.ShowEvidences, func() {
				a.Pulse.ShowEvidences = !a.Pulse.ShowEvidences
			})

			ui.Box(c).FillWidth().Height(1).Background(obsBorder)

			resetBtn := ui.ButtonBase(c).Key("pf_reset_cam").
				FillWidth().Padding(5, 4).Radius(5).
				Background(ui.Transparent).Cursor(ui.CursorPointer).Label("重置星图视角")
			resetBtn.OnClick(func() {
				mygo.Haptics.Selection()
				a.Pulse.PanX = 0
				a.Pulse.PanY = 0
				a.Pulse.Zoom = 1.0
				a.Pulse.FocusedTag = ""
				a.Pulse.FilterPopoverOpen = false
			}).Children(func() {
				ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
					ui.Icon(c, IconRefresh).Size(12.5, 12.5).TextColor(obsTextMuted)
					ui.Text(c, "重置缩放与位移").FontSize(12).TextColor(obsTextPrimary)
				})
			})

			aiBtn := ui.ButtonBase(c).Key("pf_ai_reorg").
				FillWidth().Padding(5, 4).Radius(5).
				Background(ui.Transparent).Cursor(ui.CursorPointer).Label("AI 重新归纳目录")
			aiBtn.OnClick(func() {
				a.Pulse.FilterPopoverOpen = false
				a.RunAsync("正在用 AI 重新归纳脉络文件夹…", func(ctx context.Context) error {
					return a.Deps.Organize.ReorganizeFoldersWithAI(ctx, a.Deps.AI)
				})
			}).Children(func() {
				ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
					ui.Icon(c, IconSpark).Size(12.5, 12.5).TextColor(obsAccent)
					ui.Text(c, "AI 重新归纳目录").FontSize(12).FontWeight(500).TextColor(obsAccent)
				})
			})
		})
}

func (a *App) renderDarkToggleRow(c *ui.Context, key, label string, on bool, onToggle func()) {
	row := ui.ButtonBase(c).Key(key).FillWidth().Padding(4, 2).Radius(5).
		Background(ui.Transparent).Cursor(ui.CursorPointer).Label(label)
	row.OnClick(func() {
		mygo.Haptics.Selection()
		onToggle()
	}).Children(func() {
		ui.Row(c).FillWidth().Justify(ui.SpaceBetween).AlignItems(ui.Center).Children(func() {
			ui.Text(c, label).FontSize(12).TextColor(obsTextPrimary)
			dotBg := ui.Hex("#3A3A44")
			knobX := float32(2)
			if on {
				dotBg = obsAccent
				knobX = 14
			}
			ui.Box(c).Size(28, 16).Radius(999).Background(dotBg).Children(func() {
				ui.Box(c).Absolute().Left(knobX).Top(2).Size(12, 12).Radius(6).Background(obsTextBright)
			})
		})
	})
}

func (a *App) handlePulseGraphInput(
	ev ui.InputEvent,
	pg service.PulseGraph,
	proj []projectedPulseVertex,
	stageW, stageH float32,
	phone bool,
) bool {
	p := &a.Pulse
	switch ev.Kind {
	case ui.InputScroll:
		delta := ev.DY
		if !ev.Precise {
			delta *= 1.8
		}
		factor := float32(math.Exp(float64(-delta) * 0.0045))
		nextZoom := min(float32(2.6), max(float32(0.55), p.Zoom*factor))
		p.Zoom = nextZoom
		return true

	case ui.InputPointerDown:
		// Ignore top-right filter button zone so button clicks pass cleanly
		if ev.X >= stageW-42 && ev.Y <= 40 {
			return false
		}
		if p.FilterPopoverOpen && ev.X >= stageW-192 && ev.Y <= 210 {
			return false
		}
		if phone && ev.X <= 160 && ev.Y >= stageH-28 {
			return false
		}
		p.Dragging = true
		p.DragMoved = false
		p.DownX = ev.X
		p.DownY = ev.Y
		p.StartPanX = p.PanX
		p.StartPanY = p.PanY
		p.HitVertexIdx = hitTestPulseVertex(proj, ev.X, ev.Y)
		return true

	case ui.InputPointerMove:
		if !p.Dragging {
			return false
		}
		dx := ev.X - p.DownX
		dy := ev.Y - p.DownY
		if math.Hypot(float64(dx), float64(dy)) > 5.0 {
			p.DragMoved = true
			p.FilterPopoverOpen = false
		}
		if p.DragMoved {
			maxPanX := stageW * 0.85 * max(1.0, p.Zoom)
			maxPanY := stageH * 0.85 * max(1.0, p.Zoom)
			p.PanX = min(maxPanX, max(-maxPanX, p.StartPanX+dx))
			p.PanY = min(maxPanY, max(-maxPanY, p.StartPanY+dy))
		}
		return true

	case ui.InputPointerUp:
		if !p.Dragging {
			return false
		}
		p.Dragging = false
		if !p.DragMoved {
			if p.FilterPopoverOpen {
				p.FilterPopoverOpen = false
				return true
			}
			if p.HitVertexIdx >= 0 && p.HitVertexIdx < len(pg.Nodes) {
				nd := pg.Nodes[p.HitVertexIdx]
				mygo.Haptics.Selection()
				if nd.Kind == "tag" {
					if p.FocusedTag == nd.Tag {
						p.FocusedTag = ""
					} else {
						p.FocusedTag = nd.Tag
					}
					if nd.NodeID != "" {
						a.SelectNode(nd.NodeID)
					}
				} else if nd.NodeID != "" {
					alreadyFocused := (nd.NodeID == a.SelectedID && p.FocusedTag == "")
					p.FocusedTag = ""
					a.SelectNode(nd.NodeID)
					if alreadyFocused && phone {
						a.DetailSheetOpen = true
					}
				}
			} else {
				p.FocusedTag = ""
			}
		}
		return true
	}
	return false
}

func hitTestPulseVertex(proj []projectedPulseVertex, x, y float32) int {
	bestIdx := -1
	bestScore := float32(9999)
	for _, pv := range proj {
		dx := pv.SX - x
		dy := pv.SY - y
		dist := float32(math.Hypot(float64(dx), float64(dy)))
		hitR := max(float32(16), pv.Radius+12)
		if dist > hitR {
			continue
		}
		// Slight priority preference for Note & Tag vertices over tiny Evidence dots when overlapping
		score := dist
		if pv.Node.Kind == "evidence" {
			score += 4.5
		}
		if score < bestScore {
			bestScore = score
			bestIdx = pv.Idx
		}
	}
	return bestIdx
}

func pulseVertexRadius(nd service.PulseGraphNode, isFocused bool) float32 {
	if isFocused {
		return 5.0
	}
	switch nd.Kind {
	case "tag":
		return min(float32(3.3), 2.1+float32(nd.Degree)*0.28)
	case "evidence":
		return 1.9
	default: // "note"
		if nd.IsHub {
			return 4.5
		}
		return min(float32(4.0), 2.6+float32(nd.Degree)*0.30)
	}
}

func drawPulseObsidianMesh(
	p *ui.Painter,
	r ui.Rect,
	stageW, stageH float32,
	proj []projectedPulseVertex,
	edges []service.PulseGraphEdge,
	focusIdx int,
	hasFocus bool,
	focusAccent ui.Color,
) {
	if p == nil || r.W < 20 || r.H < 20 || len(proj) == 0 {
		return
	}
	scaleX := float32(1.0)
	scaleY := float32(1.0)
	if stageW > 0 {
		scaleX = r.W / stageW
	}
	if stageH > 0 {
		scaleY = r.H / stageH
	}

	// 1. Razor-straight 0.8px hairline edges (dimming unrelated web subtly when focused)
	for _, e := range edges {
		if e.U >= len(proj) || e.V >= len(proj) {
			continue
		}
		u := proj[e.U]
		v := proj[e.V]
		ux, uy := r.X+u.SX*scaleX, r.Y+u.SY*scaleY
		vx, vy := r.X+v.SX*scaleX, r.Y+v.SY*scaleY

		activeEdge := hasFocus && (e.U == focusIdx || e.V == focusIdx)
		var col ui.Color
		width := float32(0.80)
		if activeEdge {
			col = focusAccent.Alpha(0.72)
			width = 1.05
		} else if hasFocus {
			col = ui.RGBA(165, 170, 182, 0.16)
		} else {
			col = ui.RGBA(165, 170, 182, 0.23)
		}
		p.StrokePolyline([][2]float32{{ux, uy}, {vx, vy}}, width, col)
	}

	// 2. Flat crisp matte vertices
	for _, pv := range proj {
		nx, ny := r.X+pv.SX*scaleX, r.Y+pv.SY*scaleY
		var fill ui.Color
		switch {
		case pv.IsFocus:
			fill = focusAccent
		case pv.Node.Kind == "tag":
			if hasFocus && !pv.IsNeigh {
				fill = obsEmerald.Alpha(0.55)
			} else {
				fill = obsEmerald
			}
		case pv.Node.Kind == "evidence":
			if hasFocus && !pv.IsNeigh {
				fill = obsTextDim.Alpha(0.45)
			} else {
				fill = obsTextDim
			}
		case pv.Node.IsHub:
			if hasFocus && !pv.IsNeigh {
				fill = obsTextBright.Alpha(0.72)
			} else {
				fill = obsTextBright
			}
		case pv.IsNeigh:
			fill = ui.Hex("#D4D4D8")
		default:
			fill = obsViolet.Alpha(0.75)
		}

		if pv.IsFocus {
			var halo ui.Path
			halo.Circle(nx, ny, pv.Radius+2.8)
			p.FillPath(&halo, focusAccent.Alpha(0.18))
		}
		var dot ui.Path
		dot.Circle(nx, ny, pv.Radius)
		p.FillPath(&dot, fill)
	}
}

func renderPulsePlainLabels(c *ui.Context, stageW, stageH, zoom float32, proj []projectedPulseVertex) {
	// Priority queue matching Obsidian's Text Fade Threshold:
	// Tier 1: Focused vertex (always visible)
	// Tier 2: Hub notes (always visible)
	// Tier 3: 1-hop Note & Tag neighbors of focus (visible at zoom >= 0.75)
	// Tier 4: All other Note & Tag vertices (fade in at zoom >= 1.22)
	// Tier 5: RawEvidence peripheral snippets (fade in at zoom >= 1.62)
	var order []int
	for i, pv := range proj {
		if pv.IsFocus && pv.InBounds {
			order = append(order, i)
		}
	}
	for i, pv := range proj {
		if !pv.IsFocus && pv.Node.Kind == "note" && pv.Node.IsHub && pv.InBounds {
			order = append(order, i)
		}
	}
	if zoom >= 0.75 {
		for i, pv := range proj {
			if !pv.IsFocus && !(pv.Node.Kind == "note" && pv.Node.IsHub) && pv.IsNeigh && pv.Node.Kind != "evidence" && pv.InBounds {
				order = append(order, i)
			}
		}
	}
	if zoom >= 1.22 {
		for i, pv := range proj {
			if !pv.IsFocus && !(pv.Node.Kind == "note" && pv.Node.IsHub) && !pv.IsNeigh && pv.Node.Kind != "evidence" && pv.InBounds {
				order = append(order, i)
			}
		}
	}
	if zoom >= 1.62 {
		for i, pv := range proj {
			if !pv.IsFocus && pv.Node.Kind == "evidence" && pv.InBounds {
				order = append(order, i)
			}
		}
	}

	type rect2d struct{ x0, y0, x1, y1 float32 }
	var placed []rect2d

	for _, idx := range order {
		pv := proj[idx]
		label := pv.Node.Label
		if label == "" {
			continue
		}

		fontSize := float32(10.2)
		weight := 400
		fg := obsTextMuted
		switch {
		case pv.IsFocus:
			fontSize = 11.5
			weight = 600
			fg = obsTextBright
		case pv.Node.Kind == "note" && pv.Node.IsHub:
			fontSize = 11.0
			weight = 500
			fg = obsTextPrimary
		case pv.Node.Kind == "tag":
			fontSize = 9.8
			fg = obsTextDim
		case pv.Node.Kind == "evidence":
			fontSize = 9.5
			fg = obsTextDim
		}

		estW := measurePlainLabelWidth(label, fontSize)
		estH := float32(14)
		lx := pv.SX - estW*0.5
		if lx < 6 {
			lx = 6
		}
		if lx+estW > stageW-6 {
			lx = stageW - 6 - estW
		}
		ly := pv.SY + pv.Radius + 3.5
		if ly+estH > stageH-18 {
			ly = pv.SY - pv.Radius - estH - 2.5
		}

		cand := rect2d{lx, ly, lx + estW, ly + estH}
		collides := false
		for _, b := range placed {
			if cand.x0 < b.x1 && cand.x1 > b.x0 && cand.y0 < b.y1 && cand.y1 > b.y0 {
				collides = true
				break
			}
		}
		if collides && !pv.IsFocus {
			continue
		}
		placed = append(placed, cand)

		ui.Box(c).Key("glabel_"+pv.Node.ID).
			Absolute().Left(lx).Top(ly).Width(estW).PassThrough().
			Children(func() {
				ui.Text(c, label).FontSize(fontSize).FontWeight(weight).
					TextColor(fg).TextAlign(ui.Center).SingleLine()
			})
	}
}

func measurePlainLabelWidth(s string, fontSize float32) float32 {
	var w float32
	for _, r := range s {
		if r > unicode.MaxASCII {
			w += fontSize * 1.06
		} else {
			w += fontSize * 0.62
		}
	}
	return w + 14
}

func (a *App) renderPulseSplitGrabber(c *ui.Context, availH float32) {
	grab := ui.Box(c).Key("pulse_split_grabber").
		FillWidth().Padding(8, 0, 4, 0).Center().
		Background(obsBgTree).Cursor(ui.CursorPointer).
		Draggable(&a.Pulse.SplitDummy, ui.DraggableOptions{
			Axis:   ui.Vertical,
			Bounds: &ui.Rect{},
			Rigid:  true,
		}).
		HandleInput(func(ev ui.InputEvent) bool {
			switch ev.Kind {
			case ui.InputPointerDown:
				a.Pulse.SplitDragging = true
				a.Pulse.SplitDownY = ev.Y
				a.Pulse.SplitStartRat = a.Pulse.SplitRatio
				return true
			case ui.InputPointerMove:
				if !a.Pulse.SplitDragging || availH <= 0 {
					return false
				}
				dy := ev.Y - a.Pulse.SplitDownY
				a.Pulse.SplitRatio = min(float32(0.68), max(float32(0.26), a.Pulse.SplitStartRat+dy/availH))
				return true
			case ui.InputPointerUp:
				if !a.Pulse.SplitDragging {
					return false
				}
				a.Pulse.SplitDragging = false
				return true
			}
			return false
		})
	grab.Children(func() {
		col := ui.Hex("#3A3A42")
		if a.Pulse.SplitDragging {
			col = obsAccent
		}
		ui.Box(c).Width(32).Height(3).Radius(2).Background(col)
	})
}

func (a *App) renderObsidianFileExplorer(c *ui.Context, pg service.PulseGraph, phone bool) {
	totalNotes := 0
	for _, fd := range pg.Folders {
		totalNotes += len(fd.Notes)
	}

	ui.Column(c).Grow(1).FillWidth().MinHeight(0).Padding(2, 12, 4, 12).Gap(4).Background(obsBgTree).Children(func() {
		ui.Row(c).FillWidth().Padding(2, 8, 4, 8).Justify(ui.SpaceBetween).AlignItems(ui.Center).Children(func() {
			ui.Text(c, "AI 归档目录").FontSize(11).FontWeight(600).LetterSpacing(0.4).TextColor(obsTextDim)
			ui.Textf(c, "%d 个主题夹 · %d 篇笔记", len(pg.Folders), totalNotes).
				FontSize(11).FontFeatures("tnum").TextColor(obsTextSubtle)
		})

		ui.Scroll(c).Key("pulse_tree_scroll").TrackScroll(&a.Pulse.TreeScroll).Grow(1).FillWidth().
			HandleInput(func(ev ui.InputEvent) bool {
				return a.handleSpaceBoundaryScroll(ev, SpaceDashboard, a.Pulse.TreeScroll.Y >= a.Pulse.TreeScroll.MaxY-1)
			}).
			Children(func() {
				ui.Column(c).FillWidth().Padding(0, 0, 18, 0).Gap(2).Children(func() {
					if len(pg.Folders) == 0 {
						ui.Box(c).FillWidth().Padding(22, 8).Center().Children(func() {
							ui.Text(c, "暂无匹配的灵感文件").FontSize(12.5).TextColor(obsTextDim)
						})
						return
					}

					for _, folder := range pg.Folders {
						fd := folder
						collapsed := a.Pulse.CollapsedFolders[fd.Name]
						chev := IconChevronDown
						fCol := obsTextPrimary
						if collapsed {
							chev = IconChevronRight
							fCol = obsTextMuted
						}

						folderBtn := ui.ButtonBase(c).Key("pfolder_"+fd.Name).
							FillWidth().Padding(6, 8).Radius(6).
							Background(ui.Transparent).Cursor(ui.CursorPointer).Label(fd.Name)
						mobile.Ink(folderBtn)
						folderBtn.OnClick(func() {
							mygo.Haptics.Selection()
							a.Pulse.CollapsedFolders[fd.Name] = !a.Pulse.CollapsedFolders[fd.Name]
						}).Children(func() {
							ui.Row(c).FillWidth().Gap(6).AlignItems(ui.Center).Children(func() {
								ui.Icon(c, chev).Size(12, 12).TextColor(obsTextDim).Shrink(0)
								ui.Text(c, fd.Name).FontSize(13).FontWeight(600).TextColor(fCol).Grow(1).MinWidth(0).SingleLine()
								ui.Text(c, fmt.Sprintf("%d", len(fd.Notes))).
									FontSize(11).FontFeatures("tnum").TextColor(obsTextDim).Shrink(0)
							})
						})

						if !collapsed {
							ui.Row(c).Key("pfolder_body_"+fd.Name).FillWidth().Padding(0, 0, 4, 14).Children(func() {
								ui.Column(c).FillWidth().Children(func() {
									for _, note := range fd.Notes {
										n := note
										active := n.ID == a.SelectedID
										rowBg := ui.Transparent
										txtCol := obsTextMuted
										barCol := obsGuideLine
										barW := float32(1)
										weight := 400
										if active {
											rowBg = obsBgActiveRow
											txtCol = obsTextBright
											barCol = obsAccent
											barW = 2
											weight = 600
										}

										ui.Row(c).Key("pfile_wrap_"+n.ID).FillWidth().AlignItems(ui.Stretch).Children(func() {
											ui.Box(c).Width(barW).Background(barCol).Shrink(0)
											fileBtn := ui.ButtonBase(c).Key("pfile_"+n.ID).
												Grow(1).MinWidth(0).Margin(1, 0, 1, 6).Padding(5.5, 9).Radius(5).
												Background(rowBg).Cursor(ui.CursorPointer).Label(n.Title)
											mobile.Ink(fileBtn)
											fileBtn.OnClick(func() {
												mygo.Haptics.Selection()
												a.Pulse.FocusedTag = ""
												a.SelectNode(n.ID)
												if phone {
													a.DetailSheetOpen = true
												}
											}).Children(func() {
												ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(6).Children(func() {
													ui.Text(c, n.Title).FontSize(13).FontWeight(weight).
														TextColor(txtCol).SingleLine().Grow(1).MinWidth(0)
												})
											})
										})
									}
								})
							})
						}
					}
				})
			})
	})
}
