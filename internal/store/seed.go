package store

import (
	"time"

	"github.com/pengstem/neromancer/internal/domain"
)

// SeedIfEmpty populates the vault with curated demonstration inspirations on first launch.
// All data reflects the team's design examples (Blue Vase Tulip merge, conversational RL note, etc.).
func (s *Store) SeedIfEmpty() error {
	existing, err := s.ListNodes()
	if err != nil {
		return err
	}
	if len(existing) > 0 {
		return nil
	}

	now := time.Now()
	nodes := []domain.IdeaNode{
		{
			ID:                    "orb_tulip",
			Title:                 "用一个蓝色花瓶养一株郁金香",
			ConversationalSummary: "散步路过花店时你想起阳台还空着，二十分钟后刷到手工器皿图又补了一笔：花瓶一定要是透亮钴蓝。暖珊瑚粉的花瓣撞上冷调蓝釉，每天早上拉开窗帘都像给书桌开了盏小夜灯。",
			Tags:                  []string{"生活美学", "阳台植物", "颜色碰撞"},
			Theme:                 domain.ThemeCoral,
			Visual: domain.VisualMotif{
				MotifType: "petals",
				AccentHex: "#3B82F6",
				Caption:   "珊瑚花瓣 × 钴蓝玻璃的晨间仪式",
				Energy:    0.92,
			},
			Weight: 0.94,
			Stage:  domain.StageSprouting,
			GuidingQuestions: []string{
				"比起买一大束，单支郁金香在蓝瓶里每天‘低头弯腰’的变化是不是更有陪伴感？",
				"如果花期只有七天，凋谢后那个蓝色花瓶你想用来插干枝还是留给下一季？",
				"阳台早晨十点的光线穿过蓝玻璃投在白墙上，要不要顺手拍成一组七日延时？",
			},
			ActionPlanMD: "",
			RelatedIDs:   []string{"orb_watch"},
			CreatedAt:    now.Add(-48 * time.Hour),
			UpdatedAt:    now.Add(-2 * time.Hour),
		},
		{
			ID:                    "orb_rl_prose",
			Title:                 "有模型与免模型强化学习到底差在哪一步？",
			ConversationalSummary: "常听人说有模型学习和免模型学习（Model-Based / Model-Free），这两者究竟差在哪一步？一句话答案：差在智能体手里有没有转移概率。有就解方程或求和，没有就只能采样或求平均。",
			Tags:                  []string{"学习笔记", "强化学习", "马尔可夫"},
			Theme:                 domain.ThemeSky,
			Visual: domain.VisualMotif{
				MotifType: "orbit",
				AccentHex: "#0284C7",
				Caption:   "手握地图算账 vs 蒙眼跑局采样",
				Energy:    0.84,
			},
			Weight: 0.86,
			Stage:  domain.StageIncubating,
			GuidingQuestions: []string{
				"能不能顺着这个思路，把‘动态规划’和‘蒙特卡洛’也压缩成一句菜市场讲价级别的大白话？",
				"如果环境规则只知道一半（半模型），智能体什么时候该停下来算，什么时候该先试一局？",
			},
			ActionPlanMD: "### 三步把硬核公式讲成“听别人聊天”\n\n- **第一步（5分钟）**：挑出最劝退人的那个符号（比如转移概率 $P(s'|s,a)$），先不写公式，口头录一段 30 秒语音讲给完全不懂 CS 的朋友听。\n- **第二步（20分钟）**：画一张左右对照小草图——左边是“手里有地图直接按图算账”，右边是“没地图只能多跑几局记平均分”。\n- **第三步（本周）**：把这类笔记收进同一个记忆球簇，攒够五张就能直接拼成期末复盘小册子。",
			RelatedIDs:   []string{"orb_shader"},
			CreatedAt:    now.Add(-72 * time.Hour),
			UpdatedAt:    now.Add(-5 * time.Hour),
		},
		{
			ID:                    "orb_watch",
			Title:                 "手表端做一颗单键盲操语音记忆球",
			ConversationalSummary: "在路上灵光一闪时掏手机解锁还是慢了半拍。手表上根本不需要复杂排版界面，只要表盘边缘留一颗温润的语音键：抬腕按住说十秒就收进气泡，回到电脑前再看 AI 把它变成什么样。",
			Tags:                  []string{"穿戴交互", "多端协同", "硬件交互"},
			Theme:                 domain.ThemeMint,
			Visual: domain.VisualMotif{
				MotifType: "wave",
				AccentHex: "#0D9488",
				Caption:   "抬腕即记：零视觉负担的十秒捕获",
				Energy:    0.78,
			},
			Weight: 0.78,
			Stage:  domain.StageSpark,
			GuidingQuestions: []string{
				"如果手表录音时震动反馈模仿“弹珠落进玻璃罐”的清脆手感，会不会更有记录欲？",
				"在地铁噪杂环境里，录完直接留原声，等回到静音环境再批量转写是不是比现场强行识别更好？",
			},
			RelatedIDs: []string{"orb_tulip"},
			CreatedAt:  now.Add(-36 * time.Hour),
			UpdatedAt:  now.Add(-6 * time.Hour),
		},
		{
			ID:                    "orb_shader",
			Title:                 "用不完美噪点做温暖的手绘感着色器",
			ConversationalSummary: "别一上来就追完美物理渲染了！昨晚试着故意把法线贴图弄歪一点，再叠上一层纸张纤维的颗粒抖动，冷冰冰的多边形突然像水彩插画书一样呼吸起来。",
			Tags:                  []string{"图形学", "手绘质感", "着色器"},
			Theme:                 domain.ThemeLavender,
			Visual: domain.VisualMotif{
				MotifType: "prism",
				AccentHex: "#7C3AED",
				Caption:   "故意保留 15% 的错误与笔触颗粒",
				Energy:    0.72,
			},
			Weight: 0.71,
			Stage:  domain.StageCrystallized,
			GuidingQuestions: []string{
				"如果把这套‘故意留瑕疵’的边缘抖动用在记忆球的发光边缘上，会不会比完美圆更像手吹玻璃？",
			},
			RelatedIDs: []string{"orb_rl_prose"},
			CreatedAt:  now.Add(-96 * time.Hour),
			UpdatedAt:  now.Add(-12 * time.Hour),
		},
	}

	for _, n := range nodes {
		if err := s.UpsertNode(n); err != nil {
			return err
		}
	}

	evidences := []domain.RawEvidence{
		{
			ID:         "ev_tulip_1",
			NodeID:     "orb_tulip",
			Modality:   "voice",
			RawText:    "买一盆郁金香放在阳台上养，感觉每天早上拉开窗帘心情都会亮起来。",
			DurationMs: 6400,
			Scene: domain.SceneContext{
				TimeOfDay:   "21:12 晚间散步",
				Location:    "宿舍旁街角花店",
				MoodWeather: "微风 · 放松",
				NowPlaying:  "City Pop 随听",
			},
			CapturedAt: now.Add(-48 * time.Hour),
		},
		{
			ID:       "ev_tulip_2",
			NodeID:   "orb_tulip",
			Modality: "image",
			RawText:  "郁金香的花瓶应该是蓝色的，最好是透光一点的钴蓝玻璃配橙粉色花瓣。",
			Scene: domain.SceneContext{
				TimeOfDay:   "21:35 睡前刷图",
				Location:    "书桌前",
				MoodWeather: "安静 · 期待",
				NowPlaying:  "雨声白噪音",
			},
			CapturedAt: now.Add(-47*time.Hour - 37*time.Minute),
		},
		{
			ID:       "ev_rl_1",
			NodeID:   "orb_rl_prose",
			Modality: "text",
			RawText:  "Model-Based 和 Model-Free 到底差啥？说白了不就是手头有没有转移概率表吗，有就解方程，没有就只能跑局采样求平均。",
			Scene: domain.SceneContext{
				TimeOfDay:   "15:40 下午复盘",
				Location:    "图书馆三楼窗边",
				MoodWeather: "晴朗 · 恍然大悟",
			},
			CapturedAt: now.Add(-72 * time.Hour),
		},
		{
			ID:         "ev_watch_1",
			NodeID:     "orb_watch",
			Modality:   "voice",
			RawText:    "要是手表侧面能直接按一下就录音记灵感就好了，不用掏手机选文件夹，十秒搞定。",
			DurationMs: 5200,
			Scene: domain.SceneContext{
				TimeOfDay:   "08:45 早高峰",
				Location:    "地铁换乘通道",
				MoodWeather: "赶时间 · 灵光一闪",
			},
			CapturedAt: now.Add(-36 * time.Hour),
		},
		{
			ID:       "ev_shader_1",
			NodeID:   "orb_shader",
			Modality: "sketch",
			RawText:  "把法线扰动故意加粗一点点反而更有手绘感，太工整的参数像 AI 味儿太重了。",
			Scene: domain.SceneContext{
				TimeOfDay:   "23:18 深夜实验",
				Location:    "宿舍桌前",
				MoodWeather: "微雨 · 兴奋",
				NowPlaying:  "Lo-Fi Beats",
			},
			CapturedAt: now.Add(-96 * time.Hour),
		},
	}

	for _, ev := range evidences {
		if err := s.AddRawEvidence(ev); err != nil {
			return err
		}
	}

	snapshots := []domain.StageSnapshot{
		{
			ID:        "snap_tulip_1",
			NodeID:    "orb_tulip",
			Stage:     domain.StageSpark,
			Title:     "买一盆郁金香放在阳台上养",
			Summary:   "散步经过花店时的第一句随口语音。",
			Note:      "初次捕捉：语音录入",
			CreatedAt: now.Add(-48 * time.Hour),
		},
		{
			ID:        "snap_tulip_2",
			NodeID:    "orb_tulip",
			Stage:     domain.StageSprouting,
			Title:     "用一个蓝色花瓶养一株郁金香",
			Summary:   "Agent 识别与半小时前的花瓶配色闪念同源，自动去重归并为一条完整生活灵感，同时保留两条原始证据。",
			Note:      "AI 自动去重归并（2 条原始证据）",
			CreatedAt: now.Add(-47 * time.Hour),
		},
	}
	for _, snap := range snapshots {
		if err := s.AddStageSnapshot(snap); err != nil {
			return err
		}
	}

	slices := []domain.RoamSlice{
		{
			ID:          "roam_framegraph",
			AuthorAlias: "图形学废料箱 · 阿泽",
			Title:       "把游戏引擎 FrameGraph 渲染管线类比成厨房出菜台",
			Snippet:     "大家总把 RenderPass 写得像天书，其实和沙县小吃出餐一样：哪些菜能共用一口锅（瞬态纹理复用），哪道菜必须等前一道焯水完（依赖屏障），画成连线图一眼就懂。",
			DomainTags:  []string{"游戏引擎", "渲染管线", "直觉类比"},
			Theme:       domain.ThemeSky,
			SceneHint:   "深夜改 Bug 改烦了随口吐槽",
		},
		{
			ID:          "roam_siphon",
			AuthorAlias: "阳台实验员 · 小林",
			Title:       "出远门用一条旧棉鞋带给花盆虹吸补水",
			Snippet:     "不需要买智能浇花器！玻璃水壶架高 15 厘米，棉线一头沉底一头埋进土里两指深，毛细作用自己就会按土壤干湿给水，土湿了自然停。",
			DomainTags:  []string{"生活黑客", "植物养护", "低成本"},
			Theme:       domain.ThemeMint,
			SceneHint:   "五一出门前十分钟救急想法",
		},
		{
			ID:          "roam_bpm_walk",
			AuthorAlias: "散步采样员 · Kiki",
			Title:       "按走路步频自动把歌单分成‘赶路’和‘发呆’两组",
			Snippet:     "很多歌单独听不出来情绪，其实看你听它的时候每分钟走多少步最诚实。120 步以上归进加速歌单，90 步以下全丢给傍晚散步。",
			DomainTags:  []string{"音乐", "生活数据", "可穿戴"},
			Theme:       domain.ThemeCoral,
			SceneHint:   "傍晚绕操场走第三圈",
		},
		{
			ID:          "roam_pet_vocab",
			AuthorAlias: "摸鱼产品 · 麦子",
			Title:       "背不完的单词不是扣分，而是变成桌宠偷吃的小饼干",
			Snippet:     "打卡软件总是惩罚感太重让人想卸载。如果连续三天没复习，小怪物只是抱着你的错题本在屏幕角落打瞌睡，点它一下就吐出一个昨天刚忘的词。",
			DomainTags:  []string{"学习", "反内卷交互", "桌宠"},
			Theme:       domain.ThemeAmber,
			SceneHint:   "早八困在课桌前",
		},
		{
			ID:          "roam_glass_caustics",
			AuthorAlias: "光影观察者 · 舟",
			Title:       "用廉价矿泉水瓶在桌面投影低成本水波纹焦散",
			Snippet:     "渲染器里算一遍真实焦散要好几毫秒，但物理世界里只要一盏台灯斜着穿过装半瓶水的波纹塑料瓶，桌面瞬间变成海底。",
			DomainTags:  []string{"物理光影", "摄影", "创意视觉"},
			Theme:       domain.ThemeLavender,
			SceneHint:   "下午四点阳光斜进实验室",
		},
	}
	for _, rs := range slices {
		if err := s.UpsertRoamSlice(rs); err != nil {
			return err
		}
	}

	return s.SaveSleepInsight(domain.SleepInsight{
		ID:               "sleep_seed_1",
		Title:            "昨晚睡前的小发现：玻璃蓝瓶 × 手绘颗粒感",
		ConnectedNodeIDs: []string{"orb_tulip", "orb_shader"},
		SparkSummary:     "你一边想找一只透光钴蓝花瓶养郁金香，另一边又在着色器里迷恋‘带一点不完美颗粒的手吹玻璃感’——要不要用自己写的抖动着色器给 Neromancer 的记忆球加一层清晨阳光穿过蓝玻璃的边缘光？",
		SuggestedAction:  "点进《用一个蓝色花瓶养一株郁金香》试试让 DeepSeek 把这两条线揉成一个迷你桌面视觉小玩具。",
		CreatedAt:        now.Add(-1 * time.Hour),
		Dismissed:        false,
	})
}
