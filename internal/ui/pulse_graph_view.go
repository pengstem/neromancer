package ui

import (
	"context"
	"fmt"
	"hash/fnv"
	"math"
	"strings"
	"time"
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
	obsViolet      = ui.Hex("#9B8AFB") // Secondary note tint
)

type pulseSimNode struct {
	ID        string
	X, Y      float64
	VX, VY    float64
	Phase     float64
	Appear    float32
	FocusGlow float32
	Pinned    bool
}

// PulseViewState holds pure frame-persistent physics, animation, and UI state for the Obsidian Graph + AI File Explorer.
type PulseViewState struct {
	ShowTags          bool
	ShowEvidences     bool
	FilterPopoverOpen bool

	PanX       float32
	PanY       float32
	PanVelX    float32
	PanVelY    float32
	Zoom       float32
	SplitRatio float32 // fraction of body height given to Graph stage (0.26 .. 0.68)

	FocusedTag       string
	CollapsedFolders map[string]bool
	TreeScroll       ui.ScrollState

	// Live spring-physics & organic breathing state
	SimNodes        []pulseSimNode
	LastSimTick     time.Time
	BreathTime      float64
	FocusPresence   float32
	LastStageBounds ui.Rect
	LastProj        []projectedPulseVertex

	// Pointer gesture tracking on Graph stage & Split handle
	DragDummy       ui.Point
	SplitDummy      ui.Point
	Dragging        bool
	DragMoved       bool
	DownX           float32
	DownY           float32
	LastPtrX        float32
	LastPtrY        float32
	LastPtrAt       time.Time
	StartPanX       float32
	StartPanY       float32
	HitVertexIdx    int
	DraggingNodeIdx int
	SplitDragging   bool
	SplitDownY      float32
	SplitStartRat   float32

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
		ShowTags:         false, // Default OFF so unrelated notes stand cleanly as separate islands without tag tentacles
		ShowEvidences:    true,  // Shows merged raw capture dots when a note aggregates 2+ captures
		Zoom:             1.0,
		SplitRatio:       0.46,
		HitVertexIdx:     -1,
		DraggingNodeIdx:  -1,
		CollapsedFolders: make(map[string]bool),
	}
}

func (a *App) ensurePulseGraph(reduceMotion bool) service.PulseGraph {
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
		p.SplitRatio = 0.46
	}

	if p.CachedVaultGen == a.VaultGen &&
		p.CachedQuery == q &&
		p.CachedTag == tag &&
		p.CachedShowTags == p.ShowTags &&
		p.CachedShowEvidences == p.ShowEvidences {
		if len(p.SimNodes) != len(p.Graph.Nodes) {
			p.syncSimNodes(p.Graph, reduceMotion)
		}
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
	p.syncSimNodes(p.Graph, reduceMotion)
	return p.Graph
}

func (p *PulseViewState) syncSimNodes(pg service.PulseGraph, reduceMotion bool) {
	prevByID := make(map[string]pulseSimNode, len(p.SimNodes))
	for _, sn := range p.SimNodes {
		prevByID[sn.ID] = sn
	}

	next := make([]pulseSimNode, len(pg.Nodes))
	for i, nd := range pg.Nodes {
		if old, ok := prevByID[nd.ID]; ok {
			if reduceMotion {
				old.X = nd.X
				old.Y = nd.Y
				old.VX = 0
				old.VY = 0
				old.Appear = 1.0
			}
			next[i] = old
			continue
		}
		phase := hashNodePhase(nd.ID)
		if reduceMotion {
			next[i] = pulseSimNode{
				ID:     nd.ID,
				X:      nd.X,
				Y:      nd.Y,
				Phase:  phase,
				Appear: 1.0,
			}
		} else {
			// New or freshly-opened nodes gently bloom outward from their island center!
			startX := nd.AnchorX + (nd.X-nd.AnchorX)*0.42
			startY := nd.AnchorY + (nd.Y-nd.AnchorY)*0.42
			next[i] = pulseSimNode{
				ID:     nd.ID,
				X:      startX,
				Y:      startY,
				Phase:  phase,
				Appear: 0.20,
			}
		}
	}
	p.SimNodes = next
	p.DraggingNodeIdx = -1
}

func hashNodePhase(id string) float64 {
	h := fnv.New32a()
	_, _ = h.Write([]byte(id))
	return (float64(h.Sum32()&0xFFFF) / 65535.0) * 2.0 * math.Pi
}

// stepPulsePhysics advances live Obsidian spring elasticity, coasting pan inertia,
// smooth focus transitions, and serene ambient breathing.
func (a *App) stepPulsePhysics(
	c *ui.Context,
	pg service.PulseGraph,
	focusIdx int,
	neighMap map[int]bool,
	stageW, stageH float32,
) (kineticActive bool, breathActive bool) {
	p := &a.Pulse
	n := len(pg.Nodes)
	if n == 0 || len(p.SimNodes) != n {
		return false, false
	}

	reduceMotion := c == nil || c.Preferences().ReduceMotion
	if reduceMotion {
		for i, nd := range pg.Nodes {
			p.SimNodes[i].X = nd.X
			p.SimNodes[i].Y = nd.Y
			p.SimNodes[i].VX = 0
			p.SimNodes[i].VY = 0
			p.SimNodes[i].Appear = 1.0
			switch {
			case i == focusIdx:
				p.SimNodes[i].FocusGlow = 1.0
			case neighMap[i]:
				p.SimNodes[i].FocusGlow = 0.55
			default:
				p.SimNodes[i].FocusGlow = 0.0
			}
		}
		if focusIdx >= 0 {
			p.FocusPresence = 1.0
		} else {
			p.FocusPresence = 0.0
		}
		p.PanVelX = 0
		p.PanVelY = 0
		return false, false
	}

	now := c.Now()
	dt := 0.016
	if !p.LastSimTick.IsZero() && !now.Before(p.LastSimTick) {
		elapsed := now.Sub(p.LastSimTick).Seconds()
		if elapsed > 0 && elapsed < 0.05 {
			dt = elapsed
		} else if elapsed >= 0.05 {
			dt = 0.032
		}
	}
	p.LastSimTick = now
	p.BreathTime += dt

	// 1. Pan coasting inertia when not actively dragging
	if !p.Dragging && (math.Abs(float64(p.PanVelX)) > 2.0 || math.Abs(float64(p.PanVelY)) > 2.0) {
		maxPanX := stageW * 0.85 * max(1.0, p.Zoom)
		maxPanY := stageH * 0.85 * max(1.0, p.Zoom)
		p.PanX = min(maxPanX, max(-maxPanX, p.PanX+p.PanVelX*float32(dt)))
		p.PanY = min(maxPanY, max(-maxPanY, p.PanY+p.PanVelY*float32(dt)))
		decay := float32(math.Exp(-5.2 * dt))
		p.PanVelX *= decay
		p.PanVelY *= decay
		kineticActive = true
	} else if !p.Dragging {
		p.PanVelX = 0
		p.PanVelY = 0
	}

	// 2. Live spring-repulsion physics (especially responsive when dragging a node!)
	fx := make([]float64, n)
	fy := make([]float64, n)

	// Spring attraction to each node's island equilibrium rest position (near-critically damped)
	anchorStiff := 24.0
	if p.Dragging && p.DraggingNodeIdx >= 0 {
		// Loosen equilibrium tether on same-island nodes so dragged island stretches organically!
		anchorStiff = 6.5
	}
	draggedIsland := -1
	if p.Dragging && p.DraggingNodeIdx >= 0 && p.DraggingNodeIdx < n {
		draggedIsland = pg.Nodes[p.DraggingNodeIdx].IslandID
	}

	for i, nd := range pg.Nodes {
		if p.SimNodes[i].Pinned {
			continue
		}
		k := anchorStiff
		if draggedIsland >= 0 && nd.IslandID != draggedIsland {
			k = 26.0 // Unrelated islands stay peacefully anchored in their own territory
		}
		fx[i] += (nd.X - p.SimNodes[i].X) * k
		fy[i] += (nd.Y - p.SimNodes[i].Y) * k
	}

	// Live Hooke springs along connected edges so pulling one node tugs its whole island!
	for _, e := range pg.Edges {
		u, v := e.U, e.V
		if u >= n || v >= n {
			continue
		}
		restDX := pg.Nodes[v].X - pg.Nodes[u].X
		restDY := pg.Nodes[v].Y - pg.Nodes[u].Y
		restLen := math.Hypot(restDX, restDY)

		curDX := p.SimNodes[v].X - p.SimNodes[u].X
		curDY := p.SimNodes[v].Y - p.SimNodes[u].Y
		curLen := math.Hypot(curDX, curDY) + 0.0001

		stretch := (curLen - restLen) * 36.0
		sx := (curDX / curLen) * stretch
		sy := (curDY / curLen) * stretch
		if !p.SimNodes[u].Pinned {
			fx[u] += sx
			fy[u] += sy
		}
		if !p.SimNodes[v].Pinned {
			fx[v] -= sx
			fy[v] -= sy
		}
	}

	// Soft short-range collision separation preventing dots from sitting directly on top of each other
	for i := 0; i < n; i++ {
		for j := i + 1; j < n; j++ {
			dx := p.SimNodes[i].X - p.SimNodes[j].X
			dy := p.SimNodes[i].Y - p.SimNodes[j].Y
			d2 := dx*dx + dy*dy
			if d2 > 0.006 || d2 < 1e-7 {
				continue
			}
			d := math.Sqrt(d2)
			rep := (0.006 - d2) * 22.0
			rx, ry := (dx/d)*rep, (dy/d)*rep
			if !p.SimNodes[i].Pinned {
				fx[i] += rx
				fy[i] += ry
			}
			if !p.SimNodes[j].Pinned {
				fx[j] -= rx
				fy[j] -= ry
			}
		}
	}

	damping := math.Exp(-9.2 * dt)
	totalSpeed := 0.0
	appearRate := float32(1.0 - math.Exp(-7.5*dt))
	glowRate := float32(1.0 - math.Exp(-11.0*dt))

	for i := range p.SimNodes {
		sn := &p.SimNodes[i]
		if !sn.Pinned {
			sn.VX = (sn.VX + fx[i]*dt) * damping
			sn.VY = (sn.VY + fy[i]*dt) * damping
			sn.X = min(0.95, max(0.05, sn.X+sn.VX*dt))
			sn.Y = min(0.93, max(0.07, sn.Y+sn.VY*dt))
			totalSpeed += math.Hypot(sn.VX, sn.VY)
		} else {
			sn.VX = 0
			sn.VY = 0
		}

		if sn.Appear < 0.995 {
			sn.Appear += (1.0 - sn.Appear) * appearRate
			kineticActive = true
		} else {
			sn.Appear = 1.0
		}

		targetGlow := float32(0.0)
		if i == focusIdx {
			targetGlow = 1.0
		} else if neighMap[i] {
			targetGlow = 0.55
		}
		if math.Abs(float64(targetGlow-sn.FocusGlow)) > 0.01 {
			sn.FocusGlow += (targetGlow - sn.FocusGlow) * glowRate
			kineticActive = true
		} else {
			sn.FocusGlow = targetGlow
		}
	}

	targetPresence := float32(0.0)
	if focusIdx >= 0 {
		targetPresence = 1.0
	}
	if math.Abs(float64(targetPresence-p.FocusPresence)) > 0.01 {
		p.FocusPresence += (targetPresence - p.FocusPresence) * glowRate
		kineticActive = true
	} else {
		p.FocusPresence = targetPresence
	}

	if totalSpeed > 0.008 || (p.Dragging && p.DraggingNodeIdx >= 0) {
		kineticActive = true
	}
	return kineticActive, true
}

func (a *App) renderArchivePulseSplit(c *ui.Context, phone bool) {
	reduceMotion := c.Preferences().ReduceMotion
	pg := a.ensurePulseGraph(reduceMotion)
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
	stageH := max(float32(195), min(availH-170, availH*ratio))

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
	Phase     float64
	Pinned    bool
	Radius    float32
	Appear    float32
	FocusGlow float32
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

	// Resolve focused vertex index.
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

	kineticActive, breathActive := a.stepPulsePhysics(c, pg, focusIdx, neighMap, stageW, stageH)
	if kineticActive {
		c.AnimationFrame()
	}
	reduceMotion := c.Preferences().ReduceMotion
	animOrigin := a.AnimOrigin
	if animOrigin.IsZero() {
		animOrigin = c.Now()
		a.AnimOrigin = animOrigin
	}

	zoom := min(float32(2.6), max(float32(0.55), a.Pulse.Zoom))
	panX := a.Pulse.PanX
	panY := a.Pulse.PanY
	cx := stageW * 0.5
	cy := stageH * 0.47
	usableW := stageW * 0.84
	usableH := stageH * 0.80

	proj := make([]projectedPulseVertex, len(pg.Nodes))
	for i, nd := range pg.Nodes {
		sn := a.Pulse.SimNodes[i]
		sx := cx + (float32(sn.X)-0.5)*usableW*zoom + panX
		sy := cy + (float32(sn.Y)-0.48)*usableH*zoom + panY

		isFoc := i == focusIdx
		isNei := neighMap[i]
		baseRad := pulseVertexRadius(nd, isFoc)
		rad := baseRad * (0.55 + 0.45*sn.Appear)
		if sn.FocusGlow > 0.02 {
			rad += 0.65 * sn.FocusGlow
		}

		proj[i] = projectedPulseVertex{
			Idx:       i,
			Node:      nd,
			SX:        sx,
			SY:        sy,
			Phase:     sn.Phase,
			Pinned:    sn.Pinned,
			Radius:    rad,
			Appear:    sn.Appear,
			FocusGlow: sn.FocusGlow,
			IsFocus:   isFoc,
			IsNeigh:   isNei,
			InBounds:  sx >= -30 && sx <= stageW+30 && sy >= -30 && sy <= stageH+30,
		}
	}
	a.Pulse.LastStageBounds = bounds
	a.Pulse.LastProj = proj

	edges := pg.Edges
	focusPresence := a.Pulse.FocusPresence

	stage.Draggable(&a.Pulse.DragDummy, ui.DraggableOptions{
		Axis:   ui.Horizontal | ui.Vertical,
		Bounds: &ui.Rect{},
		Rigid:  true,
	}).HandleInput(func(ev ui.InputEvent) bool {
		return a.handlePulseGraphInput(ev, pg, proj, stageW, stageH, cx, cy, usableW, usableH, zoom, phone)
	}).Draw(func(p *ui.Painter, r ui.Rect) {
		liveT := a.Pulse.BreathTime
		if !reduceMotion && !animOrigin.IsZero() {
			if el := p.Now().Sub(animOrigin).Seconds(); el > 0 {
				liveT = el
			}
		}
		drawPulseObsidianMesh(p, r, stageW, stageH, proj, edges, focusIdx, focusPresence, focusAccent, liveT, reduceMotion)
		if !kineticActive && breathActive && p.Accelerated() {
			p.AnimationFrameRate(30)
		}
	}).Children(func() {
		renderPulsePlainLabels(c, stageW, stageH, zoom, proj)

		// Top-right discreet filter button
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

		// Bottom-right quiet island, vertex & edge count
		ui.Box(c).Absolute().Right(13).Bottom(8).PassThrough().Children(func() {
			statText := fmt.Sprintf("%d 小岛 · %d 节点", pg.IslandCount, len(pg.Nodes))
			if pg.IslandCount <= 1 {
				statText = fmt.Sprintf("%d 节点 · %d 连线", len(pg.Nodes), len(pg.Edges))
			}
			ui.Text(c, statText).FontSize(10).FontFeatures("tnum").TextColor(obsTextSubtle).SingleLine()
		})
	})
}

func (a *App) renderPulseFilterOverlay(c *ui.Context) {
	ui.Column(c).Key("pulse_filter_panel").
		Absolute().Right(10).Top(40).Width(182).Padding(10, 12).Gap(8).Radius(10).
		Background(obsBgElevated).Border(1, ui.Hex("#383842")).
		Shadow(0, 8, 24, 0, ui.RGBA(0, 0, 0, 0.42)).
		ZIndex(50).
		Children(func() {
			a.renderDarkToggleRow(c, "pf_evs", "显示合并原记录点", a.Pulse.ShowEvidences, func() {
				a.Pulse.ShowEvidences = !a.Pulse.ShowEvidences
			})
			a.renderDarkToggleRow(c, "pf_tags", "显示共享标签节点", a.Pulse.ShowTags, func() {
				a.Pulse.ShowTags = !a.Pulse.ShowTags
				if !a.Pulse.ShowTags {
					a.Pulse.FocusedTag = ""
				}
			})

			ui.Box(c).FillWidth().Height(1).Background(obsBorder)

			resetBtn := ui.ButtonBase(c).Key("pf_reset_cam").
				FillWidth().Padding(5, 4).Radius(5).
				Background(ui.Transparent).Cursor(ui.CursorPointer).Label("重置星图视角")
			resetBtn.OnClick(func() {
				mygo.Haptics.Selection()
				a.Pulse.PanX = 0
				a.Pulse.PanY = 0
				a.Pulse.PanVelX = 0
				a.Pulse.PanVelY = 0
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
	stageW, stageH, cx, cy, usableW, usableH, zoom float32,
	phone bool,
) bool {
	p := &a.Pulse
	switch ev.Kind {
	case ui.InputZoom:
		if ev.Scale > 0 {
			p.Zoom = min(float32(2.6), max(float32(0.55), p.Zoom*ev.Scale))
		}
		return true

	case ui.InputScroll:
		delta := ev.DY
		if !ev.Precise {
			delta *= 1.8
		}
		factor := float32(math.Exp(float64(-delta) * 0.0045))
		p.Zoom = min(float32(2.6), max(float32(0.55), p.Zoom*factor))
		return true

	case ui.InputPointerDown:
		if ev.X >= stageW-42 && ev.Y <= 40 {
			return false
		}
		if p.FilterPopoverOpen && ev.X >= stageW-196 && ev.Y <= 210 {
			return false
		}
		if phone && ev.X <= 160 && ev.Y >= stageH-28 {
			return false
		}
		p.Dragging = true
		p.DragMoved = false
		p.DownX = ev.X
		p.DownY = ev.Y
		p.LastPtrX = ev.X
		p.LastPtrY = ev.Y
		p.LastPtrAt = ev.Time
		p.StartPanX = p.PanX
		p.StartPanY = p.PanY
		p.PanVelX = 0
		p.PanVelY = 0
		p.HitVertexIdx = hitTestPulseVertex(proj, ev.X, ev.Y)
		p.DraggingNodeIdx = p.HitVertexIdx
		if p.DraggingNodeIdx >= 0 && p.DraggingNodeIdx < len(p.SimNodes) {
			p.SimNodes[p.DraggingNodeIdx].Pinned = true
		}
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
			if p.DraggingNodeIdx >= 0 && p.DraggingNodeIdx < len(p.SimNodes) && usableW > 1 && usableH > 1 {
				// Signature Obsidian node drag: move the grabbed vertex directly, elastically pulling its island!
				nx := float64((ev.X-cx-p.PanX)/(usableW*zoom)) + 0.5
				ny := float64((ev.Y-cy-p.PanY)/(usableH*zoom)) + 0.48
				p.SimNodes[p.DraggingNodeIdx].X = min(0.95, max(0.05, nx))
				p.SimNodes[p.DraggingNodeIdx].Y = min(0.93, max(0.07, ny))
			} else {
				// Blank-space drag pans the viewport and records coasting velocity.
				if !p.LastPtrAt.IsZero() && !ev.Time.IsZero() {
					dt := float32(ev.Time.Sub(p.LastPtrAt).Seconds())
					if dt > 0.002 && dt < 0.15 {
						vx := (ev.X - p.LastPtrX) / dt
						vy := (ev.Y - p.LastPtrY) / dt
						p.PanVelX = 0.55*p.PanVelX + 0.45*vx
						p.PanVelY = 0.55*p.PanVelY + 0.45*vy
					}
				}
				maxPanX := stageW * 0.85 * max(1.0, p.Zoom)
				maxPanY := stageH * 0.85 * max(1.0, p.Zoom)
				p.PanX = min(maxPanX, max(-maxPanX, p.StartPanX+dx))
				p.PanY = min(maxPanY, max(-maxPanY, p.StartPanY+dy))
			}
		}
		p.LastPtrX = ev.X
		p.LastPtrY = ev.Y
		if !ev.Time.IsZero() {
			p.LastPtrAt = ev.Time
		}
		return true

	case ui.InputPointerUp:
		if !p.Dragging {
			return false
		}
		if p.DraggingNodeIdx >= 0 && p.DraggingNodeIdx < len(p.SimNodes) {
			p.SimNodes[p.DraggingNodeIdx].Pinned = false
		}
		p.Dragging = false
		p.DraggingNodeIdx = -1

		if !p.DragMoved {
			p.PanVelX = 0
			p.PanVelY = 0
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
		return 2.1
	default: // "note"
		if nd.IsHub && nd.Degree >= 2 {
			return 4.6
		}
		return min(float32(4.4), 3.4+float32(nd.Degree)*0.28)
	}
}

func pulseBreathOffset(pv projectedPulseVertex, breathT float64, reduceMotion bool) (float32, float32) {
	if reduceMotion || pv.Pinned {
		return 0, 0
	}
	amp := float32(2.2)
	if pv.Node.Kind == "evidence" {
		amp = 1.5
	}
	bx := amp * float32(math.Sin(breathT*0.72+pv.Phase)+0.42*math.Cos(breathT*0.41+pv.Phase*1.7))
	by := amp * float32(math.Cos(breathT*0.64+pv.Phase*1.3)+0.42*math.Sin(breathT*0.37+pv.Phase*2.1))
	return bx, by
}

func drawPulseObsidianMesh(
	p *ui.Painter,
	r ui.Rect,
	stageW, stageH float32,
	proj []projectedPulseVertex,
	edges []service.PulseGraphEdge,
	focusIdx int,
	focusPresence float32,
	focusAccent ui.Color,
	breathT float64,
	reduceMotion bool,
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

	// Precompute live GPU-animated coordinates with gentle organic breathing drift
	animX := make([]float32, len(proj))
	animY := make([]float32, len(proj))
	for i, pv := range proj {
		bx, by := pulseBreathOffset(pv, breathT, reduceMotion)
		animX[i] = r.X + (pv.SX+bx)*scaleX
		animY[i] = r.Y + (pv.SY+by)*scaleY
	}

	// 1. Razor-straight hairline edges (only within connected islands — zero forced cross-island lines!)
	for _, e := range edges {
		if e.U >= len(proj) || e.V >= len(proj) {
			continue
		}
		u := proj[e.U]
		v := proj[e.V]
		ux, uy := animX[e.U], animY[e.U]
		vx, vy := animX[e.V], animY[e.V]

		appear := min(u.Appear, v.Appear)
		activeEdge := focusIdx >= 0 && (e.U == focusIdx || e.V == focusIdx)
		var col ui.Color
		width := float32(0.85)
		if activeEdge {
			edgePulse := float32(0.70)
			if !reduceMotion {
				edgePulse = 0.64 + 0.14*float32(0.5+0.5*math.Sin(breathT*1.8))
			}
			col = focusAccent.Alpha(edgePulse * appear)
			width = 1.10
		} else {
			baseAlpha := 0.25 - 0.08*focusPresence
			col = ui.RGBA(168, 174, 186, baseAlpha*appear)
		}
		p.StrokePolyline([][2]float32{{ux, uy}, {vx, vy}}, width, col)
	}

	// 2. Flat crisp matte vertices with smooth breathing focus aura
	for i, pv := range proj {
		nx, ny := animX[i], animY[i]
		rad := pv.Radius
		if !reduceMotion && (pv.IsFocus || pv.Node.IsHub) {
			rad += 0.25 * float32(math.Sin(breathT*1.6+pv.Phase))
		}
		dimFactor := 1.0 - 0.28*focusPresence*(1.0-pv.FocusGlow*1.4)
		if dimFactor < 0.55 {
			dimFactor = 0.55
		} else if dimFactor > 1.0 {
			dimFactor = 1.0
		}
		alpha := pv.Appear * dimFactor

		var fill ui.Color
		switch {
		case pv.IsFocus:
			fill = focusAccent.Alpha(pv.Appear)
		case pv.Node.Kind == "tag":
			fill = obsEmerald.Alpha(alpha)
		case pv.Node.Kind == "evidence":
			fill = obsTextDim.Alpha(alpha)
		case pv.Node.IsHub:
			fill = obsTextBright.Alpha(alpha)
		case pv.IsNeigh:
			fill = ui.Hex("#D4D4D8").Alpha(pv.Appear)
		default:
			fill = obsViolet.Alpha(0.82 * alpha)
		}

		if pv.FocusGlow > 0.05 && pv.IsFocus {
			breathWave := float32(0.5)
			if !reduceMotion {
				breathWave = float32(0.5 + 0.5*math.Sin(breathT*1.8))
			}
			haloRad := pv.Radius + 2.6 + 1.5*breathWave
			haloAlpha := (0.13 + 0.10*breathWave) * pv.FocusGlow * pv.Appear
			var halo ui.Path
			halo.Circle(nx, ny, haloRad)
			p.FillPath(&halo, focusAccent.Alpha(haloAlpha))
		}

		var dot ui.Path
		dot.Circle(nx, ny, rad)
		p.FillPath(&dot, fill)
	}
}

func renderPulsePlainLabels(c *ui.Context, stageW, stageH, zoom float32, proj []projectedPulseVertex) {
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
	if zoom >= 0.85 {
		for i, pv := range proj {
			if !pv.IsFocus && !(pv.Node.Kind == "note" && pv.Node.IsHub) && pv.Node.Kind == "note" && pv.InBounds {
				order = append(order, i)
			}
		}
	}
	if zoom >= 1.15 {
		for i, pv := range proj {
			if !pv.IsFocus && pv.Node.Kind == "tag" && pv.InBounds {
				order = append(order, i)
			}
		}
	}
	if zoom >= 1.55 {
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

		fontSize := float32(10.5)
		weight := 400
		fg := obsTextMuted.Alpha(pv.Appear)
		switch {
		case pv.IsFocus:
			fontSize = 11.5
			weight = 600
			fg = obsTextBright.Alpha(pv.Appear)
		case pv.Node.Kind == "note" && pv.Node.IsHub:
			fontSize = 11.0
			weight = 500
			fg = obsTextPrimary.Alpha(pv.Appear)
		case pv.Node.Kind == "tag":
			fontSize = 9.8
			fg = obsTextDim.Alpha(pv.Appear)
		case pv.Node.Kind == "evidence":
			fontSize = 9.5
			fg = obsTextDim.Alpha(pv.Appear)
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
		ly := pv.SY + pv.Radius + 4.0
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
