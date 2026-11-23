package main

import (
	"log"
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/pengstem/neromancer/internal/ai"
	"github.com/pengstem/neromancer/internal/config"
	"github.com/pengstem/neromancer/internal/media"
	"github.com/pengstem/neromancer/internal/service"
	"github.com/pengstem/neromancer/internal/store"
	appui "github.com/pengstem/neromancer/internal/ui"
)

func main() {
	cfg := config.Load(".env")

	var st *store.Store
	var app *appui.App
	var mainWin *mygo.Window

	openQuickCapture := func() {
		if mainWin != nil && !mainWin.IsDestroyed() {
			mainWin.Show()
			mainWin.Focus()
			mainWin.Update(func() {
				if app != nil {
					app.CaptureModalOpen = true
				}
			})
		}
	}

	mygo.App.OnOpenURL(func(rawURL string) {
		if strings.Contains(strings.ToLower(rawURL), "capture") {
			openQuickCapture()
		}
	})

	mygo.App.OnQuit(func() {
		if st != nil {
			_ = st.Close()
		}
	})

	mygo.App.WhenReady(func() {
		if cfg.DataDir == "" {
			if userDir, err := mygo.App.Path(mygo.PathUserData); err == nil && userDir != "" {
				cfg.DataDir = userDir
			} else {
				cfg.DataDir = "./data"
			}
		}
		if err := cfg.EnsureDirs(); err != nil {
			log.Fatalf("初始化应用数据目录失败: %v", err)
		}

		var err error
		st, err = store.Open(cfg.DBPath())
		if err != nil {
			log.Fatalf("打开本地灵感库失败: %v", err)
		}
		if err := st.SeedIfEmpty(); err != nil {
			log.Fatalf("写入演示种子数据失败: %v", err)
		}

		mm := media.NewManager(cfg.AudioDir(), cfg.ImageDir())
		aiClient := ai.NewClient(cfg.DeepSeekBaseURL, cfg.DeepSeekAPIKey, cfg.DeepSeekModel)

		app, err = appui.NewApp(appui.Deps{
			Config:   cfg,
			Store:    st,
			AI:       aiClient,
			Media:    mm,
			Capture:  service.NewCaptureService(st, aiClient, mm),
			Organize: service.NewOrganizeService(st),
			Incubate: service.NewIncubateService(st, aiClient),
			Share:    service.NewShareService(st, cfg.ExportDir()),
		})
		if err != nil {
			log.Fatalf("初始化应用状态失败: %v", err)
		}

		// On phones (Android/iOS), MyGo automatically fills the screen inside safe areas.
		// On desktops (mygo dev), open a 412x892 Android phone portrait preview window.
		mainWin = mygo.NewWindow(mygo.WindowOptions{
			Title:           "Neromancer",
			Width:           412,
			Height:          892,
			MinWidth:        360,
			MinHeight:       620,
			BackgroundColor: "#F2F3F6",
			StateKey:        "neromancer_mobile",
			Content:         ui.View(app.RootView),
		})
		app.MainWin = mainWin
		app.CheckAIHealthAsync()
	})

	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}
