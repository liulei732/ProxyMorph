export type Language = "zh" | "en";

export const languageStorageKey = "proxymorph_language";

export const languages: Array<{ code: Language; label: string }> = [
  { code: "zh", label: "中文" },
  { code: "en", label: "EN" },
];

export function getInitialLanguage(savedLanguage: string | null): Language {
  return savedLanguage === "en" || savedLanguage === "zh" ? savedLanguage : "zh";
}

export const translations = {
  zh: {
    productDescription: "转换 Clash 订阅，合并固定节点，生成 Surge 6 输出。",
    username: "用户名",
    password: "密码",
    signIn: "登录",
    loginFailed: "登录失败。",
    loadingSession: "正在检查登录状态...",
    createFailed: "创建任务失败。",
    refresh: "刷新",
    tabs: {
      overview: "概览",
      tasks: "转换任务",
      nodes: "固定节点",
      preview: "预览",
      settings: "设置",
    },
    subtitles: {
      overview: "查看转换运行状态。",
      tasks: "管理 Clash 到 Surge 6 的输出。",
      nodes: "导入固定的 SS、Trojan 和 VLESS 节点。",
      preview: "检查生成的订阅内容。",
      settings: "运行时和部署信息。",
    },
    metrics: {
      tasks: "任务",
      pinnedNodes: "固定节点",
      errors: "错误",
    },
    forms: {
      name: "名称",
      clashURL: "Clash 链接",
      refreshSeconds: "刷新秒数",
      proxyURIs: "代理链接",
      create: "创建",
      importNodes: "导入节点",
    },
    placeholders: {
      taskName: "主订阅",
      clashURL: "https://example.com/clash.yaml",
      proxyURIs: "vless://uuid@example.com:443?security=tls&sni=edge.example.com#Edge",
    },
    taskList: {
      title: "转换任务",
      empty: "还没有转换任务。",
      copy: "复制",
      enabled: "启用",
      disabled: "停用",
      failed: "失败",
      route: "到",
    },
    nodeList: {
      title: "固定节点库",
      empty: "还没有固定节点。",
      default: "默认",
      manual: "手动",
      enabled: "启用",
      disabled: "停用",
    },
    preview: {
      title: "转换预览",
      empty: "还没有加载预览。",
    },
    settings: {
      cachePolicy: "缓存策略",
      cachePolicyValue: "按需刷新，失败时使用上次成功结果",
      vlessHelper: "VLESS 辅助",
      vlessHelperValue: "Docker 镜像已包含 sing-box",
    },
  },
  en: {
    productDescription: "Convert Clash subscriptions, compose fixed nodes, ship Surge 6 output.",
    username: "Username",
    password: "Password",
    signIn: "Sign in",
    loginFailed: "Login failed.",
    loadingSession: "Checking session...",
    createFailed: "Create task failed.",
    refresh: "Refresh",
    tabs: {
      overview: "Overview",
      tasks: "Tasks",
      nodes: "Nodes",
      preview: "Preview",
      settings: "Settings",
    },
    subtitles: {
      overview: "Track conversion health.",
      tasks: "Manage Clash to Surge 6 outputs.",
      nodes: "Import fixed SS, Trojan, and VLESS nodes.",
      preview: "Inspect generated output.",
      settings: "Runtime and deployment details.",
    },
    metrics: {
      tasks: "Tasks",
      pinnedNodes: "Pinned Nodes",
      errors: "Errors",
    },
    forms: {
      name: "Name",
      clashURL: "Clash URL",
      refreshSeconds: "Refresh seconds",
      proxyURIs: "Proxy URIs",
      create: "Create",
      importNodes: "Import Nodes",
    },
    placeholders: {
      taskName: "Main subscription",
      clashURL: "https://example.com/clash.yaml",
      proxyURIs: "vless://uuid@example.com:443?security=tls&sni=edge.example.com#Edge",
    },
    taskList: {
      title: "Conversion Tasks",
      empty: "No conversion tasks yet.",
      copy: "Copy",
      enabled: "Enabled",
      disabled: "Disabled",
      failed: "Failed",
      route: "to",
    },
    nodeList: {
      title: "Pinned Node Library",
      empty: "No pinned nodes yet.",
      default: "Default",
      manual: "Manual",
      enabled: "Enabled",
      disabled: "Disabled",
    },
    preview: {
      title: "Conversion Preview",
      empty: "No preview loaded.",
    },
    settings: {
      cachePolicy: "Cache Policy",
      cachePolicyValue: "Refresh on request with last-good fallback",
      vlessHelper: "VLESS Helper",
      vlessHelperValue: "sing-box bundled in Docker",
    },
  },
} as const;
