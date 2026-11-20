package main

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"log"
	"math"
	"runtime"
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
	var floatWin *mygo.Window
	var tray *mygo.Tray

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

		isMobileOrWeb := runtime.GOOS == "android" || runtime.GOOS == "ios" || runtime.GOOS == "js"
		if isMobileOrWeb {
			return
		}

		// Desktop preview companion: frameless AlwaysOnTop floating orb, system tray & global hotkey
		toggleFloat := func() {
			if floatWin == nil || floatWin.IsDestroyed() {
				return
			}
			if floatWin.IsVisible() {
				floatWin.Hide()
			} else {
				floatWin.Show()
				floatWin.Focus()
			}
		}
		app.FloatOnToggle = toggleFloat

		floatWin = mygo.NewWindow(mygo.WindowOptions{
			Title:           "Neromancer 悬浮球",
			Width:           96,
			Height:          96,
			X:               48,
			Y:               120,
			Frameless:       true,
			Transparent:     true,
			AlwaysOnTop:     true,
			DisableResize:   true,
			SkipTaskbar:     true,
			Panel:           true,
			Hidden:          true,
			BackgroundColor: "#00000000",
			Content:         ui.View(app.FloatingOrbView),
		})
		app.FloatWin = floatWin

		mainWin.OnClosed(func() {
			if floatWin != nil && !floatWin.IsDestroyed() {
				floatWin.Destroy()
			}
			mygo.App.Quit()
		})

		menu := mygo.NewMenu([]*mygo.MenuItem{
			{
				Label: "打开 Neromancer",
				Click: func(_ *mygo.MenuItem, _ *mygo.Window) {
					if mainWin != nil && !mainWin.IsDestroyed() {
						mainWin.Show()
						mainWin.Focus()
					}
				},
			},
			{
				Label:       "快速记录一条灵感",
				Accelerator: "CmdOrCtrl+Shift+I",
				Click: func(_ *mygo.MenuItem, _ *mygo.Window) {
					openQuickCapture()
				},
			},
			{
				Label: "显示 / 隐藏桌面悬浮球",
				Click: func(_ *mygo.MenuItem, _ *mygo.Window) {
					toggleFloat()
				},
			},
			mygo.Separator(),
			{
				Label: "退出 Neromancer",
				Click: func(_ *mygo.MenuItem, _ *mygo.Window) {
					mygo.App.Quit()
				},
			},
		})

		var trayErr error
		tray, trayErr = mygo.NewTray(mygo.TrayOptions{
			ID:      "neromancer-tray",
			Icon:    buildTrayIconPNG(),
			Title:   "Neromancer",
			ToolTip: "Neromancer",
			Menu:    menu,
		})
		if trayErr != nil {
			log.Printf("系统托盘初始化提示: %v", trayErr)
		}
		_ = tray

		if err := mygo.GlobalShortcut.Register("CmdOrCtrl+Shift+I", openQuickCapture); err != nil {
			log.Printf("全局快捷键 CmdOrCtrl+Shift+I 注册提示: %v", err)
		}
	})

	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}

func buildTrayIconPNG() []byte {
	const size = 32
	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	cx, cy := float64(size)/2, float64(size)/2
	r := 12.5
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			d := math.Hypot(float64(x)-cx, float64(y)-cy)
			if d > r {
				continue
			}
			t := d / r
			alpha := uint8(255)
			if d > r-1.2 {
				alpha = uint8(math.Max(0, (r-d)/1.2) * 255)
			}
			col := color.NRGBA{
				R: uint8(255 - t*30),
				G: uint8(190 - t*85),
				B: uint8(195 - t*70),
				A: alpha,
			}
			img.SetNRGBA(x, y, col)
		}
	}
	var buf bytes.Buffer
	_ = png.Encode(&buf, img)
	return buf.Bytes()
}
