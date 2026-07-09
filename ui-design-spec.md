# ModelMux UI 重构设计规范 · Aurora Console（极光控制台）

> 状态：评审稿 v1 · 2026-07-09
> 目标：彻底脱离现有"Anthropic 暖色编辑部"风格，建立科技感 + 人文温度并重的深色观测台体系。
> 配套样机：`ui-redesign-proposal.html`（可交互，含暗/亮双主题切换）。

---

## 0. 设计哲学（与旧版决裂点）

| 维度 | 旧版 (Anthropic) | 新版 (Aurora Console) |
|---|---|---|
| 画布 | 暖米色 `#eae6de` | 深空墨 `#0B0D12` |
| 标题字体 | 衬线 Times/Libre Baskerville | 几何无衬线 Space Grotesk |
| 主色 | 陶土红 `#b85f45` | 极光渐变 violet→cyan + 暖琥珀信号 |
| 圆角 | 12px 卡 / 24px 胶囊 | 10px 精准圆角 / 仅标签用胶囊 |
| 气质 | 克制、编辑、人文偏暖 | 冷静、精密、观测台偏科技，琥珀点亮温度 |
| 阴影 | 暖米调柔光 | 冷调微投影 + 极光辉光 |

**三原则**
1. **深底承载数据**：默认暗色，海量指标在深底上对比清晰、不刺眼。
2. **极光作科技、琥珀作温度**：交互/品牌/图表用极光渐变；告警、关键高光、活跃态用暖琥珀，避免"冰冷纯科技"。
3. **精准大于装饰**：更小圆角、等宽数字、明确网格，但保留弹性微动效的"活人感"。

---

## 1. 字体 (Typography)

彻底移除衬线标题，改用几何无衬线 + 等宽数字体系。

| 角色 | 字体 | 用途 | 回退 |
|---|---|---|---|
| Display / 标题 | **Space Grotesk** 400–700 | 页面标题、区块标题、Logo | Noto Sans SC, system-ui |
| Body / 正文 | **Inter** 400–700 | 所有正文、按钮、表格 | Noto Sans SC, system-ui |
| Mono / 数值 | **JetBrains Mono** 400–700 | KPI 数字、key 哈希、时间戳、代码 | ui-monospace, Menlo |
| 中文 | **FrexSansGB**（自托管） | 中文正文与标题（GB2312 子集·400/500/600/700） | PingFang SC, Microsoft YaHei, Noto Sans SC |

> **中文字体已落地**：`web/src/assets/fonts/` 下 4 个静态 woff2（FrexSansGB-400/500/600/700，各 ~3.28MB，GB2312 子集，由 `variable/Frex Sans GB[wght].ttf` 经 fonttools 实例化+子集化生成）。`base.css` 已加 4 条 `@font-face` 并把 `"FrexSansGB"` 插入 `--font-body`/`--font-display` 的中文回落位；拉丁文仍走 Space Grotesk/Inter。`npm run build` 通过，字体进入 `dist/assets` 并被 `embed.go` 打入二进制（二进制增重约 13MB）。原始字体位于 `C:\Users\Administrator\Downloads\FrexSansGB\`。

**字阶（base 16px，rem）**
- Display XL：1.75rem / 600 / `-0.02em`（页面主标题）
- H1 区块：1.25rem / 600
- H2 卡片：1rem / 600
- Body：0.875rem / 400 / line-height 1.6
- Small：0.75rem / 500（标签、说明）
- Mono 数值：1.4–1.7rem / 700 / `tabular-nums`（KPI）

中文标题不再用衬线；中文与 Space Grotesk 混排时，中文走 Noto Sans SC 保证字重一致。

---

## 2. 配色 (Color)

### 2.1 品牌与信号（双轴）
- **Accent / 极光紫** `--accent:#7C6CF0`，亮态 `#6A58E0`
- **Accent 渐变** `--aurora: linear-gradient(120deg,#7C6CF0,#5B8DEF 45%,#39C5CF)`（Logo、主按钮、图表描边、活跃态）
- **Signal / 暖琥珀** `--signal:#FFB454`（人文温度：告警、关键高光、活跃指示、warning 级高光）

### 2.2 语义色
| 名 | 暗态 | 亮态 | 用途 |
|---|---|---|---|
| Success | `#3FB950` / text `#56D364` | `#2DA44E` | 健康、active |
| Warning | `#F5A623` | `#C77700` | 冷却、降级 |
| Error | `#F85149` / text `#FF7B72` | `#CF222E` | invalid、失败 |
| Info | `#39C5CF` | `#0B8C94` | 信息、链接 |

> 语义 soft 变体统一用 `rgba(...,.12~.14)` 作徽章底。

### 2.3 表面层级（5 级，暗态）
```
Canvas   --bg     #0B0D12
L1 内嵌   --s1     #141821
L2 软背景 --s2     #181D27
L3 卡片   --s3     #1E2430
L4 强调   --s4     #262D3B
L5 头部   --s5     #2E3645
```
亮态以白/`#F6F7FB` 为基底，s3/s4 用纯白，s1/s2 用浅灰蓝。
边框：`--border rgba(255,255,255,.08)` → strong `.14`；亮态 `rgba(15,23,42,.10)`。
文字四级：`--text #E6E9F0` / `--text-2 #AEB4C2` / `--text-muted #7C8294` / `--text-subtle #5A6072`。

---

## 3. 间距 (Spacing)

4px 基准栅格，控制但不过度宽松（对比旧版更"紧致精密"）：
```
--sp-1:4  --sp-2:8  --sp-3:12 --sp-4:16 --sp-5:20
--sp-6:24 --sp-7:32 --sp-8:40 --sp-9:56 --sp-10:72
```
- 卡片内边距：20–24px（旧版 24–28，略收）
- 区块间距：32–56px
- 栅格间隙：16–20px
- 行高：正文 1.6，标题 1.1–1.2

---

## 4. 圆角 / 阴影 / 描边 (Shape)

| 元素 | 半径 |
|---|---|
| 控件/输入框/标签 | 6px (`--r-sm`) |
| 卡片/按钮 | 10px (`--r`) |
| 面板/弹窗 | 14px (`--r-lg`) |
| 标签/胶囊/状态点 | 999px |

**阴影（暗态）**
- `--shadow: 0 4px 24px rgba(0,0,0,.45)`
- `--shadow-hover: 0 10px 36px rgba(0,0,0,.55)`
- `--shadow-glow: 0 0 0 1px rgba(124,108,240,.35), 0 8px 30px rgba(124,108,240,.22)`（主按钮/活跃态辉光）

旧版暖米柔光 → 冷调微投影 + 极光色辉光，强化"科技发光"质感。

---

## 5. 动效 (Motion)

**缓动**
```
--e-out:   cubic-bezier(.22,1,.36,1)   /* 入场/退场 */
--e-spring:cubic-bezier(.34,1.56,.64,1)/* 弹性微交互 */
--e-smooth:cubic-bezier(.4,0,.2,1)     /* 主题/状态 */
```
**时长**：fast 120 / normal 200 / slow 320 / enter 420 ms

**动效清单（全部尊重 `prefers-reduced-motion`）**
1. **页面/卡片入场** `reveal`：fade + translateY(10→0)，420ms，可错峰。
2. **极光漂移**：顶部背景径向渐变缓慢呼吸（观测台氛围，非干扰）。
3. **KPI 数值闪动** `kpi-flash`：scale 1→1.06 + 瞬时暖琥珀高亮，数值更新时触发。
4. **数值计数** count-up：大数加载时从 0 滚动到目标。
5. **骨架微光** `shimmer`：加载占位 1.6s 循环。
6. **信号脉冲** `glow-pulse`：live/活跃态外发光呼吸。
7. **按钮微交互**：hover translateY(-1px)+辉光；active scale(.96)。
8. **表格行 hover**：极光软高亮 `accent-soft` + 内描边。
9. **主题切换**：全站色板 220ms 平滑过渡 + 软遮罩淡出。

---

## 6. 组件样式方向（落地到 Ant Design 覆写）

- **按钮**：主按钮用 `--aurora` 渐变底 + 白字 + 辉光；次按钮描边 hover 转极光边。
- **表格**：圆角 10px 容器，表头小字母间距大写灰字，行 hover 极光软高亮。
- **标签/徽章**：胶囊形，语义 soft 底 + 语义 text 色，左侧状态点。
- **输入框/选择器**：focus 极光边 + `0 0 0 3px accent-soft` 光环。
- **卡片**：10px 圆角，hover 轻微上浮 + 辉光，左侧 3px 极光强调条（KPI 卡）。
- **图表**：折线/面积用 `--aurora` 描边 + 极光柔光填充；配色板固定 6–8 色（含琥珀作高亮系列）。

---

## 7. 待你确认 / 可选方向

- **A 极光控制台（主推）**：violet→cyan 极光 + 暖琥珀信号。已做完整样机。
- **B 青蓝实验室**：单色青蓝，更冷更工程，温度弱。
- **C 暖阳熔炉**：深色 + 饱和暖铜主色，温度优先。

**下一步**：你选定方向（及是否要我把 B/C 也出样机）后，我按此令牌体系重写 `web/src/styles/*` 并逐页适配组件，保留现有功能与 TanStack Query 结构不变。
