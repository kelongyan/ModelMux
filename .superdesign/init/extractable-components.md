# Extractable Components — ModelMux Aurora Console

Components that appear on every page or define shared UI patterns — candidates for Superdesign `DraftComponent` extraction.

## Layout Components

### ConsoleShell (sidebar + header + content shell)
- Source: `src/app/console-shell.tsx`
- Category: layout
- Description: Full app shell — 250px aurora-tinted sidebar (brand tile, nav links with active indicator bar, LOCAL PROXY chip footer) + sticky glass header + content column. Wraps every page.
- Extractable props: activeItem (string, default: "/dashboard") — active nav key; children (ReactNode)
- Hardcoded: Brand logo tile (aurora gradient + white core), nav icons (SVG, 6 items: 总览/提供商/调用统计/设置/事件/关于), "LOCAL PROXY" chip, "Aurora Console" meta, all CSS

### HeaderStatus (top status bar)
- Source: `src/app/header-status.tsx`
- Category: layout
- Description: Sticky top bar — LIVE/DEGRADED/DOWN/IDLE pill + HealthDot + active provider name + key-pool metric pills (可用/冷却/失效/Provider) + theme toggle.
- Extractable props: state ("active" | "cooling" | "invalid" | "idle"), liveLabel (string), activeProvider (string), activeKeys/coolingKeys/invalidKeys/providerCount (numbers), loading (boolean)
- Hardcoded: metric labels (可用/冷却/失效/Provider), "Active Provider" kicker, LIVE pill text, all CSS

### ConsoleBrand (sidebar brand block)
- Source: `src/app/console-shell.tsx` (ConsoleBrand function)
- Category: layout
- Description: Logo tile (40px aurora gradient, white core, glow) + "ModelMux / 控制台 / LOCAL PROXY · OPS" copy block.
- Extractable props: (none — fully static)
- Hardcoded: gradient tile, "ModelMux" kicker (aurora text clip), "控制台" title, "LOCAL PROXY · OPS" sub

### ThemeToggle (light/dark switch)
- Source: `src/app/theme-toggle.tsx`
- Category: basic
- Description: 34px circular glass button; sun icon in dark mode, moon icon in light mode; spring hover/active animations.
- Extractable props: isDark (boolean), onToggle (function)
- Hardcoded: sun/moon SVG icons, CSS

## Basic Components (used across pages)

### HealthDot
- Source: `src/components/health-dot.tsx`
- Category: basic
- Description: Small status dot with tone variants (active/cooling/invalid/idle) + optional pulse animation.
- Extractable props: state ("active" | "cooling" | "invalid" | "idle"), pulse (boolean, default false)
- Hardcoded: CSS (`.health-dot`, `.health-dot--{state}`, `.health-dot--pulse`)

### CooldownText
- Source: `src/components/cooldown-text.tsx`
- Category: basic
- Description: Live countdown text ("28s" / "3m12s") from ISO `cool_until` timestamp, updates every second; fallback "-" when expired.
- Extractable props: until (ISO string), fallback (string, default "-"), className
- Hardcoded: formatCooldown formatting logic

### ProviderRow (dashboard list row) / ProviderTable (providers page)
- Source: `src/pages/dashboard-page.tsx` (ProviderRow) / `src/features/providers/provider-table.tsx`
- Category: basic
- Description: Provider summary row/table — tone badge + name + target URL + key counts (可用/冷却/失效/停用/共) + actions (切换/详情/删除).
- Extractable props: provider id, targetUrl, activeKeys, coolingKeys, invalidKeys, disabledKeys, active (boolean), tone, actions callbacks
- Hardcoded: key count labels, badge labels (当前活跃/不可用/波动中/待命), all CSS

## Notes

- No external icon library — all icons are inline hand-drawn SVG (stroke 1.7, round caps) in `console-shell.tsx` and `theme-toggle.tsx`. Must be reproduced 1:1.
- No brand image assets — the only brand mark is the CSS-built aurora gradient tile (no logo file to extract).