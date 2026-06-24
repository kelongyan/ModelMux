import assert from "node:assert/strict";
import test from "node:test";

import { buildModelSaveList, summarizeModelSync } from "../src/features/providers/model-sync.ts";

test("summarizeModelSync normalizes model ids and reports fetched diff groups", () => {
  const summary = summarizeModelSync(
    ["legacy-model", "gpt-4o", " foo ", "gpt-4o"],
    ["gpt-4o-mini", "foo", "gpt-4o", "gpt-4o-mini", ""],
  );

  assert.deepEqual(summary.currentModels, ["foo", "gpt-4o", "legacy-model"]);
  assert.deepEqual(summary.fetchedModels, ["foo", "gpt-4o", "gpt-4o-mini"]);
  assert.deepEqual(summary.existingModels, ["foo", "gpt-4o"]);
  assert.deepEqual(summary.newModels, ["gpt-4o-mini"]);
  assert.deepEqual(summary.removedModels, ["legacy-model"]);
});

test("buildModelSaveList supports replace and merge save modes", () => {
  assert.deepEqual(
    buildModelSaveList({
      mode: "replace",
      currentModels: ["legacy-model", "gpt-4o"],
      selectedModels: ["gpt-4o-mini", "gpt-4o-mini", " gpt-4o "],
    }),
    ["gpt-4o", "gpt-4o-mini"],
  );

  assert.deepEqual(
    buildModelSaveList({
      mode: "merge",
      currentModels: ["legacy-model", "gpt-4o"],
      selectedModels: ["gpt-4o-mini", "gpt-4o"],
    }),
    ["gpt-4o", "gpt-4o-mini", "legacy-model"],
  );
});
