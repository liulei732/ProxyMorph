export type PreviewState = {
  taskID: number | null;
};

export type PreviewChangeScope = number | "current_preview";

export function previewRefreshTaskID(changedTaskID: PreviewChangeScope, preview: PreviewState) {
  if (!preview.taskID) return null;
  if (changedTaskID === "current_preview") return preview.taskID;
  return changedTaskID === preview.taskID ? preview.taskID : null;
}
