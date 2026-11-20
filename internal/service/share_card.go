package service

import (
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/pengstem/neromancer/internal/domain"
	"github.com/pengstem/neromancer/internal/store"
)

// CardRasterizer allows the UI layer to inject a ui.Render-based high-DPI rasterizer
// (with native CJK system typography) without creating an import cycle from service -> ui.
type CardRasterizer func(node domain.IdeaNode, evCount int) *image.RGBA

// ShareService handles exporting cool Memory Orb PNG visual cards and portable .orb.json capsules.
type ShareService struct {
	store      *store.Store
	exportDir  string
	rasterizer CardRasterizer
}

// NewShareService creates a ShareService writing exports into exportDir.
func NewShareService(st *store.Store, exportDir string) *ShareService {
	return &ShareService{store: st, exportDir: exportDir}
}

// SetRasterizer injects an optional high-DPI native UI card rasterizer.
func (s *ShareService) SetRasterizer(fn CardRasterizer) {
	s.rasterizer = fn
}

// ExportPNGCard renders and saves a Memory Orb share card as a PNG image.
func (s *ShareService) ExportPNGCard(nodeID string) (string, error) {
	node, err := s.store.GetNode(nodeID)
	if err != nil {
		return "", err
	}
	evs, err := s.store.ListEvidencesByNode(nodeID)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(s.exportDir, 0o755); err != nil {
		return "", err
	}

	var img *image.RGBA
	if s.rasterizer != nil {
		img = s.rasterizer(node, len(evs))
	}
	if img == nil {
		img = renderProceduralOrbCard(node, len(evs))
	}

	filename := fmt.Sprintf("%s_%d.png", sanitizeFilename(node.Title), time.Now().Unix())
	outPath := filepath.Join(s.exportDir, filename)
	f, err := os.Create(outPath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	if err := png.Encode(f, img); err != nil {
		return "", err
	}
	return outPath, nil
}

// ExportCapsule writes a portable .orb.json capsule containing the node, immutable raw evidences, and snapshots.
func (s *ShareService) ExportCapsule(nodeID string) (string, error) {
	capsule, err := s.store.ExportOrbCapsule(nodeID)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(s.exportDir, 0o755); err != nil {
		return "", err
	}
	filename := fmt.Sprintf("%s_%d.orb.json", sanitizeFilename(capsule.Node.Title), time.Now().Unix())
	outPath := filepath.Join(s.exportDir, filename)
	raw, err := json.MarshalIndent(capsule, "", "  ")
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(outPath, raw, 0o644); err != nil {
		return "", err
	}
	return outPath, nil
}

// ImportCapsuleFile imports a shared .orb.json capsule into the local SQLite store.
func (s *ShareService) ImportCapsuleFile(path string) (domain.IdeaNode, error) {
	raw, err := os.ReadFile(strings.TrimSpace(path))
	if err != nil {
		return domain.IdeaNode{}, fmt.Errorf("读取备份文件失败: %w", err)
	}
	var capsule domain.OrbCapsule
	if err := json.Unmarshal(raw, &capsule); err != nil {
		return domain.IdeaNode{}, fmt.Errorf("备份文件格式不正确: %w", err)
	}
	return s.store.ImportOrbCapsule(capsule)
}

// renderProceduralOrbCard draws a soft luminous Memory Orb poster purely with Go's standard library.
func renderProceduralOrbCard(node domain.IdeaNode, evCount int) *image.RGBA {
	const w, h = 840, 1080
	img := image.NewRGBA(image.Rect(0, 0, w, h))

	c1, c2 := themeColors(node.Theme)
	bgTop := color.RGBA{252, 249, 244, 255}
	bgBot := mixRGBA(bgTop, c2, 0.14)

	for y := 0; y < h; y++ {
		ty := float64(y) / float64(h)
		rowC := mixRGBA(bgTop, bgBot, ty)
		for x := 0; x < w; x++ {
			img.SetRGBA(x, y, rowC)
		}
	}

	// Draw soft outer aura and glowing Inside-Out Memory Orb at upper center.
	cx, cy := float64(w)/2, 420.0
	orbR := 210.0 + float64(evCount)*6
	if orbR > 250 {
		orbR = 250
	}
	for y := int(cy - orbR*1.45); y <= int(cy+orbR*1.45); y++ {
		if y < 0 || y >= h {
			continue
		}
		for x := int(cx - orbR*1.45); x <= int(cx+orbR*1.45); x++ {
			if x < 0 || x >= w {
				continue
			}
			dx := float64(x) - cx
			dy := float64(y) - cy
			dist := math.Hypot(dx, dy)
			if dist > orbR*1.4 {
				continue
			}
			base := img.RGBAAt(x, y)
			if dist <= orbR {
				// Specular offset highlight towards top-left
				hx := float64(x) - (cx - orbR*0.32)
				hy := float64(y) - (cy - orbR*0.32)
				hd := math.Hypot(hx, hy) / (orbR * 1.45)
				if hd > 1 {
					hd = 1
				}
				orbCol := mixRGBA(c1, c2, hd)
				// Glass rim brightening near edge
				rim := dist / orbR
				if rim > 0.85 {
					orbCol = mixRGBA(orbCol, color.RGBA{255, 255, 255, 255}, (rim-0.85)*2.2)
				}
				// Soft anti-aliased edge
				alpha := 1.0
				if dist > orbR-2 {
					alpha = (orbR - dist) / 2.0
					if alpha < 0 {
						alpha = 0
					}
				}
				img.SetRGBA(x, y, mixRGBA(base, orbCol, alpha))
			} else {
				// Outer ambient glow
				falloff := 1.0 - (dist-orbR)/(orbR*0.4)
				if falloff > 0 {
					img.SetRGBA(x, y, mixRGBA(base, c2, falloff*0.24))
				}
			}
		}
	}
	return img
}

func themeColors(t domain.OrbTheme) (inner, outer color.RGBA) {
	switch t {
	case domain.ThemeSky:
		return color.RGBA{186, 230, 253, 255}, color.RGBA{56, 189, 248, 255}
	case domain.ThemeAmber:
		return color.RGBA{254, 240, 138, 255}, color.RGBA{245, 158, 11, 255}
	case domain.ThemeMint:
		return color.RGBA{167, 243, 208, 255}, color.RGBA{20, 184, 166, 255}
	case domain.ThemeLavender:
		return color.RGBA{221, 214, 254, 255}, color.RGBA{139, 92, 246, 255}
	default: // Coral
		return color.RGBA{254, 205, 211, 255}, color.RGBA{244, 114, 132, 255}
	}
}

func mixRGBA(a, b color.RGBA, t float64) color.RGBA {
	if t < 0 {
		t = 0
	}
	if t > 1 {
		t = 1
	}
	return color.RGBA{
		R: uint8(float64(a.R)*(1-t) + float64(b.R)*t),
		G: uint8(float64(a.G)*(1-t) + float64(b.G)*t),
		B: uint8(float64(a.B)*(1-t) + float64(b.B)*t),
		A: 255,
	}
}

func sanitizeFilename(name string) string {
	var b strings.Builder
	for _, r := range strings.TrimSpace(name) {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r >= 0x4E00 && r <= 0x9FFF:
			b.WriteRune(r)
		case r == '-' || r == '_':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}
	s := strings.Trim(b.String(), "_")
	if s == "" {
		return "memory_orb"
	}
	return s
}
