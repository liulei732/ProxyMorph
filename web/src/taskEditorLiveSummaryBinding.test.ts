import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import { describe, it } from "node:test";

const source = readFileSync(join(dirname(fileURLToPath(import.meta.url)), "main.tsx"), "utf8");

describe("task editor live summary binding", () => {
  it("uses draft values for conversion summary while editing", () => {
    assert.match(source, /draft\.merge_default_pinned_nodes/);
    assert.match(source, /draft\.include_global_rules/);
    assert.match(source, /draft\.rule_merge_mode/);
    assert.match(source, /draft\.final_rule_policy/);
    assert.match(source, /draft\.vless_relay_mode/);
    assert.doesNotMatch(source, /\[t\.forms\.mergeDefaults,\s*task\.MergeDefaultPinnedNodes/);
    assert.doesNotMatch(source, /\[t\.forms\.includeGlobalRules,\s*task\.IncludeGlobalRules/);
    assert.doesNotMatch(source, /\[t\.forms\.ruleMergeMode,\s*t\.ruleMergeModes\[task\.RuleMergeMode\]/);
    assert.doesNotMatch(source, /\[t\.forms\.vlessRelayMode,\s*t\.vlessRelayModes\[task\.VLESSRelayMode/);
  });

  it("routes conversion field edits through draft preview updates", () => {
    assert.match(source, /name="merge_default_pinned_nodes"[\s\S]*updateDraft\("merge_default_pinned_nodes"/);
    assert.match(source, /name="include_global_rules"[\s\S]*updateDraft\("include_global_rules"/);
    assert.match(source, /name="rule_merge_mode"[\s\S]*updateDraft\("rule_merge_mode"/);
    assert.match(source, /name="final_rule_policy"[\s\S]*updateDraft\("final_rule_policy"/);
    assert.match(source, /name="vless_relay_mode"[\s\S]*updateDraft\("vless_relay_mode"/);
    assert.match(source, /void onPreviewDraft\(task\.ID, next\)/);
  });

  it("routes structured custom policy group edits through draft preview updates", () => {
    assert.match(source, /PolicyGroupEditor/);
    assert.match(source, /onChange=\{\(value\) => updateDraft\("custom_groups_text", value\)\}/);
    assert.match(source, /policyGroupsText\(nextGroups\)/);
  });

  it("does not create an undeletable default smart policy group for empty values", () => {
    assert.doesNotMatch(source, /parsedGroups\.length \? parsedGroups : \[createPolicyGroup\("smart"\)\]/);
  });

  it("lets batch member additions target a selected policy group instead of the first one only", () => {
    assert.match(source, /targetGroupID/);
    assert.match(source, /setTargetGroupID/);
    assert.match(source, /addBatchMembers\(targetGroup\)/);
    assert.doesNotMatch(source, /addToFirstGroup/);
  });
});
