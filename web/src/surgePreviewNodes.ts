export type ProxyNodeCategory = "regular" | "subscription_info";

export type ProxyNodeCandidate = {
  name: string;
  category: ProxyNodeCategory;
};

const subscriptionInfoPatterns = [
  /剩余流量/i,
  /流量/i,
  /重置/i,
  /套餐到期/i,
  /到期/i,
  /官网/i,
  /刷新订阅/i,
  /traffic/i,
  /expire/i,
  /reset/i,
];

export function parseProxyNodeCandidates(content: string): ProxyNodeCandidate[] {
  const result: ProxyNodeCandidate[] = [];
  let inProxySection = false;

  for (const rawLine of content.split(/\r?\n/)) {
    const line = rawLine.trim();
    if (!line) continue;
    if (line.startsWith("[") && line.endsWith("]")) {
      inProxySection = line.toLowerCase() === "[proxy]";
      continue;
    }
    if (!inProxySection) continue;

    const equalIndex = line.indexOf("=");
    if (equalIndex < 1) continue;

    const name = line.slice(0, equalIndex).trim();
    if (!name) continue;
    result.push({
      name,
      category: isSubscriptionInfoNode(name) ? "subscription_info" : "regular",
    });
  }

  return result;
}

function isSubscriptionInfoNode(name: string) {
  return subscriptionInfoPatterns.some((pattern) => pattern.test(name));
}
