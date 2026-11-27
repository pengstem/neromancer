package main

import (
	"log"
	"runtime"
	"sync"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/pengstem/neromancer/internal/ai"
	"github.com/pengstem/neromancer/internal/config"
	_ "github.com/pengstem/neromancer/internal/homewidget"
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
	var pendingMu sync.Mutex
	var pendingModality *int

	openQuickCapture := func(modality int) {
		pendingMu.Lock()
		defer pendingMu.Unlock()
		if app == nil {
			m := modality
			pendingModality = &m
			return
		}
		app.ShowDesktopWidget()
		app.OpenCaptureFromWidget(modality)
	}

	mygo.App.OnOpenURL(func(rawURL string) {
		if modality, ok := appui.CaptureLaunchFromURL(rawURL); ok {
			openQuickCapture(modality)
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

		created, err := appui.NewApp(appui.Deps{
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
		pendingMu.Lock()
		app = created
		pendingMu.Unlock()

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
		pendingMu.Lock()
		queued := pendingModality
		pendingModality = nil
		pendingMu.Unlock()
		if queued != nil {
			app.OpenCaptureFromWidget(*queued)
		}

		if runtime.GOOS != "android" && runtime.GOOS != "ios" && runtime.GOOS != "js" {
			attachDesktopWidget(app, mainWin)
		}
	})

	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}

func attachDesktopWidget(app *appui.App, mainWin *mygo.Window) {
	widgetW, widgetH := appui.DesktopWidgetBounds()
	x, y := 48, 120
	work := mygo.Screen.PrimaryDisplay().WorkArea
	if work.Width > widgetW && work.Height > widgetH {
		x = work.X + work.Width - widgetW - 20
		y = work.Y + work.Height - widgetH - 20
	}
	widget := mygo.NewWindow(mygo.WindowOptions{
		Title:           "记一句",
		Width:           widgetW,
		Height:          widgetH,
		X:               x,
		Y:               y,
		Frameless:       true,
		Transparent:     true,
		AlwaysOnTop:     true,
		DisableResize:   true,
		SkipTaskbar:     true,
		Panel:           true,
		BackgroundColor: "#00000000",
		StateKey:        "neromancer_widget",
		Content:         ui.View(app.DesktopWidgetView),
	})
	app.FloatWin = widget

	mainWin.OnClosed(func() {
		if !widget.IsDestroyed() {
			widget.Destroy()
		}
		mygo.App.Quit()
	})

	if err := mygo.GlobalShortcut.RegisterWith("CmdOrCtrl+Shift+I", mygo.GlobalShortcutOptions{
		Description: "记下一条灵感",
	}, func() {
		app.ShowDesktopWidget()
		app.OpenCaptureFromWidget(1)
	}); err != nil {
		log.Printf("全局快捷键暂时没挂上: %v", err)
	}
}
