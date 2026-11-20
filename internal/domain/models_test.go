package domain

import "testing"

func TestInferIdeaIconKeyCategories(t *testing.T) {
	cases := []struct {
		name     string
		title    string
		summary  string
		tags     []string
		modality string
		want     string
	}{
		{"botanical", "用一个蓝色花瓶养一株郁金香", "阳台花店", []string{"生活美学"}, "text", IconKeyBotanical},
		{"prism", "用不完美噪点做温暖的手绘感着色器", "光影颗粒", []string{"图形学"}, "sketch", IconKeyPrism},
		{"wave", "手表端做一颗单键盲操语音记忆球", "抬腕按住说十秒", []string{"穿戴交互"}, "voice", IconKeyWave},
		{"compass", "有模型与免模型强化学习到底差在哪一步？", "转移概率和采样", []string{"学习笔记"}, "text", IconKeyCompass},
		{"music", "按走路步频自动把歌单分成两组", "120 BPM 节奏", []string{"音乐"}, "text", IconKeyMusic},
		{"code", "给本地接口加一个轻量连接池", "Go 语言服务端重构", []string{"后端"}, "text", IconKeyCode},
		{"camera", "傍晚胶片街拍光线记录", "35mm 镜头画面", []string{"胶片"}, "image", IconKeyCamera},
		{"book", "周末重读一遍那本小说摘录", "书页边注与背词", []string{"阅读"}, "text", IconKeyBook},
		{"coffee", "早晨手冲咖啡与书桌整理习惯", "日常微仪式", []string{"生活"}, "text", IconKeyCoffee},
		{"fallback_voice", "刚才想到一个点子", "", nil, "voice", IconKeyWave},
		{"fallback_image", "拍了一张图", "", nil, "image", IconKeyCamera},
		{"fallback_generic", "突然脑洞大开", "", nil, "text", IconKeyBulb},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := InferIdeaIconKey(tc.title, tc.summary, tc.tags, tc.modality)
			if got != tc.want {
				t.Fatalf("InferIdeaIconKey(%q) = %q, want %q", tc.title, got, tc.want)
			}
		})
	}
}

func TestNormalizeIconKeyAndTheme(t *testing.T) {
	aliases := map[string]string{
		"petals":        IconKeyBotanical,
		"sprout":        IconKeyBotanical,
		"orbit":         IconKeyCompass,
		"audio":         IconKeyWave,
		"art":           IconKeyPrism,
		"constellation": IconKeyBulb,
		"BOTANICAL":     IconKeyBotanical,
		"unknown":       "",
	}
	for raw, want := range aliases {
		if got := NormalizeIconKey(raw); got != want {
			t.Fatalf("NormalizeIconKey(%q) = %q, want %q", raw, got, want)
		}
	}

	if InferThemeForIcon(IconKeyBotanical) != ThemeCoral {
		t.Fatalf("expected ThemeCoral for botanical")
	}
	if InferThemeForIcon(IconKeyCompass) != ThemeSky {
		t.Fatalf("expected ThemeSky for compass")
	}
	if InferThemeForIcon(IconKeyWave) != ThemeMint {
		t.Fatalf("expected ThemeMint for wave")
	}
	if InferThemeForIcon(IconKeyPrism) != ThemeLavender {
		t.Fatalf("expected ThemeLavender for prism")
	}
	if InferThemeForIcon(IconKeyBulb) != ThemeAmber {
		t.Fatalf("expected ThemeAmber for bulb")
	}
}

func TestSceneContextLocationSummaryLine(t *testing.T) {
	sc := SceneContext{
		TimeOfDay:   "21:12 晚间散步",
		Location:    "宿舍旁街角花店",
		MoodWeather: "微风 · 放松",
	}
	got := sc.LocationSummaryLine()
	want := "晚间散步 · 宿舍旁街角花店 · 微风 · 放松"
	if got != want {
		t.Fatalf("LocationSummaryLine() = %q, want %q", got, want)
	}
}
