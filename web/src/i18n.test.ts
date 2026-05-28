import assert from "node:assert/strict";
import { describe, it } from "node:test";
import { getInitialLanguage, languages, translations } from "./i18n.ts";

describe("i18n", () => {
  it("defaults to Chinese when no saved language exists", () => {
    assert.equal(getInitialLanguage(null), "zh");
  });

  it("accepts supported saved languages", () => {
    assert.equal(getInitialLanguage("en"), "en");
    assert.equal(getInitialLanguage("zh"), "zh");
  });

  it("falls back to Chinese for unsupported saved languages", () => {
    assert.equal(getInitialLanguage("ja"), "zh");
  });

  it("provides labels for the language switcher", () => {
    assert.deepEqual(languages.map((language) => language.label), ["中文", "EN"]);
  });

  it("translates dashboard titles", () => {
    assert.equal(translations.zh.tabs.overview, "概览");
    assert.equal(translations.en.tabs.overview, "Overview");
  });

  it("translates managed config controls", () => {
    assert.equal(translations.zh.forms.managedConfigMode, "MANAGED-CONFIG");
    assert.equal(translations.zh.managedConfigModes.global, "跟随全局");
    assert.equal(translations.en.managedConfigModes.global, "Follow global");
  });

  it("translates task editor sections", () => {
    assert.equal(translations.zh.taskEditor.sections.basic, "基础配置");
    assert.equal(translations.zh.taskEditor.sections.conversion, "转换配置");
    assert.equal(translations.zh.taskEditor.subsections.nodes, "节点配置");
    assert.equal(translations.zh.taskEditor.subsections.rules, "规则配置");
    assert.equal(translations.zh.taskEditor.subsections.vless, "VLESS 配置");
    assert.equal(translations.zh.forms.mergeDefaults, "合并固定节点");
    assert.equal(translations.zh.taskEditor.descriptions.mergeDefaults, "启用后，所有已启用的固定节点会合并到当前任务输出中。");
    assert.equal(translations.zh.forms.finalRulePolicy, "FINAL 策略");
    assert.equal(translations.zh.taskEditor.autoFinalPolicy, "自动");
    assert.equal(translations.en.taskEditor.sections.custom, "Custom Rules");
    assert.equal(translations.en.taskEditor.subsections.rules, "Rule Configuration");
    assert.equal(translations.en.forms.mergeDefaults, "Merge pinned nodes");
    assert.equal(translations.en.forms.finalRulePolicy, "FINAL policy");
  });
});
