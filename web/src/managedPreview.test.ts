import assert from "node:assert/strict";
import { describe, it } from "node:test";
import { enabledManagedHeaderPreviewFromValues, managedHeaderPreviewFromValues, type ManagedConfigDefaults, type ManagedPreviewValues } from "./managedPreview.ts";

describe("managed config preview", () => {
  it("uses the current form values instead of saved task values", () => {
    const defaults: ManagedConfigDefaults = {
      Enabled: false,
      URLMode: "task_subscription",
      CustomURL: "",
      IntervalSeconds: 86400,
      Strict: false,
    };
    const values: ManagedPreviewValues = {
      ManagedConfigMode: "enabled",
      ManagedConfigURLMode: "custom",
      ManagedConfigCustomURL: "https://profiles.example.com/live.conf",
      ManagedConfigIntervalMode: "custom",
      ManagedConfigIntervalSeconds: 3600,
      ManagedConfigStrictMode: "enabled",
    };

    assert.equal(
      managedHeaderPreviewFromValues(values, "/sub/task-1", defaults, "http://localhost:8080"),
      "#!MANAGED-CONFIG https://profiles.example.com/live.conf interval=3600 strict=true",
    );
  });

  it("can show an enabled preview while global mode is currently disabled", () => {
    const defaults: ManagedConfigDefaults = {
      Enabled: false,
      URLMode: "task_subscription",
      CustomURL: "",
      IntervalSeconds: 86400,
      Strict: false,
    };
    const values: ManagedPreviewValues = {
      ManagedConfigMode: "global",
      ManagedConfigURLMode: "custom",
      ManagedConfigCustomURL: "https://profiles.example.com/changed.conf",
      ManagedConfigIntervalMode: "custom",
      ManagedConfigIntervalSeconds: 7200,
      ManagedConfigStrictMode: "enabled",
    };

    assert.equal(managedHeaderPreviewFromValues(values, "/sub/task-1", defaults, "http://localhost:8080"), "MANAGED-CONFIG disabled");
    assert.equal(
      enabledManagedHeaderPreviewFromValues(values, "/sub/task-1", defaults, "http://localhost:8080"),
      "#!MANAGED-CONFIG https://profiles.example.com/changed.conf interval=7200 strict=true",
    );
  });

  it("uses the task subscription URL when legacy global URL mode is present", () => {
    const defaults: ManagedConfigDefaults = {
      Enabled: true,
      URLMode: "custom",
      CustomURL: "https://profiles.example.com/global.conf",
      IntervalSeconds: 86400,
      Strict: false,
    };
    const values: ManagedPreviewValues = {
      ManagedConfigMode: "enabled",
      ManagedConfigURLMode: "global",
      ManagedConfigCustomURL: "",
      ManagedConfigIntervalMode: "global",
      ManagedConfigIntervalSeconds: 86400,
      ManagedConfigStrictMode: "global",
    };

    assert.equal(
      managedHeaderPreviewFromValues(values, "/sub/task-1", defaults, "http://localhost:8080"),
      "#!MANAGED-CONFIG http://localhost:8080/sub/task-1 interval=86400 strict=false",
    );
  });
});
