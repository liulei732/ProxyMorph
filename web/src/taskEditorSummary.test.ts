import assert from "node:assert/strict";
import { describe, it } from "node:test";
import { taskEditorAsideMode, taskManagedURLModes } from "./taskEditorSummary.ts";

describe("task editor summary", () => {
  it("shows managed preview only on the managed tab", () => {
    assert.equal(taskEditorAsideMode("basic"), "basic");
    assert.equal(taskEditorAsideMode("conversion"), "conversion");
    assert.equal(taskEditorAsideMode("managed"), "managed");
    assert.equal(taskEditorAsideMode("custom"), "custom");
  });

  it("does not include global mode for task managed URL source", () => {
    assert.deepEqual(taskManagedURLModes(), ["task_subscription", "custom"]);
  });
});
