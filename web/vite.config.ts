import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

// viteConfig 根据管理台部署路径和本地 Go admin 地址配置开发与构建行为。
export default defineConfig({
  base: "/console/",
  plugins: [react()],
  server: {
    host: "127.0.0.1",
    port: 5173,
    proxy: {
      "/admin": {
        target: "http://127.0.0.1:18081",
        changeOrigin: false,
      },
    },
  },
  build: {
    outDir: "dist",
    emptyOutDir: true,
    rollupOptions: {
      output: {
        manualChunks(id) {
          if (!id.includes("node_modules")) {
            return undefined;
          }
          if (id.includes("antd") || id.includes("@ant-design") || id.includes("rc-")) {
            return "antd";
          }
          return "vendor";
        },
      },
    },
  },
});
