export type ProviderKeyStatusLike = {
  state: string;
  invalid_reason?: string;
};

export function isQuotaExhaustedKey(key: ProviderKeyStatusLike): boolean {
  return key.state === "invalid" && key.invalid_reason === "quota_exhausted";
}
