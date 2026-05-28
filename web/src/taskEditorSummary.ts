import type { ManagedURLMode } from "./managedPreview.ts";

export type TaskEditorSection = "basic" | "conversion" | "managed" | "custom";
export type TaskEditorAsideMode = TaskEditorSection;

export function taskEditorAsideMode(section: TaskEditorSection): TaskEditorAsideMode {
  return section;
}

export function taskManagedURLModes(): Array<Exclude<ManagedURLMode, "global">> {
  return ["task_subscription", "custom"];
}
