package ui

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/pengstem/neromancer/internal/ai"
	"github.com/pengstem/neromancer/internal/config"
	"github.com/pengstem/neromancer/internal/media"
	"github.com/pengstem/neromancer/internal/service"
	"github.com/pengstem/neromancer/internal/store"
)

func newOnboardingTestDeps(t testing.TB, st *store.Store, baseURL, apiKey string) Deps {
	t.Helper()
	tmp := t.TempDir()
	cfg := config.Config{
		DeepSeekBaseURL: baseURL,
		DeepSeekAPIKey:  apiKey,
		DeepSeekModel:   "deepseek-flash",
		DataDir:         tmp,
	}
	if err := cfg.EnsureDirs(); err != nil {
		t.Fatal(err)
	}
	mm := media.NewManager(cfg.AudioDir(), cfg.ImageDir())
	aiClient := ai.NewClient(cfg.DeepSeekBaseURL, cfg.DeepSeekAPIKey, cfg.DeepSeekModel)
	return Deps{
		Config:   cfg,
		Store:    st,
		AI:       aiClient,
		Media:    mm,
		Capture:  service.NewCaptureService(st, aiClient, mm),
		Organize: service.NewOrganizeService(st),
		Incubate: service.NewIncubateService(st, aiClient),
		Share:    service.NewShareService(st, cfg.ExportDir()),
	}
}

func TestFirstLaunchAutoPromptsDeepSeekSetupOnce(t *testing.T) {
	st, err := store.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = st.Close() }()
	if err := st.SeedIfEmpty(); err != nil {
		t.Fatal(err)
	}

	app, err := NewApp(newOnboardingTestDeps(t, st, "https://api.deepseek.com", ""))
	if err != nil {
		t.Fatal(err)
	}
	if !app.SettingsModalOpen || app.SettingsPromptReason != settingsPromptFirstLaunch {
		t.Fatalf("expected first-launch unconfigured app to auto-open settings modal, got open=%v reason=%q",
			app.SettingsModalOpen, app.SettingsPromptReason)
	}

	tt := newFastTester(app.RootView, 412, 892)
	settleAudit(tt)
	if !tt.HasText("DeepSeek 设置") || !tt.HasText("填一次 API Key，AI 就能自动帮你整理灵感，之后就不用再管了") {
		t.Fatalf("expected first-launch invitation modal copy, texts=%v", tt.Texts())
	}

	if err := tt.Click("先跳过"); err != nil {
		t.Fatalf("click skip onboarding: %v", err)
	}
	settleAudit(tt)
	if app.SettingsModalOpen {
		t.Fatalf("expected SettingsModalOpen=false after clicking 先跳过")
	}
	seen, err := st.GetSetting(settingDeepSeekOnboardingSeen)
	if err != nil || seen != "1" {
		t.Fatalf("expected %s=\"1\" persisted after skipping, got %q err=%v", settingDeepSeekOnboardingSeen, seen, err)
	}

	// Re-launching on the same store must NOT re-prompt automatically while idle.
	app2, err := NewApp(newOnboardingTestDeps(t, st, "https://api.deepseek.com", ""))
	if err != nil {
		t.Fatal(err)
	}
	if app2.SettingsModalOpen {
		t.Fatalf("expected returning launch not to re-open onboarding settings modal")
	}
}

func TestConfiguredAppDoesNotAutoPromptOnFirstLaunch(t *testing.T) {
	st, err := store.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = st.Close() }()
	if err := st.SeedIfEmpty(); err != nil {
		t.Fatal(err)
	}

	app, err := NewApp(newOnboardingTestDeps(t, st, "https://api.deepseek.com", "sk-live-ok"))
	if err != nil {
		t.Fatal(err)
	}
	if app.SettingsModalOpen {
		t.Fatalf("expected already-configured app not to auto-open settings modal on launch")
	}
}

func TestUnusableCredentialsAutoRePromptsSettings(t *testing.T) {
	// 1. Missing API key detected during async AI action on returning user -> auto-opens settings modal.
	st1, err := store.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = st1.Close() }()
	if err := st1.SeedIfEmpty(); err != nil {
		t.Fatal(err)
	}
	if err := st1.SetSetting(settingDeepSeekOnboardingSeen, "1"); err != nil {
		t.Fatal(err)
	}

	app1, err := NewApp(newOnboardingTestDeps(t, st1, "https://api.deepseek.com", ""))
	if err != nil {
		t.Fatal(err)
	}
	app1.RunAsync("AI 整理", func(ctx context.Context) error {
		return ai.ErrMissingAPIKey
	})

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		app1.FlushPending()
		if app1.SettingsModalOpen {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if !app1.SettingsModalOpen || app1.SettingsPromptReason != settingsPromptUnavailable {
		t.Fatalf("expected ErrMissingAPIKey to auto-open unavailable settings prompt, got open=%v reason=%q",
			app1.SettingsModalOpen, app1.SettingsPromptReason)
	}

	// 2. Startup health check detecting HTTP 401 -> auto-opens settings modal with failure status.
	srv401 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":{"message":"invalid api key"}}`))
	}))
	defer srv401.Close()

	st2, err := store.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = st2.Close() }()
	if err := st2.SeedIfEmpty(); err != nil {
		t.Fatal(err)
	}

	app2, err := NewApp(newOnboardingTestDeps(t, st2, srv401.URL, "sk-expired"))
	if err != nil {
		t.Fatal(err)
	}
	app2.CheckAIHealthAsync()

	deadline = time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		app2.FlushPending()
		if app2.SettingsModalOpen {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if !app2.SettingsModalOpen || app2.SettingsPromptReason != settingsPromptUnavailable {
		t.Fatalf("expected HTTP 401 health check to auto-open settings modal, got open=%v reason=%q",
			app2.SettingsModalOpen, app2.SettingsPromptReason)
	}
	if !strings.Contains(app2.SettingsTestStatus, "401") {
		t.Fatalf("expected SettingsTestStatus to mention 401, got %q", app2.SettingsTestStatus)
	}

	// Render and click Save after updating credentials -> closes modal and persists updated key.
	app2.SettingsAPIKey = "sk-fresh-key"
	tt := newFastTester(app2.RootView, 412, 892)
	settleAudit(tt)
	if !tt.HasText("检测到当前配置暂时不能用了，重新填一下就能恢复") {
		t.Fatalf("expected unavailable prompt subtitle in settings modal, texts=%v", tt.Texts())
	}
	if err := tt.Click("保存"); err != nil {
		t.Fatalf("click 保存: %v", err)
	}
	settleAudit(tt)
	if app2.SettingsModalOpen {
		t.Fatalf("expected SettingsModalOpen=false after saving")
	}
	savedKey, _ := st2.GetSetting("deepseek_api_key")
	if savedKey != "sk-fresh-key" {
		t.Fatalf("expected updated API key persisted, got %q", savedKey)
	}

	// 3. Transient HTTP 500 at startup must NOT hijack the screen with settings modal.
	srv500 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":{"message":"server busy"}}`))
	}))
	defer srv500.Close()

	st3, err := store.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = st3.Close() }()
	if err := st3.SeedIfEmpty(); err != nil {
		t.Fatal(err)
	}

	app3, err := NewApp(newOnboardingTestDeps(t, st3, srv500.URL, "sk-ok-but-500"))
	if err != nil {
		t.Fatal(err)
	}
	app3.CheckAIHealthAsync()
	time.Sleep(80 * time.Millisecond)
	app3.FlushPending()
	if app3.SettingsModalOpen {
		t.Fatalf("expected transient HTTP 500 NOT to hijack screen with settings modal")
	}
}
