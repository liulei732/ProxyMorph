import assert from "node:assert/strict";
import { describe, it } from "node:test";
import { parseProxyNodeCandidates } from "./surgePreviewNodes.ts";

describe("Surge preview node candidates", () => {
  it("extracts proxy names from the Proxy section", () => {
    const nodes = parseProxyNodeCandidates(`
[Proxy]
美国A-线路1 | TCP = trojan, a.example.com, 443, password=secret
香港A-CMI专线1 = ss, h.example.com, 8388, encrypt-method=aes-256-gcm, password=secret

[Proxy Group]
Proxy = select, 美国A-线路1 | TCP, 香港A-CMI专线1
`);

    assert.deepEqual(nodes.map((node) => node.name), ["美国A-线路1 | TCP", "香港A-CMI专线1"]);
    assert.deepEqual(nodes.map((node) => node.category), ["regular", "regular"]);
  });

  it("keeps subscription info nodes selectable but marks them separately", () => {
    const nodes = parseProxyNodeCandidates(`
[Proxy]
剩余流量：33.69 GB = trojan, resolution1.example.com, 2096, password=secret
套餐到期：2027-04-10 = trojan, expire.example.com, 2096, password=secret
美国A-线路1 | TCP = trojan, a.example.com, 443, password=secret
`);

    assert.deepEqual(nodes, [
      { name: "剩余流量：33.69 GB", category: "subscription_info" },
      { name: "套餐到期：2027-04-10", category: "subscription_info" },
      { name: "美国A-线路1 | TCP", category: "regular" },
    ]);
  });
});
