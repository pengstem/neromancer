package service

import (
	"context"
	"hash/fnv"
	"math"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/pengstem/neromancer/internal/ai"
	"github.com/pengstem/neromancer/internal/domain"
)

// PulseGraphOptions configures which structural layers appear in the Obsidian-style network graph.
type PulseGraphOptions struct {
	ShowTags      bool
	ShowEvidences bool
}

// PulseGraphNode represents a single vertex in the force-directed Inspiration Network.
type PulseGraphNode struct {
	ID      string  // unique graph vertex id ("note:<id>", "tag:<tag>", "ev:<id>")
	Kind    string  // "note" | "tag" | "evidence"
	NodeID  string  // associated IdeaNode.ID for focus/navigation
	Tag     string  // non-empty when Kind == "tag"
	Label   string  // plain-text label (never wrapped in pill boxes)
	Folder  string  // resolved folder cluster name
	IsHub   bool    // high-centrality anchor node
	Degree  int     // undirected link count
	X       float64 // equilibrium normalized X in [0..1]
	Y       float64 // equilibrium normalized Y in [0..1]
}

// PulseGraphEdge represents a straight hairline connection between two vertices.
type PulseGraphEdge struct {
	U    int
	V    int
	Kind string // "semantic" | "tag" | "evidence"
}

// PulseFolder represents an AI-summarized collapsible directory in the File Explorer view.
type PulseFolder struct {
	Name  string
	Notes []domain.IdeaNode
}

// PulseGraph bundles the relaxed force-directed constellation and its AI-summarized File Explorer tree.
type PulseGraph struct {
	Nodes          []PulseGraphNode
	Edges          []PulseGraphEdge
	Folders        []PulseFolder
	NoteIndexByID  map[string]int
	NeighborsByIdx map[int]map[int]bool
}

// ListAllEvidences exposes the vault's raw evidences for graph construction.
func (s *OrganizeService) ListAllEvidences() ([]domain.RawEvidence, error) {
	return s.store.ListAllEvidences()
}

// ReorganizeFoldersWithAI asks DeepSeek to re-cluster all vault IdeaNodes into natural folders and enrich cross-links.
func (s *OrganizeService) ReorganizeFoldersWithAI(ctx context.Context, aiClient *ai.Client) error {
	if aiClient == nil || !aiClient.Configured() {
		return ai.ErrMissingAPIKey
	}
	nodes, err := s.store.ListNodes()
	if err != nil || len(nodes) == 0 {
		return err
	}
	digests := make([]ai.NodeDigest, 0, len(nodes))
	byID := make(map[string]domain.IdeaNode, len(nodes))
	for _, n := range nodes {
		byID[n.ID] = n
		digests = append(digests, ai.NodeDigest{
			ID:      n.ID,
			Folder:  domain.ResolveIdeaFolder(n),
			Title:   n.Title,
			Summary: n.ConversationalSummary,
			Tags:    n.Tags,
		})
	}
	assignments, err := aiClient.SummarizeVaultFolders(ctx, digests)
	if err != nil {
		return err
	}
	for _, asg := range assignments {
		n, ok := byID[asg.NodeID]
		if !ok {
			continue
		}
		if f := strings.TrimSpace(asg.Folder); f != "" {
			n.Folder = f
		}
		if len(asg.RelatedIDs) > 0 {
			n.RelatedIDs = filterValidOtherIDs(n.ID, mergeUniqueStrings(n.RelatedIDs, asg.RelatedIDs), byID)
		}
		if err := s.store.UpsertNode(n); err != nil {
			return err
		}
		byID[n.ID] = n
	}
	return nil
}

// BuildPulseGraph builds the Obsidian-style force-directed network and AI-summarized folder tree
// from the current vault IdeaNodes, RawEvidences, and SleepInsights.
func (s *OrganizeService) BuildPulseGraph(
	nodes []domain.IdeaNode,
	evidences []domain.RawEvidence,
	insights []domain.SleepInsight,
	opts PulseGraphOptions,
) PulseGraph {
	pg := PulseGraph{
		NoteIndexByID:  make(map[string]int, len(nodes)),
		NeighborsByIdx: make(map[int]map[int]bool),
	}
	if len(nodes) == 0 {
		return pg
	}

	// 1. Group IdeaNodes into clean AI-summarized folders ("文件列表页的形状").
	pg.Folders = buildPulseFolders(nodes)

	folderSector := make(map[string]float64, len(pg.Folders))
	for i, fd := range pg.Folders {
		folderSector[fd.Name] = (2.0 * math.Pi * float64(i)) / float64(max(1, len(pg.Folders)))
	}

	// 2. Add Note vertices first.
	byID := make(map[string]domain.IdeaNode, len(nodes))
	for _, n := range nodes {
		byID[n.ID] = n
		folder := domain.ResolveIdeaFolder(n)
		x0, y0 := seededClusterPos("note:"+n.ID, folderSector[folder], 0.42, 0.22)
		idx := len(pg.Nodes)
		pg.NoteIndexByID[n.ID] = idx
		pg.Nodes = append(pg.Nodes, PulseGraphNode{
			ID:     "note:" + n.ID,
			Kind:   "note",
			NodeID: n.ID,
			Label:  compactGraphNoteLabel(n.Title),
			Folder: folder,
			X:      x0,
			Y:      y0,
		})
	}

	edgeSeen := make(map[[2]int]bool)
	addEdge := func(u, v int, kind string) {
		if u == v || u < 0 || v < 0 || u >= len(pg.Nodes) || v >= len(pg.Nodes) {
			return
		}
		a, b := u, v
		if a > b {
			a, b = b, a
		}
		key := [2]int{a, b}
		if edgeSeen[key] {
			return
		}
		edgeSeen[key] = true
		pg.Edges = append(pg.Edges, PulseGraphEdge{U: u, V: v, Kind: kind})
		pg.Nodes[u].Degree++
		pg.Nodes[v].Degree++
		if pg.NeighborsByIdx[u] == nil {
			pg.NeighborsByIdx[u] = make(map[int]bool)
		}
		if pg.NeighborsByIdx[v] == nil {
			pg.NeighborsByIdx[v] = make(map[int]bool)
		}
		pg.NeighborsByIdx[u][v] = true
		pg.NeighborsByIdx[v][u] = true
	}

	// 3. Connect Note <-> Note via explicit RelatedIDs, ParentID, SleepInsights, and semantic/folder affinity.
	for _, n := range nodes {
		u := pg.NoteIndexByID[n.ID]
		if n.ParentID != "" {
			if v, ok := pg.NoteIndexByID[n.ParentID]; ok {
				addEdge(u, v, "semantic")
			}
		}
		for _, relID := range n.RelatedIDs {
			if v, ok := pg.NoteIndexByID[strings.TrimSpace(relID)]; ok {
				addEdge(u, v, "semantic")
			}
		}
	}
	for _, ins := range insights {
		for i := 0; i < len(ins.ConnectedNodeIDs); i++ {
			for j := i + 1; j < len(ins.ConnectedNodeIDs); j++ {
				u, okU := pg.NoteIndexByID[ins.ConnectedNodeIDs[i]]
				v, okV := pg.NoteIndexByID[ins.ConnectedNodeIDs[j]]
				if okU && okV {
					addEdge(u, v, "semantic")
				}
			}
		}
	}
	// Automatic semantic similarity & co-folder linking so newly captured thoughts naturally connect.
	for i := 0; i < len(nodes); i++ {
		for j := i + 1; j < len(nodes); j++ {
			if notesNaturallyRelated(nodes[i], nodes[j]) {
				addEdge(pg.NoteIndexByID[nodes[i].ID], pg.NoteIndexByID[nodes[j].ID], "semantic")
			}
		}
	}

	// 4. Add Obsidian-style emerald Tag vertices connecting Notes that share themes.
	if opts.ShowTags {
		type tagEntry struct {
			tag     string
			noteIDs []string
		}
		var tagList []tagEntry
		tagMap := make(map[string]int)
		for _, n := range nodes {
			seenInNode := make(map[string]bool)
			for _, rawTag := range n.Tags {
				tg := strings.TrimSpace(strings.TrimLeft(strings.TrimSpace(rawTag), "#＃"))
				if tg == "" || seenInNode[tg] {
					continue
				}
				seenInNode[tg] = true
				idx, exists := tagMap[tg]
				if !exists {
					idx = len(tagList)
					tagMap[tg] = idx
					tagList = append(tagList, tagEntry{tag: tg})
				}
				tagList[idx].noteIDs = append(tagList[idx].noteIDs, n.ID)
			}
		}
		for _, te := range tagList {
			if len(te.noteIDs) == 0 {
				continue
			}
			// Average anchor position near the notes carrying this tag
			var sumX, sumY float64
			for _, nid := range te.noteIDs {
				p := pg.Nodes[pg.NoteIndexByID[nid]]
				sumX += p.X
				sumY += p.Y
			}
			cx := sumX / float64(len(te.noteIDs))
			cy := sumY / float64(len(te.noteIDs))
			jx, jy := seededJitter("tag:"+te.tag, 0.14)
			primaryNode := byID[te.noteIDs[0]]
			tIdx := len(pg.Nodes)
			pg.Nodes = append(pg.Nodes, PulseGraphNode{
				ID:     "tag:" + te.tag,
				Kind:   "tag",
				NodeID: primaryNode.ID,
				Tag:    te.tag,
				Label:  te.tag,
				Folder: domain.ResolveIdeaFolder(primaryNode),
				X:      cx + jx,
				Y:      cy + jy,
			})
			for _, nid := range te.noteIDs {
				addEdge(tIdx, pg.NoteIndexByID[nid], "tag")
			}
		}
	}

	// 5. Add RawEvidence capture fragments as peripheral satellite dots tethered to their IdeaNode.
	if opts.ShowEvidences {
		for _, ev := range evidences {
			nIdx, ok := pg.NoteIndexByID[ev.NodeID]
			if !ok {
				continue
			}
			parent := pg.Nodes[nIdx]
			jx, jy := seededJitter("ev:"+ev.ID, 0.22)
			eIdx := len(pg.Nodes)
			pg.Nodes = append(pg.Nodes, PulseGraphNode{
				ID:     "ev:" + ev.ID,
				Kind:   "evidence",
				NodeID: ev.NodeID,
				Label:  compactEvidenceLabel(ev),
				Folder: parent.Folder,
				X:      parent.X + jx,
				Y:      parent.Y + jy,
			})
			addEdge(eIdx, nIdx, "evidence")
		}
	}

	// Mark Hub status: top-degree note in each folder (or any note with degree >= 4).
	for _, fd := range pg.Folders {
		bestIdx := -1
		bestDeg := -1
		for _, n := range fd.Notes {
			idx := pg.NoteIndexByID[n.ID]
			if pg.Nodes[idx].Degree > bestDeg {
				bestDeg = pg.Nodes[idx].Degree
				bestIdx = idx
			}
			if pg.Nodes[idx].Degree >= 4 {
				pg.Nodes[idx].IsHub = true
			}
		}
		if bestIdx >= 0 {
			pg.Nodes[bestIdx].IsHub = true
		}
	}

	// 6. Run deterministic Fruchterman-Reingold force-directed relaxation to equilibrium.
	relaxPulseGraph(&pg)
	return pg
}

func relaxPulseGraph(pg *PulseGraph) {
	n := len(pg.Nodes)
	if n == 0 {
		return
	}
	if n == 1 {
		pg.Nodes[0].X = 0.5
		pg.Nodes[0].Y = 0.48
		return
	}

	vx := make([]float64, n)
	vy := make([]float64, n)
	const iters = 120

	for step := 0; step < iters; step++ {
		temp := 0.044 * (1.0 - float64(step)/float64(iters)*0.86)

		// All-pairs Coulomb repulsion (stronger between Note hubs so major clusters breathe cleanly)
		for i := 0; i < n; i++ {
			for j := i + 1; j < n; j++ {
				dx := pg.Nodes[i].X - pg.Nodes[j].X
				dy := pg.Nodes[i].Y - pg.Nodes[j].Y
				d2 := dx*dx + dy*dy + 0.0038
				d := math.Sqrt(d2)
				repK := 0.012
				if pg.Nodes[i].Kind == "note" && pg.Nodes[j].Kind == "note" {
					repK = 0.024
				}
				rep := repK / d2
				fx, fy := (dx/d)*rep, (dy/d)*rep
				vx[i] += fx
				vy[i] += fy
				vx[j] -= fx
				vy[j] -= fy
			}
		}

		// Hooke spring attraction along edges
		for _, e := range pg.Edges {
			dx := pg.Nodes[e.V].X - pg.Nodes[e.U].X
			dy := pg.Nodes[e.V].Y - pg.Nodes[e.U].Y
			d := math.Hypot(dx, dy) + 0.0001
			restLen := 0.22
			stiff := 0.15
			switch e.Kind {
			case "evidence":
				restLen = 0.14
				stiff = 0.20
			case "tag":
				restLen = 0.18
				stiff = 0.17
			}
			force := (d - restLen) * stiff
			fx, fy := (dx/d)*force, (dy/d)*force
			vx[e.U] += fx
			vy[e.U] += fy
			vx[e.V] -= fx
			vy[e.V] -= fy
		}

		// Gentle center gravity keeping disconnected orphans peacefully in frame
		for i := 0; i < n; i++ {
			vx[i] -= pg.Nodes[i].X * 0.032
			vy[i] -= pg.Nodes[i].Y * 0.038
			mag := math.Hypot(vx[i], vy[i])
			if mag > temp {
				vx[i] = (vx[i] / mag) * temp
				vy[i] = (vy[i] / mag) * temp
			}
			pg.Nodes[i].X += vx[i]
			pg.Nodes[i].Y += vy[i]
			vx[i] *= 0.38
			vy[i] *= 0.38
		}
	}

	minX, maxX := pg.Nodes[0].X, pg.Nodes[0].X
	minY, maxY := pg.Nodes[0].Y, pg.Nodes[0].Y
	for _, nd := range pg.Nodes {
		minX = min(minX, nd.X)
		maxX = max(maxX, nd.X)
		minY = min(minY, nd.Y)
		maxY = max(maxY, nd.Y)
	}
	spanX := max(0.01, maxX-minX)
	spanY := max(0.01, maxY-minY)
	for i := range pg.Nodes {
		pg.Nodes[i].X = 0.10 + ((pg.Nodes[i].X-minX)/spanX)*0.80
		pg.Nodes[i].Y = 0.12 + ((pg.Nodes[i].Y-minY)/spanY)*0.74
	}
}

func buildPulseFolders(nodes []domain.IdeaNode) []PulseFolder {
	type folderAcc struct {
		name      string
		notes     []domain.IdeaNode
		latestAt  int64
		isDefault bool
	}
	accMap := make(map[string]*folderAcc)
	var keys []string

	for _, n := range nodes {
		fName := domain.ResolveIdeaFolder(n)
		acc, ok := accMap[fName]
		if !ok {
			acc = &folderAcc{
				name:      fName,
				isDefault: fName == "最近散点闪念",
			}
			accMap[fName] = acc
			keys = append(keys, fName)
		}
		acc.notes = append(acc.notes, n)
		ts := n.UpdatedAt.UnixNano()
		if ts > acc.latestAt {
			acc.latestAt = ts
		}
	}

	sort.Slice(keys, func(i, j int) bool {
		a, b := accMap[keys[i]], accMap[keys[j]]
		if a.isDefault != b.isDefault {
			return !a.isDefault
		}
		if len(a.notes) != len(b.notes) {
			return len(a.notes) > len(b.notes)
		}
		if a.latestAt != b.latestAt {
			return a.latestAt > b.latestAt
		}
		return a.name < b.name
	})

	out := make([]PulseFolder, 0, len(keys))
	for _, k := range keys {
		acc := accMap[k]
		sort.Slice(acc.notes, func(i, j int) bool {
			if !acc.notes[i].UpdatedAt.Equal(acc.notes[j].UpdatedAt) {
				return acc.notes[i].UpdatedAt.After(acc.notes[j].UpdatedAt)
			}
			return acc.notes[i].ID < acc.notes[j].ID
		})
		out = append(out, PulseFolder{
			Name:  acc.name,
			Notes: acc.notes,
		})
	}
	return out
}

func notesNaturallyRelated(a, b domain.IdeaNode) bool {
	// 1. Shared normalized tag (excluding generic fallback modality tags)
	genericTags := map[string]bool{
		"随口一记": true, "语音闪念": true, "图像灵感": true, "手绘草图": true,
	}
	tagSet := make(map[string]bool, len(a.Tags))
	for _, t := range a.Tags {
		clean := strings.TrimSpace(strings.TrimLeft(strings.TrimSpace(t), "#＃"))
		if clean != "" && !genericTags[clean] {
			tagSet[clean] = true
		}
	}
	for _, t := range b.Tags {
		clean := strings.TrimSpace(strings.TrimLeft(strings.TrimSpace(t), "#＃"))
		if tagSet[clean] {
			return true
		}
	}

	// 2. Shared non-default AI folder AND shared icon motif or concept token
	fa := domain.ResolveIdeaFolder(a)
	fb := domain.ResolveIdeaFolder(b)
	if fa == fb && fa != "最近散点闪念" {
		return true
	}

	// 3. Shared extracted concept phrase
	ca := extractNodeConcepts(a)
	cb := extractNodeConcepts(b)
	cSet := make(map[string]bool, len(ca))
	for _, c := range ca {
		cSet[c] = true
	}
	for _, c := range cb {
		if cSet[c] {
			return true
		}
	}
	return false
}

func seededClusterPos(seedKey string, sectorAngle, radius, jitter float64) (float64, float64) {
	cx := math.Cos(sectorAngle) * radius
	cy := math.Sin(sectorAngle) * radius
	jx, jy := seededJitter(seedKey, jitter)
	return cx + jx, cy + jy
}

func seededJitter(seedKey string, amp float64) (float64, float64) {
	h := fnv.New64a()
	_, _ = h.Write([]byte(seedKey))
	v := h.Sum64()
	angle := float64(v&0xFFFF) / 65535.0 * 2.0 * math.Pi
	dist := (0.25 + 0.75*(float64((v>>16)&0xFFFF)/65535.0)) * amp
	return math.Cos(angle) * dist, math.Sin(angle) * dist
}

func compactGraphNoteLabel(title string) string {
	s := strings.TrimSpace(strings.TrimRight(strings.TrimSpace(title), "？?！!。"))
	if utf8.RuneCountInString(s) <= 15 {
		return s
	}
	r := []rune(s)
	return string(r[:14]) + "…"
}

func compactEvidenceLabel(ev domain.RawEvidence) string {
	raw := strings.TrimSpace(ev.RawText)
	if raw == "" {
		return ev.Scene.LocationSummaryLine()
	}
	r := []rune(raw)
	if len(r) <= 8 {
		return string(r)
	}
	return string(r[:8]) + "…"
}

func filterValidOtherIDs(selfID string, ids []string, valid map[string]domain.IdeaNode) []string {
	var out []string
	seen := make(map[string]bool)
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" || id == selfID || seen[id] {
			continue
		}
		if _, ok := valid[id]; ok {
			seen[id] = true
			out = append(out, id)
		}
	}
	return out
}
