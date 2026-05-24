type APIErrorPayload = {
  error?: string;
};

// requestJSON 统一封装管理台 API 调用，并把错误尽量转换为可读消息。
export async function requestJSON<T>(path: string, init?: RequestInit): Promise<T> {
  const response = await fetch(path, {
    headers: {
      Accept: "application/json",
      ...init?.headers,
    },
    ...init,
  });

  if (!response.ok) {
    let message = `${response.status} ${response.statusText}`;
    try {
      const payload = (await response.json()) as APIErrorPayload;
      if (payload.error) {
        message = payload.error;
      }
    } catch {
      // 这里故意忽略解析失败，保留默认 HTTP 文本即可。
    }
    throw new Error(message);
  }

  return (await response.json()) as T;
}

type DownloadResult = {
  blob: Blob;
  filename: string;
};

// requestDownload 统一封装附件下载请求，并尝试从响应头中解析文件名。
export async function requestDownload(path: string, init?: RequestInit): Promise<DownloadResult> {
  const response = await fetch(path, init);

  if (!response.ok) {
    let message = `${response.status} ${response.statusText}`;
    try {
      const payload = (await response.json()) as { error?: string };
      if (payload.error) {
        message = payload.error;
      }
    } catch {
      // 这里故意忽略解析失败，保留默认状态描述。
    }
    throw new Error(message);
  }

  const disposition = response.headers.get("Content-Disposition") ?? "";
  const filenameMatch = disposition.match(/filename="([^"]+)"/i);
  return {
    blob: await response.blob(),
    filename: filenameMatch?.[1] ?? "download.json",
  };
}

// saveDownloadBlob 在浏览器里把后端返回的附件保存到本地。
export function saveDownloadBlob(blob: Blob, filename: string) {
  const url = window.URL.createObjectURL(blob);
  const anchor = document.createElement("a");
  anchor.href = url;
  anchor.download = filename;
  document.body.appendChild(anchor);
  anchor.click();
  anchor.remove();
  window.URL.revokeObjectURL(url);
}
