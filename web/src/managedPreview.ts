export type RuleMergeMode = "custom_first" | "upstream_first" | "custom_first_dedupe" | "upstream_first_dedupe";
export type VLESSRelayMode = "global" | "enabled" | "disabled";
export type TriStateMode = "global" | "enabled" | "disabled";
export type ManagedURLMode = "global" | "task_subscription" | "custom";
export type GlobalManagedURLMode = "task_subscription" | "custom";
export type ManagedIntervalMode = "global" | "custom";

export type ManagedConfigDefaults = {
  Enabled: boolean;
  URLMode: GlobalManagedURLMode;
  CustomURL: string;
  IntervalSeconds: number;
  Strict: boolean;
};

export type ManagedPreviewValues = {
  ManagedConfigMode: TriStateMode;
  ManagedConfigURLMode: ManagedURLMode;
  ManagedConfigCustomURL: string;
  ManagedConfigIntervalMode: ManagedIntervalMode;
  ManagedConfigIntervalSeconds: number;
  ManagedConfigStrictMode: TriStateMode;
};

export function managedHeaderPreviewFromValues(
  values: ManagedPreviewValues,
  subscriptionURL: string,
  defaults: ManagedConfigDefaults,
  origin: string,
) {
  const mode = values.ManagedConfigMode || "global";
  const enabled = mode === "enabled" || (mode === "global" && defaults.Enabled);
  if (!enabled) return "MANAGED-CONFIG disabled";
  return enabledManagedHeaderPreviewFromValues(values, subscriptionURL, defaults, origin);
}

export function enabledManagedHeaderPreviewFromValues(
  values: ManagedPreviewValues,
  subscriptionURL: string,
  defaults: ManagedConfigDefaults,
  origin: string,
) {
  const urlMode = values.ManagedConfigURLMode === "custom" ? "custom" : "task_subscription";
  const url = urlMode === "custom"
    ? (values.ManagedConfigCustomURL || defaults.CustomURL || "<custom-url>")
    : absoluteSubscriptionURL(subscriptionURL, origin);
  const interval = values.ManagedConfigIntervalMode === "custom"
    ? values.ManagedConfigIntervalSeconds
    : defaults.IntervalSeconds;
  const strict = values.ManagedConfigStrictMode === "enabled"
    || (values.ManagedConfigStrictMode === "global" && defaults.Strict);

  return `#!MANAGED-CONFIG ${url} interval=${interval || 86400} strict=${strict}`;
}

export function absoluteSubscriptionURL(url: string, origin: string) {
  if (url.startsWith("http://") || url.startsWith("https://")) {
    return url;
  }
  return `${origin}${url.startsWith("/") ? "" : "/"}${url}`;
}
