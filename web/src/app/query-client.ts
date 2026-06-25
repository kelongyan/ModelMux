import { QueryCache, QueryClient } from "@tanstack/react-query";

import { message } from "antd";

export const queryClient = new QueryClient({
  queryCache: new QueryCache({
    onError: (error) => {
      // 网络错误或服务器错误时显示全局提示
      if (error instanceof Error) {
        const msg = error.message;
        // 避免重复显示相同错误
        if (msg && !msg.includes("Failed to fetch")) {
          message.error(`请求失败：${msg}`);
        }
      }
    },
  }),
  defaultOptions: {
    queries: {
      retry: 2,
      retryDelay: (attemptIndex) => Math.min(1000 * 2 ** attemptIndex, 10000),
      refetchOnWindowFocus: false,
    },
    mutations: {
      retry: false,
    },
  },
});
