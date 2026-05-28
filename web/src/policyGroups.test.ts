import assert from "node:assert/strict";
import { describe, it } from "node:test";
import { createPolicyGroup, policyGroupLine, policyGroupsText } from "./policyGroups.ts";

describe("policy group editor helpers", () => {
  it("does not render a default group when the structured list is empty", () => {
    assert.equal(policyGroupsText([]), "");
  });

  it("renders a smart group with quoted members and include-all-proxies disabled", () => {
    const group = createPolicyGroup("smart");
    group.name = "Proxy";
    group.members = [
      "美国A-线路1 | TCP",
      "美国A-线路2+|+TCP",
      "美国A-CMI专线1",
    ];
    group.includeAllProxies = false;

    assert.equal(
      policyGroupLine(group),
      'Proxy = smart, "美国A-线路1 | TCP", 美国A-线路2+|+TCP, 美国A-CMI专线1, include-all-proxies=0',
    );
  });
});
