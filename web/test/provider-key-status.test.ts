import assert from "node:assert/strict";
import test from "node:test";

import { isQuotaExhaustedKey } from "../src/features/providers/provider-key-status.ts";

test("isQuotaExhaustedKey only marks invalid quota-exhausted keys", () => {
  assert.equal(isQuotaExhaustedKey({ state: "invalid", invalid_reason: "quota_exhausted" }), true);
  assert.equal(isQuotaExhaustedKey({ state: "invalid", invalid_reason: "unauthorized" }), false);
  assert.equal(isQuotaExhaustedKey({ state: "cooling", invalid_reason: "quota_exhausted" }), false);
  assert.equal(isQuotaExhaustedKey({ state: "disabled", invalid_reason: "quota_exhausted" }), false);
});
