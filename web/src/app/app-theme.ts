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
      colorBorder: dark ? "#2A3140" : "#E2E8F0",
      colorBorderSecondary: dark ? "#2A3140" : "#E2E8F0",
      colorBgBase: dark ? "#0B0D12" : "#F6F7FB",
      colorBgLayout: dark ? "#0B0D12" : "#F6F7FB",
      colorBgContainer: dark ? "#1E2430" : "#FFFFFF",

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
    },
    components: {
      Button: {
        primaryShadow: dark
          ? "0 4px 16px rgba(124, 108, 240, 0.45)"
          : "0 4px 16px rgba(106, 88, 224, 0.30)",
        paddingInline: 18,
      },
      Tag: {
        borderRadiusSM: 999,
        fontSizeSM: 13,
        lineHeightSM: 1.8,
      },
    },
  };
}
