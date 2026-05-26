import React from "react";
import ReactDOM from "react-dom/client";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { ConfigProvider } from "antd";
import { BrowserRouter } from "react-router-dom";

import { App } from "./App";
import "./styles.css";

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      retry: false,
      refetchOnWindowFocus: false,
    },
  },
});

const rootElement = document.getElementById("root");
if (!rootElement) {
  throw new Error("missing #root element");
}

ReactDOM.createRoot(rootElement).render(
  <React.StrictMode>
    <ConfigProvider
      theme={{
        token: {
          colorPrimary: "#0f766e",
          colorInfo: "#0f766e",
          // 控件圆角分三级，避免单一 borderRadius 让按钮/输入变成胶囊。
          borderRadius: 10,
          borderRadiusLG: 20,
          borderRadiusSM: 6,
          borderRadiusXS: 4,
          // 按钮和输入再厚 4px，给"扎实"的感觉。
          controlHeight: 36,
          controlHeightSM: 28,
          controlHeightLG: 44,
          fontFamily: '"Aptos", "Segoe UI", "Microsoft YaHei UI", sans-serif',
        },
        components: {
          Button: {
            // 主按钮稍微更柔和的阴影，避免和卡片底色硬碰硬。
            primaryShadow: "0 6px 14px rgba(15, 118, 110, 0.18)",
          },
          Tag: {
            borderRadiusSM: 6,
          },
        },
      }}
    >
      <QueryClientProvider client={queryClient}>
        <BrowserRouter basename={normalizeBaseName(import.meta.env.BASE_URL)}>
          <App />
        </BrowserRouter>
      </QueryClientProvider>
    </ConfigProvider>
  </React.StrictMode>
);

// normalizeBaseName 把 Vite 的 BASE_URL 规范为 BrowserRouter 可以识别的 basename。
function normalizeBaseName(baseUrl: string): string {
  if (!baseUrl || baseUrl === "/") {
    return "/";
  }
  return baseUrl.endsWith("/") ? baseUrl.slice(0, -1) : baseUrl;
}
