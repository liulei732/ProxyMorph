export type PolicyGroupType = "select" | "url-test" | "fallback" | "load-balance" | "smart";

export type PolicyGroup = {
  id: string;
  name: string;
  type: PolicyGroupType;
  members: string[];
  includeAllProxies: boolean;
};

const policyGroupTypes: PolicyGroupType[] = ["select", "url-test", "fallback", "load-balance", "smart"];

export function createPolicyGroup(type: PolicyGroupType = "select"): PolicyGroup {
  return {
    id: `${type}-${Date.now()}-${Math.random().toString(36).slice(2, 8)}`,
    name: type === "smart" ? "Proxy" : "Manual",
    type,
    members: [],
    includeAllProxies: false,
  };
}

export function policyGroupLine(group: PolicyGroup) {
  const name = group.name.trim();
  if (!name) return "";

  const parts = [
    `${name} = ${group.type}`,
    ...group.members.map((member) => member.trim()).filter(Boolean).map(formatPolicyMember),
  ];
  if (group.type === "smart") {
    parts.push(`include-all-proxies=${group.includeAllProxies ? "1" : "0"}`);
  }
  return parts.join(", ");
}

export function policyGroupsText(groups: PolicyGroup[]) {
  return groups.map(policyGroupLine).filter(Boolean).join("\n");
}

export function parsePolicyGroupsText(text: string): PolicyGroup[] {
  return text
    .split(/\r?\n/)
    .map((line) => line.trim())
    .filter(Boolean)
    .map(parsePolicyGroupLine)
    .filter((group): group is PolicyGroup => Boolean(group));
}

export function membersFromText(text: string) {
  return text
    .split(/\r?\n|,/)
    .map((item) => item.trim())
    .filter(Boolean);
}

function parsePolicyGroupLine(line: string): PolicyGroup | null {
  const equalIndex = line.indexOf("=");
  if (equalIndex < 1) return null;

  const name = line.slice(0, equalIndex).trim();
  const segments = splitCommaList(line.slice(equalIndex + 1));
  const type = segments.shift()?.trim() as PolicyGroupType | undefined;
  if (!name || !type || !policyGroupTypes.includes(type)) return null;

  const group = createPolicyGroup(type);
  group.name = name;
  group.members = [];
  for (const segment of segments) {
    const value = segment.trim();
    if (type === "smart" && value.startsWith("include-all-proxies=")) {
      group.includeAllProxies = value.endsWith("1");
      continue;
    }
    group.members.push(unquotePolicyMember(value));
  }
  return group;
}

function formatPolicyMember(member: string) {
  if (!/[,\s"]/.test(member)) return member;
  return `"${member.replaceAll("\\", "\\\\").replaceAll('"', '\\"')}"`;
}

function unquotePolicyMember(member: string) {
  if (!member.startsWith('"') || !member.endsWith('"')) return member;
  return member.slice(1, -1).replaceAll('\\"', '"').replaceAll("\\\\", "\\");
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
