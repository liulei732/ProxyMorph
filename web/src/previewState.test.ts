import assert from "node:assert/strict";
import { describe, it } from "node:test";
import { previewRefreshTaskID } from "./previewState.ts";

describe("preview refresh state", () => {
  it("refreshes the visible preview after its task changes", () => {
    assert.equal(previewRefreshTaskID(7, { taskID: 7 }), 7);
  });

  it("keeps unrelated task saves from replacing the current preview", () => {
    assert.equal(previewRefreshTaskID(8, { taskID: 7 }), null);
  });

  it("refreshes the current task preview after any output-affecting shared config changes", () => {
    assert.equal(previewRefreshTaskID("current_preview", { taskID: 7 }), 7);
  });
});
