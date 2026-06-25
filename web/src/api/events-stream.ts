import type { AdminEvent } from "../types/admin";

type EventStreamMessage =
  | { type: "connected"; last_seq: number }
  | AdminEvent;

type EventStreamOptions = {
  onEvent: (event: AdminEvent) => void;
  onConnected?: (lastSeq: number) => void;
  onError?: (error: Event) => void;
};

// createEventStream 创建 SSE 连接接收实时事件。
export function createEventStream(options: EventStreamOptions): () => void {
  const eventSource = new EventSource("/admin/api/v1/events/stream");

  eventSource.onmessage = (event: MessageEvent) => {
    try {
      const data = JSON.parse(event.data) as EventStreamMessage;
      if ("type" in data && data.type === "connected") {
        options.onConnected?.(data.last_seq);
      } else {
        options.onEvent(data as AdminEvent);
      }
    } catch {
      // 解析失败静默忽略
    }
  };

  eventSource.onerror = (error: Event) => {
    options.onError?.(error);
  };

  // 返回清理函数
  return () => {
    eventSource.close();
  };
}
