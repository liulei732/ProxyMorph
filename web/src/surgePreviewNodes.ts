export type ProxyNodeCategory = "regular" | "subscription_info";

export type ProxyNodeCandidate = {
  name: string;
  category: ProxyNodeCategory;
};

export type ProxyGroupCandidate = {
  name: string;
  type: string;
  members: string[];
};

export const defaultSubscriptionInfoKeywordsText = "剩余流量\n流量\n重置\n套餐到期\n到期\n官网\n刷新订阅\ntraffic\nexpire\nreset";

export function parseProxyNodeCandidates(content: string, subscriptionInfoKeywordsText = defaultSubscriptionInfoKeywordsText): ProxyNodeCandidate[] {
  const result: ProxyNodeCandidate[] = [];
  let inProxySection = false;
  const subscriptionInfoPatterns = subscriptionInfoPatternsFromText(subscriptionInfoKeywordsText);

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
      category: isSubscriptionInfoNode(name, subscriptionInfoPatterns) ? "subscription_info" : "regular",
    });
  }

  return result;
}

export function parseProxyGroupCandidates(content: string): ProxyGroupCandidate[] {
  const result: ProxyGroupCandidate[] = [];
  let inProxyGroupSection = false;

  for (const rawLine of content.split(/\r?\n/)) {
    const line = rawLine.trim();
    if (!line) continue;
    if (line.startsWith("[") && line.endsWith("]")) {
      inProxyGroupSection = line.toLowerCase() === "[proxy group]";
      continue;
    }
    if (!inProxyGroupSection) continue;

    const equalIndex = line.indexOf("=");
    if (equalIndex < 1) continue;

    const name = line.slice(0, equalIndex).trim();
    const parts = splitCommaList(line.slice(equalIndex + 1));
    const type = parts.shift()?.trim() || "";
    if (!name || !type) continue;
    result.push({
      name,
      type,
      members: parts.map(unquoteValue).filter((member) => member && !isProxyGroupOption(member)),
    });
  }

  return result;
}

function isSubscriptionInfoNode(name: string, patterns: RegExp[]) {
  if (!patterns.length) return false;
  return patterns.some((pattern) => pattern.test(name));
}

function subscriptionInfoPatternsFromText(text: string) {
  const keywords = text.split(/\r?\n|,/).map((keyword) => keyword.trim()).filter(Boolean);
  const source = keywords.length ? keywords : defaultSubscriptionInfoKeywordsText.split(/\r?\n/);
  return source.map((keyword) => new RegExp(escapeRegExp(keyword), "i"));
}

function escapeRegExp(value: string) {
  return value.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
}

function splitCommaList(value: string) {
  const items: string[] = [];
  let current = "";
  let quoted = false;
  let escaped = false;

  for (const char of value) {
    if (escaped) {
      current += char;
      escaped = false;
      continue;
    }
    if (char === "\\") {
      current += char;
      escaped = true;
      continue;
    }
    if (char === '"') {
      current += char;
      quoted = !quoted;
      continue;
    }
    if (char === "," && !quoted) {
      items.push(current);
      current = "";
      continue;
    }
    current += char;
  }
  items.push(current);
  return items.map((item) => item.trim()).filter(Boolean);
}

function unquoteValue(value: string) {
  if (!value.startsWith('"') || !value.endsWith('"')) return value;
  return value.slice(1, -1).replaceAll('\\"', '"').replaceAll("\\\\", "\\");
}

function isProxyGroupOption(value: string) {
  return /^[A-Za-z][A-Za-z0-9-]*\s*=/.test(value);
}
