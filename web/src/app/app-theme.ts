import { theme, type ThemeConfig } from "antd";

export type AppThemeMode = "light" | "dark";

export function createAppTheme(mode: AppThemeMode): ThemeConfig {
  const dark = mode === "dark";

  return {
    algorithm: dark ? theme.darkAlgorithm : theme.defaultAlgorithm,
    token: {
      /* === Anthropic Warm Color Palette === */
      colorPrimary: "#c86f55",
      colorInfo: "#c86f55",
      colorSuccess: "#5aab6a",
      colorWarning: "#d4a017",
      colorError: "#c44848",
      colorText: dark ? "#f0ece6" : "#141413",
      colorTextSecondary: dark ? "#d4cfc7" : "#363533",
      colorBorder: dark ? "#2a2926" : "#e4e0da",
      colorBorderSecondary: dark ? "#2a2926" : "#e4e0da",
      colorBgBase: dark ? "#161614" : "#fafaf8",
      colorBgLayout: dark ? "#161614" : "#fafaf8",
      colorBgContainer: dark ? "#222120" : "#ffffff",

      /* === Anthropic Rounded Corners === */
      borderRadius: 12,
      borderRadiusLG: 16,
      borderRadiusSM: 8,
      borderRadiusXS: 6,

      /* === Controls === */
      controlHeight: 42,
      controlHeightSM: 34,
      controlHeightLG: 48,

      /* === Typography === */
      fontSize: 15,
      fontSizeSM: 13,
      lineHeight: 1.62,
      padding: 18,
      paddingSM: 14,
      paddingXS: 10,
      margin: 18,
      marginSM: 14,
      marginXS: 10,
      fontFamily:
        '"Libre Baskerville", "Inter Variable", "PingFang SC", "Microsoft YaHei UI", "Noto Sans SC", system-ui, sans-serif',
      fontFamilyCode:
        '"Cascadia Code", "JetBrains Mono", "SF Mono", Menlo, Consolas, monospace',
    },
    components: {
      Button: {
        primaryShadow: dark
          ? "none"
          : "0 4px 16px rgba(200, 111, 85, 0.25)",
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
