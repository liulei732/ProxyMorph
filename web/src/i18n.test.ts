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
});
