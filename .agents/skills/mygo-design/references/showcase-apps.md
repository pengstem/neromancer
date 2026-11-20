# Benchmark MyGo Works & Reusable Design Patterns

Curated from `egoist/mygo-showcase`, `echoVic/awesome-mygo-apps`, `mygodock.dev`, and `egoist/mygo/examples`. Use this catalog to match your feature to a proven open-source design precedent.

---

## 1. Quick Decision Matrix: Which Work Should You Study?

| Target Experience / Feature | Primary Reference Work(s) | Core MyGo Packages Used | Key Design Pattern to Borrow |
| :--- | :--- | :--- | :--- |
| **Infinite canvas, node graph, spatial board, whiteboard** | `egoist/godraw` | `ui`, `ui/shader` (`DrawInstances`), `mygo.NewDocuments` | Immutable record store + trivial snapshot Undo/Redo + per-shape display-list caching + floating tool palette |
| **Agentic IDE, terminal, multi-session workspace** | `egoist/kero`, `egoist/gorex` | `ui`, `plugins/terminal`, `ui.Split` | Background daemon (`kerod`) decoupled from UI window; spatial cards for tabs/splits instead of cramped tab bars |
| **Menu-bar utility, system monitor, quick capture companion** | `egoist/pulse`, `examples/menubar-flyout`, `examples/launcher` | `mygo.NewTray`, `mygo.NewFlyout`, `GlobalShortcut`, `ui/chart` | Frameless anchored flyout / floating panel + live tray readings + event-driven updates (0% idle CPU) |
| **Cross-platform Desktop + Mobile AI / chat app** | `egoist/lorca`, `examples/mobile`, `examples/markdown-chat` | `ui`, `ui/mobile`, `ui/markdown` | Shared Go view state with adaptive navigation (`BottomBar` + `Sheet` on phone; `Sidebar` + `Split` on desktop) |
| **Design system & reusable 70+ widget kit** | `lobehub/lobe-mygo` | `ui` (`*Base` primitives), `ui.Oklch` | `(c, content..., Props)` API convention, generated semantic tokens, Lucide SVG icons, spring-driven active indicators |
| **Strong brand / editorial / non-generic aesthetic** | `ELY-Open-Source/MujicaUI`, `echoVic/liubai` | `ui`, `ui.RegisterFont`, `ui/richtext` | Custom embedded typography, high-taste palettes (ivory/charcoal, hairline accents), configurable compact/comfortable density |
| **Extensible media player / JS plugin host** | `egoist/mygo-music`, `fzn0x/mixmusic` | `mygo` (WebView + `Bind` + `NewEvent`), `mygo/audio`, `NowPlaying` | Manifest-driven plugin sandbox (`plugin.json`), explicit permission prompts, native OS media transport integration |
| **Tactile shaders, Liquid Glass & micro-interactions** | `examples/liquid-glass`, `examples/chat-glow`, `examples/effort-slider` | `plugins/glass`, `ui/shader` | Backdrop refraction (`glass.Glass{}`), Kage SDF glow/caustics reactive to drag velocity or audio energy |

---

## 2. Deep-Dive Case Studies

### A. `egoist/godraw` — Infinite Canvas & Spatial Tooling
- **Repository**: [github.com/egoist/godraw](https://github.com/egoist/godraw) (Pure-Go `tldraw`-compatible `.tldr` whiteboard)
- **Why It Excels**:
  1. **Immutable State Records**: Every committed shape/record is immutable. Editing a shape replaces its record in the map rather than mutating fields in place.
     - *Benefit 1*: Undo/Redo history simply stores prior immutable record references with zero deep-clone bugs.
     - *Benefit 2*: Background auto-save snapshots copy top-level map references cleanly without blocking the 60/120fps UI thread.
  2. **Display-List & Instance Batching**: Caches per-shape geometry/display lists and batches repeated primitives (`shader.DrawInstances`), keeping pan/zoom buttery smooth with thousands of items.
  3. **Distraction-Free Spatial Chrome**: Canvas fills 100% of the viewport; toolbars, style inspectors, and the minimap float over the canvas with subtle elevation and collapse cleanly in **Focus Mode**.
  4. **Agent/E2E Driveability**: Compiles an opt-in automation socket (`-tags e2e` / `mygo/plugins/automation`) so tests and agents can inspect and drive canvas interactions deterministically.

### B. `egoist/kero` & `egoist/gorex` — High-Density Workspace & Card Splits
- **Repositories**: [kero.sh](https://kero.sh) (`egoist/kero`) & [github.com/egoist/gorex](https://github.com/egoist/gorex)
- **Why They Excel**:
  1. **Process Lifecycle Separation (`kero`)**: Long-running coding agent sessions live in a lightweight background daemon (`kerod`), while the `mygo/ui` frontend is a pure stateless/reattachable presenter. Closing the window never loses live streams.
  2. **Card-Framed Panes (`gorex`)**: Instead of 1px dividers cramming terminal panes edge-to-edge, panes sit inside rounded surface cards with subtle outer padding, clear active-focus borders, and live process status headers.
  3. **Project-Grouped Status Sidebar (`kero`)**: Groups items by context/project and surfaces state at a glance (idle, running, awaiting human input) using restrained semantic color dots and crisp vector icons rather than noisy badges.

### C. `egoist/pulse` — Zero-Overhead Tray & Flyout Utility
- **Website / Issues**: [pulse.egoist.dev](https://pulse.egoist.dev) / [github.com/egoist/pulse-feedback](https://github.com/egoist/pulse-feedback)
- **Why It Excels**:
  1. **Dual-Surface Presence**: Uses `mygo.NewTray` for glanceable menu-bar metrics and `mygo.NewFlyout` (`mygo.FlyoutOptions`) for an anchored, frameless popover dashboard that dismisses cleanly on outside click.
  2. **High-Density Sparklines & Meters**: Pairs tabular monospace numbers (`FontFeatures` / fixed-width digits so values never jitter horizontally when updating!) with compact `Painter.StrokePolyline` / `ui/chart` sparklines.
  3. **Strict Power Discipline**: Pauses visual ticks when the flyout window is hidden (`Window.OnHide` / `Window.IsVisible()`), ensuring background monitoring costs almost zero CPU/GPU.

### D. `lobehub/lobe-mygo` & `ELY-Open-Source/MujicaUI` — Component Systems
- **Repositories**: [github.com/lobehub/lobe-mygo](https://github.com/lobehub/lobe-mygo) & [github.com/ELY-Open-Source/MujicaUI](https://github.com/ELY-Open-Source/MujicaUI)
- **Why They Excel**:
  1. **`lobe-mygo` Ergonomics**: Demonstrates how to scale a `mygo/ui` codebase to 70+ cohesive components (ActionIcon, DraggablePanel, Segmented, Modal, Toast, SortableList, QRCode, Highlighter) using generated design tokens (`theme.TokensOf(c)`), 2,300+ vector Lucide icons, and spring-animated selection indicators.
  2. **`MujicaUI` Expressive Theming**: Proves `mygo/ui` can deliver distinctive, non-cookie-cutter visual identities—pairing warm ivory / deep charcoal surfaces, burgundy & old-gold 1px rules, embedded display serifs, and switchable layout density (`Compact` vs `Comfortable`).

### E. `echoVic/liubai` (留白) & `examples/notes` — Calm Local-First Writing
- **Repositories**: [github.com/echoVic/liubai](https://github.com/echoVic/liubai) & `github.com/egoist/mygo/examples/notes`
- **Why They Excel**:
  1. **Typography-First Hierarchy**: Generous line-height, constrained comfortable reading measure (640–720 DIP max width centered in wide windows), and zero chrome clutter until needed.
  2. **Local-First Persistence**: Instant local writes to user data directories (`mygo.App.Path(mygo.PathUserData)`) with atomic file replacement and clean Markdown/PNG/PDF export pipelines.

### F. `DIYgod/pinky` & `looplj/axonhub` — Agentic & Human-in-the-Loop UX
- **Repositories**: [github.com/DIYgod/pinky](https://github.com/DIYgod/pinky) & [github.com/looplj/axonhub](https://github.com/looplj/axonhub)
- **Why They Excel**:
  1. **Clear Trust & Approval Boundaries**: When AI/background services act on user data or devices, surface pending actions in non-destructive inline review cards with one-click approve/undo rather than blocking modal alert storms.

---

## 3. Official `egoist/mygo/examples/*` Cheat Sheet

When implementing a specific mechanism, check the matching official example pattern in `github.com/egoist/mygo/examples/`:

- **Materials & Window Chrome**: `liquid-glass`, `vibrancy`, `frameless`, `flyout`, `menubar-flyout`, `menubar-native`, `launcher`
- **Shaders & Motion**: `chat-glow`, `effort-slider`, `blackhole`, `confetti`, `wind`, `timeline`, `loading`
- **Rich Content & Editors**: `notes` (`ui/richtext`), `markdown-chat` (`ui/markdown`), `code-editor` (`ui/editor`), `chart-streaming` (`ui/chart`), `video` (`ui/video`)
- **System & Cross-Platform**: `mobile` (`ui/mobile`), `documents` (`mygo.NewDocuments`), `drag-drop`, `clipboard`, `permissionflow`, `sf-symbols`
