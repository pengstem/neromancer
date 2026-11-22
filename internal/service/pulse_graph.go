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
	ID       string  // unique graph vertex id ("note:<id>", "tag:<tag>", "ev:<id>")
	Kind     string  // "note" | "tag" | "evidence"
	NodeID   string  // associated IdeaNode.ID for focus/navigation
	Tag      string  // non-empty when Kind == "tag"
	Label    string  // plain-text label (never wrapped in pill boxes)
	Folder   string  // resolved folder cluster name
	IslandID int     // connected component index (0 .. IslandCount-1)
	IsHub    bool    // high-centrality anchor node within multi-node island
	Degree   int     // undirected link count
	AnchorX  float64 // island gravity center X in normalized [0..1]
	AnchorY  float64 // island gravity center Y in normalized [0..1]
	X        float64 // equilibrium normalized X in [0..1]
	Y        float64 // equilibrium normalized Y in [0..1]
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

// PulseGraph bundles the multi-island force-directed constellation and its AI-summarized File Explorer tree.
type PulseGraph struct {
	Nodes          []PulseGraphNode
	Edges          []PulseGraphEdge
	Folders        []PulseFolder
	IslandCount    int
	NoteIndexByID  map[string]int
	NeighborsByIdx map[int]map[int]bool
}

// ListAllEvidences exposes the vault's raw evidences for graph construction.
func (s *OrganizeService) ListAllEvidences() ([]domain.RawEvidence, error) {
	return s.store.ListAllEvidences()
}

// ReorganizeFoldersWithAI asks DeepSeek to re-cluster all vault IdeaNodes into natural folders and refine cross-links.
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
		if asg.RelatedIDs != nil {
			n.RelatedIDs = filterValidOtherIDs(n.ID, asg.RelatedIDs, byID)
		}
		if err := s.store.UpsertNode(n); err != nil {
			return err
		}
		byID[n.ID] = n
	}
	return nil
}

// BuildPulseGraph builds the Obsidian-style multi-island network and AI-summarized folder tree.
// Unrelated notes stay cleanly separated as independent islands rather than being forced together.
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

	// 2. Add Note vertices.
	byID := make(map[string]domain.IdeaNode, len(nodes))
	for _, n := range nodes {
		byID[n.ID] = n
		folder := domain.ResolveIdeaFolder(n)
		idx := len(pg.Nodes)
		pg.NoteIndexByID[n.ID] = idx
		pg.Nodes = append(pg.Nodes, PulseGraphNode{
			ID:     "note:" + n.ID,
			Kind:   "note",
			NodeID: n.ID,
			Label:  compactGraphNoteLabel(n.Title),
			Folder: folder,
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

	// 3. Connect Note <-> Note ONLY when genuinely related (explicit links, SleepInsight, or specific shared concept/tag).
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
	for i := 0; i < len(nodes); i++ {
		for j := i + 1; j < len(nodes); j++ {
			if notesNaturallyRelated(nodes[i], nodes[j]) {
				addEdge(pg.NoteIndexByID[nodes[i].ID], pg.NoteIndexByID[nodes[j].ID], "semantic")
			}
		}
	}

	// 4. Optional Tag bridge vertices (only non-broad tags when user enables ShowTags).
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
				if tg == "" || seenInNode[tg] || broadUmbrellaTags[tg] {
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
			primaryNode := byID[te.noteIDs[0]]
			tIdx := len(pg.Nodes)
			pg.Nodes = append(pg.Nodes, PulseGraphNode{
				ID:     "tag:" + te.tag,
				Kind:   "tag",
				NodeID: primaryNode.ID,
				Tag:    te.tag,
				Label:  te.tag,
				Folder: domain.ResolveIdeaFolder(primaryNode),
			})
			for _, nid := range te.noteIDs {
				addEdge(tIdx, pg.NoteIndexByID[nid], "tag")
			}
		}
	}

	// 5. RawEvidence capture dots: rendered when a note merged >= 2 captures (so every merged raw thought
	// shows up as a point in that note's constellation without cluttering 1-capture standalone notes!).
	if opts.ShowEvidences {
		evsByNode := make(map[string][]domain.RawEvidence)
		for _, ev := range evidences {
			if _, ok := pg.NoteIndexByID[ev.NodeID]; ok {
				evsByNode[ev.NodeID] = append(evsByNode[ev.NodeID], ev)
			}
		}
		for nodeID, list := range evsByNode {
			if len(list) < 2 {
				continue
			}
			nIdx := pg.NoteIndexByID[nodeID]
			parent := pg.Nodes[nIdx]
			for _, ev := range list {
				eIdx := len(pg.Nodes)
				pg.Nodes = append(pg.Nodes, PulseGraphNode{
					ID:     "ev:" + ev.ID,
					Kind:   "evidence",
					NodeID: ev.NodeID,
					Label:  compactEvidenceLabel(ev),
					Folder: parent.Folder,
				})
				addEdge(eIdx, nIdx, "evidence")
			}
		}
	}

	// 6. Partition into Connected Components (Independent Islands!) and relax each island cleanly.
	layoutMultiIslandPulseGraph(&pg)
	return pg
}

type pulseIsland struct {
	members   []int
	noteCount int
	seedKey   string
}

func layoutMultiIslandPulseGraph(pg *PulseGraph) {
	n := len(pg.Nodes)
	if n == 0 {
		return
	}

	// Find connected components via BFS.
	visited := make([]bool, n)
	var islands []pulseIsland

	for i := 0; i < n; i++ {
		if visited[i] {
			continue
		}
		var comp []int
		noteCnt := 0
		queue := []int{i}
		visited[i] = true
		for len(queue) > 0 {
			curr := queue[0]
			queue = queue[1:]
			comp = append(comp, curr)
			if pg.Nodes[curr].Kind == "note" {
				noteCnt++
			}
			for nb := range pg.NeighborsByIdx[curr] {
				if !visited[nb] {
					visited[nb] = true
					queue = append(queue, nb)
				}
			}
		}
		// Sort members deterministically
		sort.Ints(comp)
		islands = append(islands, pulseIsland{
			members:   comp,
			noteCount: noteCnt,
			seedKey:   pg.Nodes[comp[0]].ID,
		})
	}

	// Largest connected islands take prominent central-ring positions; smaller standalone islands orbit cleanly around.
	sort.Slice(islands, func(i, j int) bool {
		if len(islands[i].members) != len(islands[j].members) {
			return len(islands[i].members) > len(islands[j].members)
		}
		return islands[i].seedKey < islands[j].seedKey
	})

	pg.IslandCount = len(islands)

	// Compute well-separated island centers in [-1..1] space so separate islands NEVER crowd together.
	islandCenters := computeIslandCenters(len(islands), islands)

	for islIdx, isl := range islands {
		cx, cy := islandCenters[islIdx][0], islandCenters[islIdx][1]
		// Mark hub note inside each island (note with highest degree, or sole note in a 1-note island so its title shows!)
		bestIdx := -1
		bestDeg := -1
		for _, idx := range isl.members {
			pg.Nodes[idx].IslandID = islIdx
			if pg.Nodes[idx].Kind == "note" && pg.Nodes[idx].Degree > bestDeg {
				bestDeg = pg.Nodes[idx].Degree
				bestIdx = idx
			}
		}
		if bestIdx >= 0 {
			pg.Nodes[bestIdx].IsHub = true
		}

		// Seed deterministic positions around this island's own center
		spread := 0.08 + 0.035*math.Sqrt(float64(len(isl.members)))
		if len(isl.members) == 1 {
			pg.Nodes[isl.members[0]].X = cx
			pg.Nodes[isl.members[0]].Y = cy
			continue
		}
		for _, idx := range isl.members {
			jx, jy := seededJitter(pg.Nodes[idx].ID, spread)
			pg.Nodes[idx].X = cx + jx
			pg.Nodes[idx].Y = cy + jy
		}
	}

	if n == 1 {
		pg.Nodes[0].X = 0.5
		pg.Nodes[0].Y = 0.48
		pg.Nodes[0].AnchorX = 0.5
		pg.Nodes[0].AnchorY = 0.48
		return
	}

	vx := make([]float64, n)
	vy := make([]float64, n)
	const iters = 130

	for step := 0; step < iters; step++ {
		temp := 0.038 * (1.0 - float64(step)/float64(iters)*0.86)

		// All-pairs Coulomb repulsion (stronger between different islands to preserve clear dark channels!)
		for i := 0; i < n; i++ {
			for j := i + 1; j < n; j++ {
				dx := pg.Nodes[i].X - pg.Nodes[j].X
				dy := pg.Nodes[i].Y - pg.Nodes[j].Y
				d2 := dx*dx + dy*dy + 0.004
				d := math.Sqrt(d2)
				repK := 0.0065
				if pg.Nodes[i].IslandID != pg.Nodes[j].IslandID {
					repK = 0.015
				} else if pg.Nodes[i].Kind == "note" && pg.Nodes[j].Kind == "note" {
					repK = 0.012
				}
				rep := repK / d2
				fx, fy := (dx/d)*rep, (dy/d)*rep
				vx[i] += fx
				vy[i] += fy
				vx[j] -= fx
				vy[j] -= fy
			}
		}

		// Intra-island Hooke spring attraction along real edges ONLY
		for _, e := range pg.Edges {
			dx := pg.Nodes[e.V].X - pg.Nodes[e.U].X
			dy := pg.Nodes[e.V].Y - pg.Nodes[e.U].Y
			d := math.Hypot(dx, dy) + 0.0001
			restLen := 0.20
			stiff := 0.18
			switch e.Kind {
			case "evidence":
				restLen = 0.12
				stiff = 0.24
			case "tag":
				restLen = 0.15
				stiff = 0.20
			}
			force := (d - restLen) * stiff
			fx, fy := (dx/d)*force, (dy/d)*force
			vx[e.U] += fx
			vy[e.U] += fy
			vx[e.V] -= fx
			vy[e.V] -= fy
		}

		// Per-island anchor gravity keeping separate islands well-spaced and balanced
		for i := 0; i < n; i++ {
			ic := islandCenters[pg.Nodes[i].IslandID]
			vx[i] += (ic[0] - pg.Nodes[i].X) * 0.075
			vy[i] += (ic[1] - pg.Nodes[i].Y) * 0.085
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
	spanX := max(0.35, maxX-minX)
	spanY := max(0.35, maxY-minY)
	midX := (minX + maxX) * 0.5
	midY := (minY + maxY) * 0.5

	for i := range pg.Nodes {
		pg.Nodes[i].X = 0.50 + ((pg.Nodes[i].X-midX)/spanX)*0.76
		pg.Nodes[i].Y = 0.48 + ((pg.Nodes[i].Y-midY)/spanY)*0.68
		ic := islandCenters[pg.Nodes[i].IslandID]
		pg.Nodes[i].AnchorX = 0.50 + ((ic[0]-midX)/spanX)*0.76
		pg.Nodes[i].AnchorY = 0.48 + ((ic[1]-midY)/spanY)*0.68
	}
}

func computeIslandCenters(k int, islands []pulseIsland) [][2]float64 {
	centers := make([][2]float64, k)
	if k <= 1 {
		return centers
	}
	if k == 2 {
		centers[0] = [2]float64{-0.42, -0.10}
		centers[1] = [2]float64{0.44, 0.14}
		return centers
	}
	if k == 3 {
		// Organic asymmetric triangle so 3 islands feel natural rather than rigid grid
		centers[0] = [2]float64{-0.34, -0.18}
		centers[1] = [2]float64{0.46, -0.22}
		centers[2] = [2]float64{0.08, 0.42}
		return centers
	}
	// Golden-angle spiral distribution for 4+ islands
	const golden = 2.399963229728653 // pi * (3 - sqrt(5))
	for i := 0; i < k; i++ {
		h := fnv.New32a()
		_, _ = h.Write([]byte(islands[i].seedKey))
		phaseJitter := (float64(h.Sum32()&0xFF)/255.0 - 0.5) * 0.18
		r := 0.28 + 0.34*math.Sqrt(float64(i)/float64(k))
		if i == 0 && len(islands[0].members) >= 3 {
			r = 0.18
		}
		theta := float64(i)*golden + phaseJitter - 0.55
		centers[i] = [2]float64{
			math.Cos(theta) * r * 1.18,
			math.Sin(theta) * r * 0.92,
		}
	}
	return centers
}

var broadUmbrellaTags = map[string]bool{
	"随口一记": true, "语音闪念": true, "图像灵感": true, "手绘草图": true,
	"生活美学": true, "学习笔记": true, "日常记录": true, "工作思考": true,
	"奇思妙想": true, "阅读摘录": true, "随手闪念": true, "多端协同": true,
}

// notesNaturallyRelated returns true ONLY when two IdeaNodes share a specific, concrete
// domain tag or concrete concept phrase — never linking notes merely for sharing a folder or broad category!
func notesNaturallyRelated(a, b domain.IdeaNode) bool {
	tagSet := make(map[string]bool, len(a.Tags))
	for _, t := range a.Tags {
		clean := strings.TrimSpace(strings.TrimLeft(strings.TrimSpace(t), "#＃"))
		if clean != "" && !broadUmbrellaTags[clean] {
			tagSet[clean] = true
		}
	}
	for _, t := range b.Tags {
		clean := strings.TrimSpace(strings.TrimLeft(strings.TrimSpace(t), "#＃"))
		if tagSet[clean] {
			return true
		}
	}

	// Concrete curated phrase overlap (e.g. both discuss "郁金香", "钴蓝玻璃", "转移概率", "蒙特卡洛", "手表盲操")
	hayA := a.Title + " " + a.ConversationalSummary
	hayB := b.Title + " " + b.ConversationalSummary
	for _, phrase := range curatedConceptPhrases {
		if strings.Contains(hayA, phrase) && strings.Contains(hayB, phrase) {
			return true
		}
	}

	// Specific >=3-rune title token overlap (e.g. "郁金香", "强化学习", "着色器", "花瓶")
	toksA := specificTitleKeywords(a.Title)
	toksB := specificTitleKeywords(b.Title)
	for tok := range toksA {
		if toksB[tok] {
			return true
		}
	}
	return false
}

var genericTitleWords = map[string]bool{
	"用一个": true, "做一颗": true, "做温暖": true, "到底差": true, "哪一步": true,
	"有模型": true, "免模型": true, "不完美": true, "一株": true, "一个": true,
}

func specificTitleKeywords(title string) map[string]bool {
	out := make(map[string]bool)
	for _, kw := range []string{
		"郁金香", "花瓶", "钴蓝", "花盆", "薄荷", "虹吸", "焦散",
		"着色器", "噪点", "法线", "渲染管线",
		"强化学习", "马尔可夫", "转移概率", "动态规划", "蒙特卡洛",
		"手表", "盲操", "语音球", "语音记忆球",
	} {
		if strings.Contains(title, kw) && !genericTitleWords[kw] {
			out[kw] = true
		}
	}
	return out
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
