# ModelMux Aurora Console — Design System

## Product context

**ModelMux** is a local reverse proxy that fronts multiple model-API providers behind one address, rotating API keys intelligently. The admin console ("Aurora Console") is an operations dashboard for a single-machine local service: monitor provider/key health, switch active provider, manage keys, inspect call statistics, adjust settings, and audit events.

- **Users**: developers running ModelMux locally (single operator, technical)
- **Key mental model**: "mission control for API keys" — at-a-glance health of key pools (active / cooling / invalid), provider circuit state, live traffic
- **Key pages & flows**:
  - Dashboard (总览): active provider + key health signals + provider list with switch/delete
  - Providers (提供商): table + detail drawer with key management (test/reset/delete, batch import/export)
  - Stats (调用统计): KPI summary, timeline chart, per-model stats, recent call logs
  - Settings (设置): grouped config forms with hot-reload vs restart markers
  - Events (事件): filterable event log with detail drawer
  - About (关于): version info + config/state backup export
- **Constraints**: Chinese-language UI (zh-CN); must remain dense but scannable; single-window desktop-class layout (sidebar + header + content, max-width 1600px); keyboard shortcuts are a first-class feature (g→x navigation, Ctrl/Cmd+R reload)

## Branding & styling

**Brand name**: Aurora Console. **Concept**: 极光 (Aurora) — deep-space observatory. Violet is the primary identity color; cyan accents; faint animated aurora wash + observatory grid on the canvas; glass surfaces; cool-toned shadows.

### Color palette

| Role | Light | Dark |
|---|---|---|
| Canvas bg | `#F6F7FB` | `#0B0D12` |
| Card surface (L3) | `#FFFFFF` | `#1E2430` |
| Surface muted (L1) | `#EEF1F6` | `#141821` |
| Text primary | `#0F172A` | `#E6E9F0` |
| Text muted | `#64748B` | `#7C8294` |
| Border/divider | `#E2E8F0` / `rgba(15,23,42,.12)` | `#2A3140` / `rgba(255,255,255,.08)` |
| **Primary violet** | `#6A58E0` | `#7C6CF0` |
| Primary text | `#5A48D0` | `#9B8EF5` |
| Success | `#2DA44E` | `#3FB950` |
| Warning | `#C77700` | `#F5A623` |
| Error | `#CF222E` | `#F85149` |
| Cyan accent | `#39C5CF` (both) | — |

**Brand gradient** (logo tile, active indicator, kicker text):
`linear-gradient(120deg, #7C6CF0, #5B8DEF 45%, #39C5CF)`

**Canvas atmosphere**: radial violet glow top-left (14%) + cyan glow top-right (11%), slow 18s drift; faint 24px grid with radial mask. Sidebar has subtle violet-tinted gradient.

### Typography

- Display / headings / provider names: **Space Grotesk Variable**, tight letter-spacing (−0.02 to −0.04em)
- Body / UI: **Inter Variable** (Chinese fallback: self-hosted FrexSansGB, then PingFang SC / Microsoft YaHei / Noto Sans SC)
- Numbers / metrics / timestamps / code: **JetBrains Mono Variable**, tabular-nums, weight 700 for values
- Base 15px, line-height 1.62; kicker/labels: 0.66–0.72rem, 700–800 weight, 0.1–0.14em letter-spacing, uppercase

### Layout & structure

- Fixed 250px sidebar (sticky, full height) + sticky glass header (backdrop blur) + centered content column (max-width 1600px, padding 20px 24px 32px)
- Cards with 22px padding, 10px radius (lg 14px), soft cool shadows; sections separated by 20px gaps
- Pill language: status pills, metric pills, chips — radius 999px
- Status semantics: green LIVE (pulse), amber DEGRADED, red DOWN, violet loading/SYNC

### Motion

- Ease-out `cubic-bezier(0.22,1,0.36,1)` for entrances; spring `cubic-bezier(0.34,1.56,0.64,1)` for micro-interactions; smooth `cubic-bezier(0.4,0,0.2,1)` for hover states
- Durations: 120/200/320/420/500ms; nav active left-bar grows 300ms; live dot pulse 1.8s; page transitions fade+slide; theme switch soft-fade overlay 340ms; staggered card reveal on dashboard
- `prefers-reduced-motion: reduce` respected globally

## UX patterns

- **Density**: operations tool — tables with compact rows, mono numeric values, status tags; avoid oversized hero elements
- **Health communication**: always show key-pool counts (可用/冷却/失效) and provider count in header; provider-level tone (active/cooling/invalid/idle) drives row styling
- **Feedback**: optimistic updates on provider activation (rollback on error), message toasts, Popconfirm on destructive deletes
- **Empty/error states**: antd Result/Empty with retry actions; ErrorBoundary per page
- **Details-over-navigation**: provider details in a right Drawer; event details in a Drawer — keep list context
- **Danger**: destructive actions use red tone + Popconfirm confirmation

## Technical implementation notes

- Ant Design 5 (zh-CN locale) with tokens from `src/app/app-theme.ts`; CSS custom properties in `src/styles/base.css` (`--mm-*`) are the single source of truth
- Antd component overrides: violet focus rings on Input, `#EEF0FF` row hover / violet 10% dark, pill radius Tags, violet primary button glow
- All icons are inline SVG (stroke 1.7, round caps) — no icon library; brand tile is CSS gradient, not an image file
- Fonts: Inter/Space Grotesk/JetBrains Mono via @fontsource-variable + self-hosted FrexSansGB (GB2312 subset, 4 static weights)

## Fidelity constraints (for design iterations)

Use ONLY the fonts, colors, spacing, and component styles defined above. Do not introduce serif/decorative fonts, neon/rainbow palettes, or non-glass surface treatments. Keep the aurora violet identity and the observatory atmosphere. Chinese UI copy stays zh-CN.