import assert from "node:assert/strict";
import { afterEach, describe, it } from "node:test";

import { copyTextToClipboard } from "./clipboard.ts";

const originalNavigator = Object.getOwnPropertyDescriptor(globalThis, "navigator");
const originalDocument = Object.getOwnPropertyDescriptor(globalThis, "document");

afterEach(() => {
  restoreGlobal("navigator", originalNavigator);
  restoreGlobal("document", originalDocument);
});

describe("copyTextToClipboard", () => {
  it("uses the Clipboard API when it is available", async () => {
    let copied = "";
    Object.defineProperty(globalThis, "navigator", {
      configurable: true,
      value: {
        clipboard: {
          writeText: async (text: string) => {
            copied = text;
          },
        },
      },
    });

    const ok = await copyTextToClipboard("https://example.test/sub/token");

    assert.equal(ok, true);
    assert.equal(copied, "https://example.test/sub/token");
  });

  it("falls back to textarea copy when navigator.clipboard is unavailable", async () => {
    let selected = "";
    let removed = false;
    Object.defineProperty(globalThis, "navigator", {
      configurable: true,
      value: {},
    });
    Object.defineProperty(globalThis, "document", {
      configurable: true,
      value: {
        body: {
          appendChild() {},
          removeChild() {
            removed = true;
          },
        },
        createElement() {
          return {
            value: "",
            style: {},
            setAttribute() {},
            select() {
              selected = this.value;
            },
          };
        },
        execCommand(command: string) {
          return command === "copy";
        },
      },
    });

    const ok = await copyTextToClipboard("fallback-value");

    assert.equal(ok, true);
    assert.equal(selected, "fallback-value");
    assert.equal(removed, true);
  });
});

function restoreGlobal(name: "navigator" | "document", descriptor: PropertyDescriptor | undefined) {
  if (descriptor) {
    Object.defineProperty(globalThis, name, descriptor);
    return;
  }
  delete (globalThis as Record<string, unknown>)[name];
}
