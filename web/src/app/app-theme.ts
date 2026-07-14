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
