package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/pengstem/neromancer/internal/ai"
	"github.com/pengstem/neromancer/internal/domain"
	"github.com/pengstem/neromancer/internal/store"
)

// IncubateService powers DeepSeek guiding questions, actionable execution plans,
// inspiration collisions, and overnight sleep incubation.
type IncubateService struct {
	store *store.Store
	ai    *ai.Client
}

// NewIncubateService creates an IncubateService.
func NewIncubateService(st *store.Store, aiClient *ai.Client) *IncubateService {
	return &IncubateService{store: st, ai: aiClient}
}

// RefreshGuidingQuestions asks DeepSeek for 3 non-generic exploratory questions.
func (s *IncubateService) RefreshGuidingQuestions(ctx context.Context, nodeID string) (domain.IdeaNode, error) {
	node, err := s.store.GetNode(nodeID)
	if err != nil {
		return domain.IdeaNode{}, err
	}
	evs, err := s.store.ListEvidencesByNode(nodeID)
	if err != nil {
		return domain.IdeaNode{}, err
	}

	questions, err := s.ai.GenerateGuidingQuestions(ctx, node, evs)
	if err != nil {
		return domain.IdeaNode{}, err
	}
	if len(questions) > 0 {
		node.GuidingQuestions = questions
		if node.Stage == domain.StageSpark {
			node.Stage = domain.StageSprouting
		}
		node.UpdatedAt = time.Now()
		if err := s.store.UpsertNode(node); err != nil {
			return domain.IdeaNode{}, err
		}
	}
	return node, nil
}

// GenerateActionPlan asks DeepSeek ("AI 帮忙") for a concrete, step-by-step execution plan.
func (s *IncubateService) GenerateActionPlan(ctx context.Context, nodeID string) (domain.IdeaNode, error) {
	node, err := s.store.GetNode(nodeID)
	if err != nil {
		return domain.IdeaNode{}, err
	}
	evs, err := s.store.ListEvidencesByNode(nodeID)
	if err != nil {
		return domain.IdeaNode{}, err
	}

	planMD, err := s.ai.GenerateActionPlan(ctx, node, evs)
	if err != nil {
		return domain.IdeaNode{}, err
	}
	node.ActionPlanMD = planMD
	if node.Stage == domain.StageSpark || node.Stage == domain.StageSprouting {
		node.Stage = domain.StageIncubating
	}
	node.Weight = min(1.0, node.Weight+0.06)
	node.UpdatedAt = time.Now()
	if err := s.store.UpsertNode(node); err != nil {
		return domain.IdeaNode{}, err
	}
	_ = s.store.AddStageSnapshot(domain.StageSnapshot{
		ID:        fmt.Sprintf("snap_%d", time.Now().UnixNano()),
		NodeID:    node.ID,
		Stage:     node.Stage,
		Title:     node.Title,
		Summary:   node.ConversationalSummary,
		Note:      "DeepSeek「AI 帮忙」生成可执行方案",
		CreatedAt: time.Now(),
	})
	return node, nil
}

// SuggestShowcaseSpark generates a fresh spark based on the user's existing tags.
func (s *IncubateService) SuggestShowcaseSpark(ctx context.Context) (string, error) {
	nodes, err := s.store.ListNodes()
	if err != nil {
		return "", err
	}
	var tags, titles []string
	seen := make(map[string]struct{})
	for i, n := range nodes {
		if i < 6 {
			titles = append(titles, n.Title)
		}
		for _, t := range n.Tags {
			if _, ok := seen[t]; !ok && len(tags) < 10 {
				seen[t] = struct{}{}
				tags = append(tags, t)
			}
		}
	}
	return s.ai.SuggestFromTags(ctx, tags, titles)
}

// CollideWithRoam combines a user's IdeaNode with a read-only RoamSlice using DeepSeek,
// saving the newly spawned hybrid idea with an immutable provenance RawEvidence.
func (s *IncubateService) CollideWithRoam(ctx context.Context, myNodeID string, roam domain.RoamSlice) (domain.IdeaNode, *ai.CollisionResult, error) {
	mine, err := s.store.GetNode(myNodeID)
	if err != nil {
		return domain.IdeaNode{}, nil, err
	}
	res, err := s.ai.CollideInspirations(ctx, mine, roam)
	if err != nil {
		return domain.IdeaNode{}, nil, err
	}

	now := time.Now()
	hybrid := domain.IdeaNode{
		ID:                    fmt.Sprintf("orb_col_%d", now.UnixNano()),
		Title:                 strings.TrimSpace(res.Title),
		ConversationalSummary: strings.TrimSpace(res.ConversationalSummary),
		Tags:                  mergeUniqueStrings(res.Tags, []string{"灵感碰撞"}),
		Theme:                 domain.NormalizeTheme(res.Theme),
		Visual: domain.VisualMotif{
			MotifType: "constellation",
			Caption:   "跨界碰撞出的新星",
			Energy:    0.88,
		},
		Weight:       0.80,
		Stage:        domain.StageSprouting,
		ActionPlanMD: fmt.Sprintf("### 15 分钟最小试一试\n\n%s", strings.TrimSpace(res.FirstStep)),
		RelatedIDs:   []string{mine.ID},
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if hybrid.Title == "" {
		hybrid.Title = mine.Title + " × " + roam.Title
	}
	if err := s.store.UpsertNode(hybrid); err != nil {
		return domain.IdeaNode{}, nil, err
	}
	_ = s.store.AddRawEvidence(domain.RawEvidence{
		ID:       fmt.Sprintf("ev_col_%d", now.UnixNano()),
		NodeID:   hybrid.ID,
		Modality: "text",
		RawText:  fmt.Sprintf("灵感碰撞来源：我的《%s》 × 「%s」的切片《%s》", mine.Title, roam.AuthorAlias, roam.Title),
		Scene: domain.SceneContext{
			TimeOfDay:   timeOfDayLabel(now),
			Location:    "Neromancer 漫游碰撞室",
			MoodWeather: "跨界连线",
		},
		CapturedAt: now,
	})
	_ = s.store.AddStageSnapshot(domain.StageSnapshot{
		ID:        fmt.Sprintf("snap_col_%d", now.UnixNano()),
		NodeID:    hybrid.ID,
		Stage:     hybrid.Stage,
		Title:     hybrid.Title,
		Summary:   hybrid.ConversationalSummary,
		Note:      "与漫游切片碰撞诞生",
		CreatedAt: now,
	})
	return hybrid, res, nil
}

// RunSleepIncubation asks DeepSeek to discover non-obvious bridges across existing orbs.
func (s *IncubateService) RunSleepIncubation(ctx context.Context) ([]domain.SleepInsight, error) {
	nodes, err := s.store.ListNodes()
	if err != nil {
		return nil, err
	}
	insights, err := s.ai.SleepIncubate(ctx, nodes)
	if err != nil {
		return nil, err
	}
	for _, ins := range insights {
		if err := s.store.SaveSleepInsight(ins); err != nil {
			return nil, err
		}
	}
	return s.store.ListActiveSleepInsights()
}
