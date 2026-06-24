export type ModelSaveMode = "replace" | "merge";

export type ModelSyncSummary = {
  currentModels: string[];
  fetchedModels: string[];
  newModels: string[];
  existingModels: string[];
  removedModels: string[];
};

export function normalizeModelIDs(models: string[]): string[] {
  const seen = new Set<string>();
  const normalized: string[] = [];

  for (const model of models) {
    const trimmed = model.trim();
    if (!trimmed || seen.has(trimmed)) {
      continue;
    }
    seen.add(trimmed);
    normalized.push(trimmed);
  }

  return normalized.sort();
}

export function summarizeModelSync(currentModels: string[], fetchedModels: string[]): ModelSyncSummary {
  const current = normalizeModelIDs(currentModels);
  const fetched = normalizeModelIDs(fetchedModels);
  const currentSet = new Set(current);
  const fetchedSet = new Set(fetched);

  return {
    currentModels: current,
    fetchedModels: fetched,
    newModels: fetched.filter((model) => !currentSet.has(model)),
    existingModels: fetched.filter((model) => currentSet.has(model)),
    removedModels: current.filter((model) => !fetchedSet.has(model)),
  };
}

export function buildModelSaveList({
  mode,
  currentModels,
  selectedModels,
}: {
  mode: ModelSaveMode;
  currentModels: string[];
  selectedModels: string[];
}): string[] {
  if (mode === "merge") {
    return normalizeModelIDs([...currentModels, ...selectedModels]);
  }
  return normalizeModelIDs(selectedModels);
}
