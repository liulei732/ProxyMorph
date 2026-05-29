import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import { describe, it } from "node:test";

const source = readFileSync(join(dirname(fileURLToPath(import.meta.url)), "main.tsx"), "utf8");
const styles = readFileSync(join(dirname(fileURLToPath(import.meta.url)), "styles.css"), "utf8");

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

  it("supports importing members from existing preview policy groups", () => {
    assert.match(source, /parseProxyGroupCandidates\(previewContent\)/);
    assert.match(source, /selectedImportGroup/);
    assert.match(source, /importGroupMembers\(targetGroup, selectedImportGroup.members, "append"\)/);
    assert.match(source, /importGroupMembers\(targetGroup, selectedImportGroup.members, "replace"\)/);
  });

  it("keeps policy member textarea text stable while editing", () => {
    assert.match(source, /memberDrafts/);
    assert.match(source, /handleMemberDraftChange\(group, event\.currentTarget\.value\)/);
    assert.doesNotMatch(source, /value=\{group\.members\.join\("\\n"\)\}/);
  });

  it("uses configurable subscription info keywords and cached preview for policy member selection", () => {
    assert.match(source, /SubscriptionInfoKeywordsText/);
    assert.match(source, /parseProxyNodeCandidates\(previewContent, subscriptionInfoKeywordsText\)/);
    assert.match(source, /loadCachedPreview\(task\.ID\)/);
    assert.match(source, /cached-preview/);
  });

  it("supports filtering and removing subscription info policy members", () => {
    assert.match(source, /memberCategoryFilter/);
    assert.match(source, /keywordFilter/);
    assert.match(source, /removeSubscriptionInfoMembers/);
    assert.match(source, /selectVisibleNodes/);
  });

  it("keeps policy member selector actions in their own dialog row", () => {
    assert.match(styles, /\.policy-member-dialog \{[^}]*grid-template-rows:\s*auto auto auto minmax\(0, 1fr\) auto/);
    assert.match(styles, /\.policy-member-actions \{[^}]*min-width:\s*0/);
    assert.match(styles, /\.policy-member-actions button \{[^}]*white-space:\s*normal/);
  });

  it("shows validation warnings while keeping generated preview content", () => {
    assert.match(source, /type TaskOutputResponse/);
    assert.match(source, /setPreviewContent\(result\.content\)/);
    assert.match(source, /result\.error[\s\S]*setValidationError\(result\.error\)/);
    assert.match(source, /ValidationErrorDialog/);
    assert.match(source, /formatValidationMessage/);
    assert.match(styles, /\.validation-error-dialog pre \{[^}]*white-space:\s*pre-wrap/);
  });

  it("does not report create or import success as failure when refresh steps fail", () => {
    assert.match(source, /notify\(t\.taskList\.created\)[\s\S]*try \{\s*await refresh\(\);[\s\S]*notify\(t\.refreshFailed\)/);
    assert.match(source, /notify\(t\.nodeList\.imported\)[\s\S]*try \{\s*await refresh\(\);[\s\S]*notify\(t\.refreshFailed\)/);
    assert.doesNotMatch(source, /await api\("\/api\/nodes\/import"[\s\S]*await refresh\(\)[\s\S]*notify\(t\.createFailed\)/);
  });

  it("supports deleting pinned nodes from the node list", () => {
    assert.match(source, /async function deleteNode\(id: number\)/);
    assert.match(source, /api\(`\/api\/nodes\/\$\{id\}`,\s*\{ method: "DELETE" \}\)/);
    assert.match(source, /<NodeManagement nodes=\{nodes\}[\s\S]*onDelete=\{deleteNode\}/);
    assert.match(source, /onClick=\{\(\) => onDelete\(node\.ID\)\}/);
  });

  it("supports batch node operations from the node list", () => {
    assert.match(source, /async function batchNodes\(action: NodeBatchAction, ids: number\[\]\)/);
    assert.match(source, /api\("\/api\/nodes\/batch",\s*\{ method: "POST", body: JSON\.stringify\(\{ action, ids \}\) \}\)/);
    assert.match(source, /<NodeManagement nodes=\{nodes\}[\s\S]*onBatch=\{batchNodes\}/);
    assert.match(source, /selectedNodeIDs/);
    assert.match(source, /toggleVisibleNodes/);
  });

  it("keeps the current tab after generating while manual preview still opens preview", () => {
    const generateStart = source.indexOf("async function generateTask");
    const loadPreviewStart = source.indexOf("async function loadPreview");
    const generateBlock = source.slice(generateStart, loadPreviewStart);
    const loadPreviewBlock = source.slice(loadPreviewStart, source.indexOf("async function previewDraft"));
    assert.doesNotMatch(generateBlock, /setTab\("preview"\)/);
    assert.match(generateBlock, /setPreviewContent\(result\.content\)/);
    assert.match(generateBlock, /setPreviewState\(\{ taskID: id \}\)/);
    assert.match(loadPreviewBlock, /setTab\("preview"\)/);
  });

  it("uses the redesigned pinned node management layout", () => {
    assert.match(source, /NodeManagement/);
    assert.match(source, /node-management/);
    assert.match(source, /node-import-preview/);
    assert.match(source, /node-library-tools/);
    assert.match(source, /node-table/);
    assert.match(source, /filteredNodes/);
    assert.match(source, /copyNodeName/);
    assert.match(source, /nodes\.filter\(\(node\) => node\.Enabled\)\.length/);
    assert.match(styles, /\.node-management \{/);
    assert.match(styles, /\.node-table \{/);
    assert.match(styles, /\.node-action-button/);
    assert.doesNotMatch(source, /className="row" key=\{node\.ID\}[\s\S]*danger-button/);
  });

  it("does not show the obsolete manual source column in pinned nodes", () => {
    const nodeListStart = source.indexOf("function NodeList");
    const nodeListBlock = source.slice(nodeListStart, source.indexOf("function nodeFilterLabel"));
    assert.doesNotMatch(nodeListBlock, /nodeList\.manual/);
    assert.doesNotMatch(styles, /30px minmax\(240px, 1\.4fr\) 0\.55fr 0\.45fr 0\.45fr auto/);
  });
});
