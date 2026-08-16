# Theme — ModelMux Aurora Console

## Part 1 — Compact token summary

Brand name: **Aurora Console**. Design language: "极光" (Aurora) — deep-space observatory feel, violet primary with cyan accent gradient, glass surfaces, cool-toned shadows. Bilingual UI (Chinese primary).

### Colors (light / dark)

| Token | Light | Dark |
|---|---|---|
| Canvas bg (`--mm-bg`) | `#F6F7FB` | `#0B0D12` |
| Surface muted L1 | `#EEF1F6` | `#141821` |
| Surface soft L2 | `#F2F5FA` | `#181D27` |
| Surface L3 (cards) | `#FFFFFF` | `#1E2430` |
| Surface strong L4 | `#FFFFFF` | `#262D3B` |
| Text primary | `#0F172A` | `#E6E9F0` |
| Text secondary | `#1E293B` | `#AEB4C2` |
| Text muted | `#64748B` | `#7C8294` |
| Text subtle | `#94A3B8` | `#5A6072` |
| Border | `rgba(15,23,42,0.12)` | `rgba(255,255,255,0.08)` |
| Divider | `#E2E8F0` | `#2A3140` |
| **Primary (violet)** | `#6A58E0` | `#7C6CF0` |
| Primary text | `#5A48D0` | `#9B8EF5` |
| Primary strong | `#4A38C0` | `#B3A8F8` |
| Primary soft bg | `#EEF0FF` | `rgba(124,108,240,0.14)` |
| Success | `#2DA44E` | `#3FB950` |
| Warning | `#C77700` | `#F5A623` |
| Error | `#CF222E` | `#F85149` |
| Accent (cross-theme) | `#7C6CF0` | same |
| Cyan accent | `#39C5CF` | same |
| Signal amber | `#FFB454` | same |
| Live green | `#3FB950` | same |

### Brand gradients

- `--mm-aurora`: `linear-gradient(120deg, #7C6CF0, #5B8DEF 45%, #39C5CF)` — logo tile, active indicator bars, brand kicker text
- `--mm-aurora-soft`: `linear-gradient(120deg, rgba(124,108,240,0.14), rgba(57,197,207,0.12))` — active nav background
- Shell background wash: radial `accent 14%` top-left + `#39C5CF 11%` top-right, animated 18s drift; 24px grid lines with radial mask

### Typography

- Display: `"Space Grotesk Variable"` (numbers/titles, `letter-spacing: -0.04em`), fallback FrexSansGB/Noto Sans SC
- Body: `"Inter Variable"`, fallback `FrexSansGB` (self-hosted GB2312 Chinese), PingFang SC, Microsoft YaHei, Noto Sans SC
- Mono: `"JetBrains Mono Variable"` — metric values, timestamps, code (tabular-nums)
- Base size 15px, line-height 1.62; kicker/label style: 0.66–0.72rem, weight 700–800, `letter-spacing: 0.1–0.14em`, uppercase
- Section titles: `Typography.Title level={3}` with `section-title` class

### Radii

- sm 6px / default 10px / lg 14px / pill 999px（Tag、live pill、metric pill、chip）

### Shadows

- Default: `0 4px 24px rgba(15,23,42,0.08)` (dark: `rgba(0,0,0,0.45)`)
- Hover: `0 10px 32px rgba(15,23,42,0.12)` (dark: `0 10px 36px rgba(0,0,0,0.55)`)
- Glow (active nav): `0 0 0 1px rgba(124,108,240,0.35), 0 8px 30px rgba(124,108,240,0.22)` (light uses softer variant)
- Panel: `4px 0 20px rgba(15,23,42,0.06)` (sidebar)

### Spacing

4px base scale: 4/8/12/16/20/24/32/40/56/72. Content padding `20px 24px 32px`; card padding 22px (antd Card paddingLG override); page section gap 20px (`Space size={20}`).

### Animation

- Easing: `--ease-out: cubic-bezier(0.22,1,0.36,1)`; `--ease-spring: cubic-bezier(0.34,1.56,0.64,1)` (toggle/icon micro-interactions); `--ease-smooth: cubic-bezier(0.4,0,0.2,1)` (nav hover)
- Durations: fast 120ms / normal 200ms / slow 320ms / enter 420ms / exit 250ms / complex 500ms
- Motifs: nav active left bar grows (300ms), aurora background drift 18s infinite, live dot pulse 1.8s, page transition fade/slide (`.page-transition.css`), theme switch soft-fade overlay (340ms), reveal-card staggered entrance
- `prefers-reduced-motion: reduce` → all animations 0.01ms

### Ant Design tokens (from `app-theme.ts`)

- `colorPrimary` = violet, radii 10/14/6/4, controlHeight 40 (SM 32, LG 46), fontSize 15 (SM 13), lineHeight 1.6
- Button: primary shadow violet glow, paddingInline 18, fontWeight 600
- Table: headerBg `#F2F5FA`/`#181D27`, headerColor muted, rowHoverBg `#EEF0FF`/violet 10%
- Input: violet active/hover border + focus ring `rgba(106,88,224,0.14)`
- Segmented: white selected item on `#EEF1F6` track (dark: `#262D3B` on `#141821`)
- Modal/Drawer: surface L3 bg

## Part 2 — Raw source dumps

### base.css (design token source of truth, full)

```css
/* === FrexSansGB — 中文字体（GB2312 子集 · 静态多字重 · 自托管） === */
@font-face {
  font-family: "FrexSansGB";
  src: url("../assets/fonts/FrexSansGB-400.woff2") format("woff2");
  font-weight: 400;
  font-style: normal;
  font-display: swap;
}
@font-face {
  font-family: "FrexSansGB";
  src: url("../assets/fonts/FrexSansGB-500.woff2") format("woff2");
  font-weight: 500;
  font-style: normal;
  font-display: swap;
}
@font-face {
  font-family: "FrexSansGB";
  src: url("../assets/fonts/FrexSansGB-600.woff2") format("woff2");
  font-weight: 600;
  font-style: normal;
  font-display: swap;
}
@font-face {
  font-family: "FrexSansGB";
  src: url("../assets/fonts/FrexSansGB-700.woff2") format("woff2");
  font-weight: 700;
  font-style: normal;
  font-display: swap;
}

:root {
  color-scheme: light;
  font-family: "Inter Variable", "FrexSansGB", "PingFang SC", "Microsoft YaHei", "Noto Sans SC", system-ui, sans-serif;
  font-size: 15px;
  line-height: 1.62;
  font-weight: 400;
  font-feature-settings: "cv02", "cv03", "cv04", "cv11";
  background: var(--mm-bg);
  color: var(--mm-text);

  /* === Aurora Console — Light Token Layer === */

  /* Canvas & Background */
  --mm-bg: #F6F7FB;
  --mm-body-bg: #F6F7FB;

  /* Glass surfaces */
  --mm-glass-bg: #FFFFFF;
  --mm-glass-bg-strong: #FFFFFF;
  --mm-glass-bg-subtle: rgba(255, 255, 255, 0.7);
  --mm-glass-border: rgba(15, 23, 42, 0.10);
  --mm-glass-border-strong: rgba(15, 23, 42, 0.16);
  --mm-glass-highlight: rgba(255, 255, 255, 0.9);

  /* Surfaces — progressive elevation */
  --mm-surface-muted: #EEF1F6;     /* L1: 内嵌区域 */
  --mm-surface-soft: #F2F5FA;      /* L2 */
  --mm-surface: #FFFFFF;           /* L3: 标准卡片 */
  --mm-surface-strong: #FFFFFF;    /* L4 */
  --mm-surface-solid: #FFFFFF;     /* L5 */
  --mm-surface-active: #EEF0FF;    /* 极光软高亮 */

  /* Text — 4-level hierarchy */
  --mm-text: #0F172A;
  --mm-text-secondary: #1E293B;
  --mm-text-muted: #64748B;
  --mm-text-subtle: #94A3B8;
  --mm-text-inverted: #FFFFFF;

  /* Borders & Dividers */
  --mm-border: rgba(15, 23, 42, 0.12);
  --mm-border-soft: rgba(15, 23, 42, 0.08);
  --mm-border-muted: #E2E8F0;
  --mm-divider: #E2E8F0;

  /* Primary — Aurora Violet */
  --mm-primary: #6A58E0;
  --mm-primary-text: #5A48D0;
  --mm-primary-strong: #4A38C0;
  --mm-primary-soft: #EEF0FF;
  --mm-primary-softer: #F5F6FF;
  --mm-primary-border: #C8CCF5;
  --mm-primary-border-strong: #B0B6F0;

  /* Semantic */
  --mm-success: #2DA44E;
  --mm-success-text: #1E8E3E;
  --mm-success-soft: #E7F6EC;
  --mm-warning: #C77700;
  --mm-warning-text: #A86500;
  --mm-warning-soft: #FCF3E3;
  --mm-error: #CF222E;
  --mm-error-text: #B91C28;
  --mm-error-soft: #FCEBEC;

  /* Shadows — cool-toned */
  --mm-shadow: 0 4px 24px rgba(15, 23, 42, 0.08);
  --mm-shadow-hover: 0 10px 32px rgba(15, 23, 42, 0.12);
  --mm-shadow-soft: 0 2px 12px rgba(15, 23, 42, 0.06);
  --mm-shadow-panel: 4px 0 20px rgba(15, 23, 42, 0.06);
  --mm-shadow-inset: inset 0 1px 0 rgba(255, 255, 255, 0.9);

  /* Selection */
  --mm-selection: rgba(106, 88, 224, 0.18);

  /* === Aurora Brand & Signal (cross-theme) === */
  --mm-accent: #7C6CF0;
  --mm-accent-strong: #6A58E0;
  --mm-aurora: linear-gradient(120deg, #7C6CF0, #5B8DEF 45%, #39C5CF);
  --mm-aurora-soft: linear-gradient(120deg, rgba(124, 108, 240, 0.14), rgba(57, 197, 207, 0.12));
  --mm-signal: #FFB454;
  --mm-signal-strong: #F59E2E;
  --mm-signal-soft: rgba(255, 180, 84, 0.14);

  /* Radii */
  --mm-radius-sm: 6px;
  --mm-radius: 10px;
  --mm-radius-lg: 14px;
  --mm-radius-pill: 999px;

  /* Spacing scale (4px base) */
  --mm-sp-1: 4px;
  --mm-sp-2: 8px;
  --mm-sp-3: 12px;
  --mm-sp-4: 16px;
  --mm-sp-5: 20px;
  --mm-sp-6: 24px;
  --mm-sp-7: 32px;
  --mm-sp-8: 40px;
  --mm-sp-9: 56px;
  --mm-sp-10: 72px;

  /* Typography */
  --font-display: "Space Grotesk Variable", "FrexSansGB", "Noto Sans SC", system-ui, sans-serif;
  --font-body: "Inter Variable", "FrexSansGB", "PingFang SC", "Microsoft YaHei", "Noto Sans SC", system-ui, sans-serif;
  --font-mono: "JetBrains Mono Variable", ui-monospace, "SF Mono", Menlo, Consolas, monospace;

  /* === Animation System === */

  /* Easing Curves */
  --ease-out: cubic-bezier(0.22, 1, 0.36, 1);
  --ease-in-out: cubic-bezier(0.64, 0.04, 0.35, 1);
  --ease-spring: cubic-bezier(0.34, 1.56, 0.64, 1);
  --ease-bounce: cubic-bezier(0.68, -0.55, 0.265, 1.55);
  --ease-smooth: cubic-bezier(0.4, 0, 0.2, 1);

  /* Duration Scale */
  --duration-fast: 120ms;
  --duration-normal: 200ms;
  --duration-slow: 320ms;
  --duration-enter: 420ms;
  --duration-exit: 250ms;
  --duration-complex: 500ms;
}

/* === Dark Theme — Aurora Console deep-space ink === */
:root[data-theme="dark"] {
  color-scheme: dark;

  /* Canvas — deep space ink */
  --mm-bg: #0B0D12;
  --mm-body-bg: #0B0D12;

  /* Glass surfaces */
  --mm-glass-bg: #141821;
  --mm-glass-bg-strong: #1A202C;
  --mm-glass-bg-subtle: rgba(20, 24, 33, 0.6);
  --mm-glass-border: rgba(124, 108, 240, 0.12);
  --mm-glass-border-strong: rgba(124, 108, 240, 0.20);
  --mm-glass-highlight: rgba(255, 255, 255, 0.04);

  /* Surfaces — progressive elevation */
  --mm-surface-muted: #141821;     /* L1: 内嵌区域 */
  --mm-surface-soft: #181D27;      /* L2 */
  --mm-surface: #1E2430;           /* L3: 标准卡片 */
  --mm-surface-strong: #262D3B;    /* L4 */
  --mm-surface-solid: #2E3645;     /* L5 */
  --mm-surface-active: rgba(124, 108, 240, 0.14);

  /* Text — 4-level hierarchy */
  --mm-text: #E6E9F0;
  --mm-text-secondary: #AEB4C2;
  --mm-text-muted: #7C8294;
  --mm-text-subtle: #5A6072;
  --mm-text-inverted: #0B0D12;

  /* Borders & Dividers */
  --mm-border: rgba(255, 255, 255, 0.08);
  --mm-border-soft: rgba(255, 255, 255, 0.05);
  --mm-border-muted: #2A3140;
  --mm-divider: #2A3140;

  /* Primary — Aurora Violet */
  --mm-primary: #7C6CF0;
  --mm-primary-text: #9B8EF5;
  --mm-primary-strong: #B3A8F8;
  --mm-primary-soft: rgba(124, 108, 240, 0.14);
  --mm-primary-softer: rgba(124, 108, 240, 0.07);
  --mm-primary-border: rgba(124, 108, 240, 0.30);
  --mm-primary-border-strong: rgba(124, 108, 240, 0.45);

  /* Semantic */
  --mm-success: #3FB950;
  --mm-success-text: #56D364;
  --mm-success-soft: rgba(63, 185, 80, 0.14);
  --mm-warning: #F5A623;
  --mm-warning-text: #F7B955;
  --mm-warning-soft: rgba(245, 166, 35, 0.14);
  --mm-error: #F85149;
  --mm-error-text: #FF7B72;
  --mm-error-soft: rgba(248, 81, 73, 0.14);

  /* Shadows — cool deep */
  --mm-shadow: 0 4px 24px rgba(0, 0, 0, 0.45);
  --mm-shadow-hover: 0 10px 36px rgba(0, 0, 0, 0.55);
  --mm-shadow-soft: 0 2px 12px rgba(0, 0, 0, 0.35);
  --mm-shadow-panel: 4px 0 20px rgba(0, 0, 0, 0.40);
  --mm-shadow-inset: inset 0 1px 0 rgba(255, 255, 255, 0.04);

  /* Selection */
  --mm-selection: rgba(124, 108, 240, 0.30);

  /* Extended tokens */
  --mm-text-primary: #9B8EF5;
  --mm-surface-dark: #0B0D12;
  --mm-border-dark: #2A3140;
  --mm-shadow-float: 0 4px 16px rgba(0, 0, 0, 0.50);
  --mm-grid-line: rgba(255, 255, 255, 0.028);
  --mm-glow: 0 0 0 1px rgba(124, 108, 240, 0.35), 0 8px 30px rgba(124, 108, 240, 0.22);
  --mm-glow-soft: 0 0 0 1px rgba(124, 108, 240, 0.22), 0 4px 18px rgba(124, 108, 240, 0.14);
  --mm-live: #3FB950;
  --mm-live-soft: rgba(63, 185, 80, 0.16);
  --mm-cyan: #39C5CF;
  --mm-cyan-soft: rgba(57, 197, 207, 0.14);

  /* Animation tokens inherit from :root */
}

* {
  box-sizing: border-box;
}

html,
body,
#root {
  min-height: 100%;
  margin: 0;
}

body {
  min-height: 100vh;
  text-rendering: geometricPrecision;
  -webkit-font-smoothing: antialiased;
  background: var(--mm-body-bg);
  color: var(--mm-text);
}

body::selection {
  background: var(--mm-selection);
}

a {
  color: inherit;
}

@media (prefers-reduced-motion: no-preference) {
  .theme-transition,
  .theme-transition *,
  .theme-transition *::before,
  .theme-transition *::after {
    transition:
      background-color 220ms ease,
      background 220ms ease,
      border-color 220ms ease,
      color 220ms ease,
      box-shadow 220ms ease,
      fill 220ms ease,
      stroke 220ms ease !important;
  }

  .theme-soft-fade {
    position: fixed;
    inset: 0;
    z-index: 2147483647;
    pointer-events: none;
    background:
      radial-gradient(circle at 82% 16%, color-mix(in srgb, var(--theme-fade-surface) 46%, transparent), transparent 34%),
      var(--theme-fade-bg);
    animation: theme-soft-fade 340ms cubic-bezier(0.22, 1, 0.36, 1) both;
    will-change: opacity, filter;
  }

  @keyframes theme-soft-fade {
    0% {
      opacity: 0.72;
      filter: blur(0) saturate(1);
    }

    100% {
      opacity: 0;
      filter: blur(10px) saturate(1.04);
    }
  }
}

button,
input,
textarea,
select {
  font: inherit;
}

@media (prefers-reduced-motion: reduce) {
  *,
  *::before,
  *::after {
    animation-duration: 0.01ms !important;
    animation-iteration-count: 1 !important;
    transition-duration: 0.01ms !important;
  }
}
```

### app-theme.ts (Ant Design theme config, full)

```ts
import { theme, type ThemeConfig } from "antd";

export type AppThemeMode = "light" | "dark";

export function createAppTheme(mode: AppThemeMode): ThemeConfig {
  const dark = mode === "dark";

  return {
    algorithm: dark ? theme.darkAlgorithm : theme.defaultAlgorithm,
    token: {
      /* === Aurora Console Palette (single source of truth: base.css) === */
      colorPrimary: dark ? "#7C6CF0" : "#6A58E0",
      colorInfo: dark ? "#7C6CF0" : "#6A58E0",
      colorSuccess: dark ? "#3FB950" : "#2DA44E",
      colorWarning: dark ? "#F5A623" : "#C77700",
      colorError: dark ? "#F85149" : "#CF222E",
      colorText: dark ? "#E6E9F0" : "#0F172A",
      colorTextSecondary: dark ? "#AEB4C2" : "#1E293B",
      colorTextTertiary: dark ? "#7C8294" : "#64748B",
      colorTextQuaternary: dark ? "#5A6072" : "#94A3B8",
      colorBorder: dark ? "#2A3140" : "#E2E8F0",
      colorBorderSecondary: dark ? "#2A3140" : "#E2E8F0",
      colorBgBase: dark ? "#0B0D12" : "#F6F7FB",
      colorBgLayout: dark ? "#0B0D12" : "#F6F7FB",
      colorBgContainer: dark ? "#1E2430" : "#FFFFFF",
      colorBgElevated: dark ? "#262D3B" : "#FFFFFF",
      colorFillSecondary: dark ? "rgba(124, 108, 240, 0.10)" : "rgba(106, 88, 224, 0.06)",
      colorFillTertiary: dark ? "rgba(255, 255, 255, 0.04)" : "rgba(15, 23, 42, 0.04)",
      colorLink: dark ? "#9B8EF5" : "#5A48D0",
      colorLinkHover: dark ? "#B3A8F8" : "#4A38C0",

      /* === Aurora radii === */
      borderRadius: 10,
      borderRadiusLG: 14,
      borderRadiusSM: 6,
      borderRadiusXS: 4,

      /* === Controls === */
      controlHeight: 40,
      controlHeightSM: 32,
      controlHeightLG: 46,

      /* === Typography === */
      fontSize: 15,
      fontSizeSM: 13,
      lineHeight: 1.6,
      padding: 16,
      paddingSM: 12,
      paddingXS: 8,
      margin: 16,
      marginSM: 12,
      marginXS: 8,
      fontFamily:
        '"Inter Variable", "FrexSansGB", "PingFang SC", "Microsoft YaHei", "Noto Sans SC", system-ui, sans-serif',
      fontFamilyCode:
        '"JetBrains Mono Variable", ui-monospace, "SF Mono", Menlo, Consolas, monospace',
      boxShadow: dark
        ? "0 4px 24px rgba(0, 0, 0, 0.45)"
        : "0 4px 24px rgba(15, 23, 42, 0.08)",
      boxShadowSecondary: dark
        ? "0 10px 36px rgba(0, 0, 0, 0.55)"
        : "0 10px 32px rgba(15, 23, 42, 0.12)",
    },
    components: {
      Button: {
        primaryShadow: dark
          ? "0 4px 16px rgba(124, 108, 240, 0.45)"
          : "0 4px 16px rgba(106, 88, 224, 0.30)",
        paddingInline: 18,
        fontWeight: 600,
      },
      Tag: {
        borderRadiusSM: 999,
        fontSizeSM: 12,
        lineHeightSM: 1.8,
      },
      Card: {
        paddingLG: 22,
      },
      Table: {
        headerBg: dark ? "#181D27" : "#F2F5FA",
        headerColor: dark ? "#7C8294" : "#64748B",
        rowHoverBg: dark ? "rgba(124, 108, 240, 0.10)" : "#EEF0FF",
        borderColor: dark ? "#2A3140" : "#E2E8F0",
      },
      Input: {
        activeBorderColor: dark ? "#7C6CF0" : "#6A58E0",
        hoverBorderColor: dark ? "#9B8EF5" : "#5A48D0",
        activeShadow: dark
          ? "0 0 0 3px rgba(124, 108, 240, 0.18)"
          : "0 0 0 3px rgba(106, 88, 224, 0.14)",
      },
      Select: {
        optionSelectedBg: dark ? "rgba(124, 108, 240, 0.14)" : "#EEF0FF",
      },
      Segmented: {
        itemSelectedBg: dark ? "#262D3B" : "#FFFFFF",
        trackBg: dark ? "#141821" : "#EEF1F6",
      },
      Modal: {
        contentBg: dark ? "#1E2430" : "#FFFFFF",
        headerBg: dark ? "#1E2430" : "#FFFFFF",
      },
      Drawer: {
        colorBgElevated: dark ? "#1E2430" : "#FFFFFF",
      },
      Tooltip: {
        colorBgSpotlight: dark ? "#262D3B" : "#1E293B",
      },
    },
  };
}
```

### Theme provider notes

- `web/src/app/theme-mode.tsx` — `AppThemeProvider`: mode state (light/dark), persisted in `localStorage["modelmux-theme"]`, falls back to `prefers-color-scheme`; applies `data-theme` attribute on `<html>`; soft-fade overlay animation on switch; antd `ConfigProvider` with zhCN locale.
- CSS import order (`src/styles.css`): animations → base → layout → surfaces → dashboard → providers → events → stats → settings → shared → about → responsive.