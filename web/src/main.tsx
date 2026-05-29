import React from "react";
import { createRoot } from "react-dom/client";
import { AlertCircle, Copy, Eye, KeyRound, Layers, LogOut, Pencil, Play, Plus, RefreshCw, Save, Search, Server, ShieldCheck, Trash2, UserRound, X } from "lucide-react";
import { api } from "./api";
import { copyTextToClipboard } from "./clipboard";
import { getInitialLanguage, languageStorageKey, languages, translations, type Language } from "./i18n";
import { absoluteSubscriptionURL as buildAbsoluteSubscriptionURL, enabledManagedHeaderPreviewFromValues, managedHeaderPreviewFromValues, type GlobalManagedURLMode, type ManagedConfigDefaults, type ManagedIntervalMode, type ManagedPreviewValues, type ManagedURLMode, type RuleMergeMode, type TriStateMode, type VLESSRelayMode } from "./managedPreview";
import { createPolicyGroup, membersFromText, parsePolicyGroupsText, policyGroupLine, policyGroupsText, type PolicyGroup, type PolicyGroupType } from "./policyGroups";
import { previewRefreshTaskID, type PreviewChangeScope, type PreviewState } from "./previewState";
import { defaultSubscriptionInfoKeywordsText, parseProxyGroupCandidates, parseProxyNodeCandidates, type ProxyNodeCandidate, type ProxyNodeCategory } from "./surgePreviewNodes";
import { taskEditorAsideMode, taskManagedURLModes, type TaskEditorSection } from "./taskEditorSummary";
import "./styles.css";

type Task = {
  ID: number;
  Name: string;
  InputType: string;
  OutputType: string;
  SourceURL: string;
  Enabled: boolean;
  RefreshIntervalSeconds: number;
  MergeDefaultPinnedNodes: boolean;
  SubscriptionURL: string;
  LastSuccessAt?: string | null;
  LastErrorAt?: string | null;
  LastErrorMessage: string;
  IncludeGlobalRules: boolean;
  CustomRulesText: string;
  RuleMergeMode: RuleMergeMode;
  FinalRulePolicy: string;
  CustomGroupsText: string;
  VLESSRelayMode: VLESSRelayMode;
  ManagedConfigMode: TriStateMode;
  ManagedConfigURLMode: ManagedURLMode;
  ManagedConfigCustomURL: string;
  ManagedConfigIntervalMode: ManagedIntervalMode;
  ManagedConfigIntervalSeconds: number;
  ManagedConfigStrictMode: TriStateMode;
};

type RuleConfig = {
  CustomRulesText: string;
  VLESSRelayEnabled: boolean;
  SubscriptionInfoKeywordsText: string;
};

type PinnedNode = {
  ID: number;
  Name: string;
  Protocol: string;
  Server: string;
  Port: number;
  Enabled: boolean;
  DefaultInclude: boolean;
};

type CurrentUser = {
  id: number;
  username: string;
};

type TaskOutputResponse = {
  content: string;
  error?: string;
};

type NodeBatchAction = "enable" | "disable" | "delete";

function App() {
  const [loggedIn, setLoggedIn] = React.useState(false);
  const [currentUser, setCurrentUser] = React.useState<CurrentUser | null>(null);
  const [checkingSession, setCheckingSession] = React.useState(true);
  const [showPasswordDialog, setShowPasswordDialog] = React.useState(false);
  const [tab, setTab] = React.useState("overview");
  const [language, setLanguage] = React.useState<Language>(() => getInitialLanguage(localStorage.getItem(languageStorageKey)));
  const [tasks, setTasks] = React.useState<Task[]>([]);
  const [nodes, setNodes] = React.useState<PinnedNode[]>([]);
  const [ruleConfig, setRuleConfig] = React.useState<RuleConfig>({ CustomRulesText: "", VLESSRelayEnabled: false, SubscriptionInfoKeywordsText: defaultSubscriptionInfoKeywordsText });
  const [managedConfigDefaults, setManagedConfigDefaults] = React.useState<ManagedConfigDefaults>(defaultManagedConfigDefaults);
  const [error, setError] = React.useState("");
  const [toast, setToast] = React.useState("");
  const [busyTaskID, setBusyTaskID] = React.useState<number | null>(null);
  const [previewContent, setPreviewContent] = React.useState("");
  const [previewState, setPreviewState] = React.useState<PreviewState>({ taskID: null });
  const [validationError, setValidationError] = React.useState("");
  const t = translations[language];

  function changeLanguage(nextLanguage: Language) {
    setLanguage(nextLanguage);
    localStorage.setItem(languageStorageKey, nextLanguage);
  }

  React.useEffect(() => {
    let cancelled = false;
    api<CurrentUser>("/api/me")
      .then(async (user) => {
        if (cancelled) {
          return;
        }
        setCurrentUser(user);
        setLoggedIn(true);
        await refresh();
      })
      .catch(() => {
        if (!cancelled) {
          setCurrentUser(null);
          setLoggedIn(false);
        }
      })
      .finally(() => {
        if (!cancelled) {
          setCheckingSession(false);
        }
      });
    return () => {
      cancelled = true;
    };
  }, []);

  function notify(message: string) {
    setToast(message);
    window.setTimeout(() => setToast(""), 2600);
  }

  async function refresh(showToast = false) {
    const [nextTasks, nextNodes, nextRuleConfig, nextManagedConfigDefaults] = await Promise.all([
      api<Task[]>("/api/tasks").catch(() => []),
      api<PinnedNode[]>("/api/nodes").catch(() => []),
      api<RuleConfig>("/api/rule-config").catch(() => ({ CustomRulesText: "", VLESSRelayEnabled: false, SubscriptionInfoKeywordsText: defaultSubscriptionInfoKeywordsText })),
      api<ManagedConfigDefaults>("/api/managed-config-defaults").catch(() => defaultManagedConfigDefaults),
    ]);
    setTasks(nextTasks);
    setNodes(nextNodes);
    setRuleConfig(nextRuleConfig);
    setManagedConfigDefaults(nextManagedConfigDefaults);
    if (showToast) notify(t.refreshed);
  }

  async function refreshVisiblePreview(changedTaskID: PreviewChangeScope) {
    const taskID = previewRefreshTaskID(changedTaskID, previewState);
    if (!taskID) return;
    try {
      const result = await api<TaskOutputResponse>(`/api/tasks/${taskID}/preview`);
      setPreviewContent(result.content);
      if (result.error) setValidationError(result.error);
    } catch {
      notify(t.preview.loadFailed);
    }
  }

  async function loadCachedPreview(id: number) {
    try {
      const result = await api<{ content: string }>(`/api/tasks/${id}/cached-preview`);
      setPreviewContent(result.content);
      setPreviewState({ taskID: id });
      return result.content;
    } catch {
      return "";
    }
  }

  async function login(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setError("");
    const form = new FormData(event.currentTarget);
    try {
      await api("/api/login", { method: "POST", body: JSON.stringify(Object.fromEntries(form)) });
      const user = await api<CurrentUser>("/api/me");
      setCurrentUser(user);
      setLoggedIn(true);
      await refresh();
    } catch {
      setError(t.loginFailed);
    }
  }

  async function logout() {
    try {
      await api("/api/logout", { method: "POST" });
    } catch {
      // Local state still needs to be cleared if the server is already unreachable.
    }
    setLoggedIn(false);
    setCurrentUser(null);
    setTasks([]);
    setNodes([]);
    setPreviewContent("");
    setPreviewState({ taskID: null });
    setValidationError("");
  }

  async function changePassword(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const formElement = event.currentTarget;
    const form = new FormData(formElement);
    const currentPassword = String(form.get("current_password") || "");
    const newPassword = String(form.get("new_password") || "");
    const confirmPassword = String(form.get("confirm_password") || "");
    if (newPassword !== confirmPassword) {
      notify(t.account.passwordMismatch);
      return;
    }
    try {
      await api("/api/me/password", { method: "PATCH", body: JSON.stringify({ current_password: currentPassword, new_password: newPassword }) });
    } catch {
      notify(t.account.passwordChangeFailed);
      return;
    }
    formElement.reset();
    setShowPasswordDialog(false);
    notify(t.account.passwordChanged);
  }

  async function createTask(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setError("");
    const form = new FormData(event.currentTarget);
    const payload = Object.fromEntries(form);
    payload.refresh_interval_seconds = Number(payload.refresh_interval_seconds || 3600) as unknown as FormDataEntryValue;
    try {
      await api("/api/tasks", { method: "POST", body: JSON.stringify(payload) });
    } catch {
      setError(t.createFailed);
      notify(t.createFailed);
      return;
    }
    event.currentTarget.reset();
    notify(t.taskList.created);
    try {
      await refresh();
    } catch {
      notify(t.refreshFailed);
    }
  }

  async function importNodes(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const formElement = event.currentTarget;
    try {
      const payload = Object.fromEntries(new FormData(event.currentTarget));
      await api("/api/nodes/import", { method: "POST", body: JSON.stringify(payload) });
    } catch {
      notify(t.nodeList.importFailed);
      return;
    }
    formElement.reset();
    notify(t.nodeList.imported);
    try {
      await refresh();
    } catch {
      notify(t.refreshFailed);
    }
    try {
      await refreshVisiblePreview("current_preview");
    } catch {
      notify(t.preview.loadFailed);
    }
  }

  async function deleteNode(id: number) {
    if (!window.confirm(t.nodeList.confirmDelete)) return;
    try {
      await api(`/api/nodes/${id}`, { method: "DELETE" });
    } catch {
      notify(t.nodeList.deleteFailed);
      return;
    }
    notify(t.nodeList.deleted);
    try {
      await refresh();
    } catch {
      notify(t.refreshFailed);
    }
    try {
      await refreshVisiblePreview("current_preview");
    } catch {
      notify(t.preview.loadFailed);
    }
  }

  async function batchNodes(action: NodeBatchAction, ids: number[]) {
    if (ids.length === 0) return;
    if (action === "delete" && !window.confirm(t.nodeList.confirmBatchDelete.replace("{count}", String(ids.length)))) return;
    try {
      await api("/api/nodes/batch", { method: "POST", body: JSON.stringify({ action, ids }) });
    } catch {
      notify(t.nodeList.batchFailed);
      return;
    }
    notify(t.nodeList.batchUpdated.replace("{count}", String(ids.length)));
    try {
      await refresh();
    } catch {
      notify(t.refreshFailed);
    }
    try {
      await refreshVisiblePreview("current_preview");
    } catch {
      notify(t.preview.loadFailed);
    }
  }

  async function updateTask(id: number, input: TaskUpdateInput) {
    setBusyTaskID(id);
    try {
      await api(`/api/tasks/${id}`, { method: "PATCH", body: JSON.stringify(input) });
      await refresh();
      await refreshVisiblePreview(id);
      notify(t.taskList.saved);
    } catch {
      notify(t.taskList.saveFailed);
    } finally {
      setBusyTaskID(null);
    }
  }

  async function updateRuleConfig(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const form = new FormData(event.currentTarget);
    try {
      const next = await api<RuleConfig>("/api/rule-config", {
        method: "PUT",
        body: JSON.stringify({
          custom_rules_text: String(form.get("custom_rules_text") || ""),
          vless_relay_enabled: form.get("vless_relay_enabled") === "on",
          subscription_info_keywords_text: String(form.get("subscription_info_keywords_text") || ""),
        }),
      });
      setRuleConfig(next);
      await refreshVisiblePreview("current_preview");
      notify(t.settings.ruleConfigSaved);
    } catch {
      notify(t.settings.ruleConfigSaveFailed);
    }
  }

  async function updateManagedConfigDefaults(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const form = new FormData(event.currentTarget);
    const preset = String(form.get("interval_preset") || "86400");
    const interval = preset === "custom" ? Number(form.get("interval_seconds") || 86400) : Number(preset);
    try {
      const next = await api<ManagedConfigDefaults>("/api/managed-config-defaults", {
        method: "PUT",
        body: JSON.stringify({
          enabled: form.get("enabled") === "on",
          url_mode: String(form.get("url_mode") || "task_subscription"),
          custom_url: String(form.get("custom_url") || ""),
          interval_seconds: interval,
          strict: form.get("strict") === "on",
        }),
      });
      setManagedConfigDefaults(next);
      await refreshVisiblePreview("current_preview");
      notify(t.settings.managedConfigSaved);
    } catch {
      notify(t.settings.managedConfigSaveFailed);
    }
  }

  async function deleteTask(id: number) {
    if (!window.confirm(t.confirmDelete)) return;
    setBusyTaskID(id);
    try {
      await api(`/api/tasks/${id}`, { method: "DELETE" });
      await refresh();
      await refreshVisiblePreview("current_preview");
      notify(t.taskList.deleted);
    } catch {
      notify(t.taskList.deleteFailed);
    } finally {
      setBusyTaskID(null);
    }
  }

  async function generateTask(id: number) {
    setBusyTaskID(id);
    try {
      const result = await api<TaskOutputResponse>(`/api/tasks/${id}/generate`, { method: "POST" });
      setPreviewContent(result.content);
      setPreviewState({ taskID: id });
      await refresh();
      if (result.error) {
        setValidationError(result.error);
        notify(t.taskList.generatedWithWarnings);
      } else {
        notify(t.taskList.generated);
      }
    } catch {
      await refresh();
      notify(t.taskList.generateFailed);
    } finally {
      setBusyTaskID(null);
    }
  }

  async function loadPreview(id: number) {
    setBusyTaskID(id);
    try {
      const result = await api<TaskOutputResponse>(`/api/tasks/${id}/preview`);
      setPreviewContent(result.content);
      setPreviewState({ taskID: id });
      setTab("preview");
      await refresh();
      if (result.error) {
        setValidationError(result.error);
        notify(t.preview.loadedWithWarnings);
      } else {
        notify(t.preview.loaded);
      }
    } catch {
      notify(t.preview.loadFailed);
    } finally {
      setBusyTaskID(null);
    }
  }

  async function previewDraft(id: number, input: TaskUpdateInput) {
    const taskID = previewRefreshTaskID(id, previewState);
    if (!taskID) return;
    try {
      const result = await api<TaskOutputResponse>(`/api/tasks/${taskID}/preview`, { method: "POST", body: JSON.stringify(input) });
      setPreviewContent(result.content);
      if (result.error) setValidationError(result.error);
    } catch {
      notify(t.preview.loadFailed);
    }
  }

  if (checkingSession) {
    return (
      <main className="login">
        <section className="identity">
          <div className="logo">PM</div>
          <div>
            <h1>ProxyMorph</h1>
            <p>{t.loadingSession}</p>
          </div>
        </section>
      </main>
    );
  }

  if (!loggedIn) {
    return (
      <main className="login">
        <LanguageSwitch language={language} onChange={changeLanguage} />
        <section className="identity">
          <div className="logo">PM</div>
          <div>
            <h1>ProxyMorph</h1>
            <p>{t.productDescription}</p>
          </div>
        </section>
        <form className="panel login-panel" onSubmit={login}>
          <label>{t.username}<input name="username" defaultValue="admin" autoComplete="username" /></label>
          <label>{t.password}<input name="password" type="password" autoComplete="current-password" /></label>
          <button><KeyRound size={17} /> {t.signIn}</button>
          {error && <p className="error">{error}</p>}
        </form>
      </main>
    );
  }

  const errors = tasks.filter((task) => task.LastErrorMessage);

  return (
    <main className="shell">
      <aside className="sidebar">
        <div className="identity small"><div className="logo">PM</div><strong>ProxyMorph</strong></div>
        <nav>
          {["overview", "tasks", "nodes", "preview", "settings"].map((item) => (
            <button key={item} className={tab === item ? "active" : ""} onClick={() => setTab(item)}>{t.tabs[item as keyof typeof t.tabs]}</button>
          ))}
        </nav>
      </aside>
        <section className="workspace">
        <header className="topbar">
          <div>
            <h2>{t.tabs[tab as keyof typeof t.tabs]}</h2>
            <p>{t.subtitles[tab as keyof typeof t.subtitles]}</p>
          </div>
          <div className="top-actions">
            <LanguageSwitch language={language} onChange={changeLanguage} />
            <AccountMenu user={currentUser} t={t} onChangePassword={() => setShowPasswordDialog(true)} onLogout={logout} />
            <button onClick={() => refresh(true)}><RefreshCw size={16} /> {t.refresh}</button>
          </div>
        </header>
          {toast && <div className="toast">{toast}</div>}
          {validationError && <ValidationErrorDialog message={validationError} title={t.validationError.title} onClose={() => setValidationError("")} closeLabel={t.forms.cancel} />}
          {showPasswordDialog && <ChangePasswordDialog t={t} onSubmit={changePassword} onClose={() => setShowPasswordDialog(false)} />}

        {tab === "overview" && (
          <section className="stack">
            <div className="metrics">
              <Metric icon={<Server />} value={tasks.length} label={t.metrics.tasks} />
              <Metric icon={<ShieldCheck />} value={nodes.length} label={t.metrics.pinnedNodes} />
              <Metric icon={<Layers />} value={tasks.filter((task) => task.IncludeGlobalRules).length} label={t.metrics.globalRules} />
              <Metric icon={<AlertCircle />} value={errors.length} label={t.metrics.errors} />
            </div>
            <TaskList tasks={tasks} t={t} managedConfigDefaults={managedConfigDefaults} previewContent={previewContent} subscriptionInfoKeywordsText={ruleConfig.SubscriptionInfoKeywordsText} busyTaskID={busyTaskID} onCopy={notify} onUpdate={updateTask} onDelete={deleteTask} onGenerate={generateTask} onPreview={loadPreview} onPreviewDraft={previewDraft} onLoadCachedPreview={loadCachedPreview} />
          </section>
        )}

        {tab === "tasks" && (
          <section className="stack">
            <form className="panel grid-form" onSubmit={createTask}>
              <label>{t.forms.name}<input name="name" placeholder={t.placeholders.taskName} /></label>
              <label>{t.forms.clashURL}<input name="source_url" placeholder={t.placeholders.clashURL} /></label>
              <label>{t.forms.refreshSeconds}<input name="refresh_interval_seconds" type="number" defaultValue="3600" /></label>
              <button><Plus size={16} /> {t.forms.create}</button>
            </form>
            {error && <p className="error">{error}</p>}
            <TaskList tasks={tasks} t={t} managedConfigDefaults={managedConfigDefaults} previewContent={previewContent} subscriptionInfoKeywordsText={ruleConfig.SubscriptionInfoKeywordsText} busyTaskID={busyTaskID} onCopy={notify} onUpdate={updateTask} onDelete={deleteTask} onGenerate={generateTask} onPreview={loadPreview} onPreviewDraft={previewDraft} onLoadCachedPreview={loadCachedPreview} />
          </section>
        )}

        {tab === "nodes" && (
              <NodeManagement nodes={nodes} t={t} placeholder={t.placeholders.proxyURIs} onImport={importNodes} onDelete={deleteNode} onBatch={batchNodes} onCopy={notify} importToast={toast} />
        )}

        {tab === "preview" && <section className="panel"><h3>{t.preview.title}</h3><pre>{previewContent || t.preview.empty}</pre></section>}
        {tab === "settings" && (
          <section className="stack">
            <form className="panel settings-form" onSubmit={updateRuleConfig}>
              <h3>{t.settings.globalRuleTitle}</h3>
              <label className="check-label"><input name="vless_relay_enabled" type="checkbox" defaultChecked={ruleConfig.VLESSRelayEnabled} /> {t.settings.vlessRelayToggle}</label>
              <label className="wide-field">{t.forms.customRules}<textarea name="custom_rules_text" rows={8} defaultValue={ruleConfig.CustomRulesText} placeholder={t.placeholders.customRules} /></label>
              <label className="wide-field">{t.forms.subscriptionInfoKeywords}<textarea name="subscription_info_keywords_text" rows={5} defaultValue={ruleConfig.SubscriptionInfoKeywordsText} placeholder={defaultSubscriptionInfoKeywordsText} /></label>
              <button><Save size={15} /> {t.forms.save}</button>
            </form>
            <form className="panel settings-form managed-defaults-form" onSubmit={updateManagedConfigDefaults}>
              <h3>{t.forms.managedConfigDefaults}</h3>
              <label className="check-label"><input name="enabled" type="checkbox" defaultChecked={managedConfigDefaults.Enabled} /> {t.forms.enabled}</label>
              <label>{t.forms.managedConfigURLMode}<select name="url_mode" defaultValue={managedConfigDefaults.URLMode}>{globalManagedURLModeOptions(t)}</select></label>
              <label>{t.forms.managedConfigCustomURL}<input name="custom_url" defaultValue={managedConfigDefaults.CustomURL} placeholder="https://profiles.example.com/default.conf" /></label>
              <label>{t.forms.managedConfigIntervalMode}<select name="interval_preset" defaultValue={intervalPresetValue(managedConfigDefaults.IntervalSeconds)}>{managedIntervalPresetOptions(t)}</select></label>
              <label>{t.forms.managedConfigIntervalCustom}<input name="interval_seconds" type="number" min="60" defaultValue={managedConfigDefaults.IntervalSeconds || 86400} /></label>
              <label className="check-label"><input name="strict" type="checkbox" defaultChecked={managedConfigDefaults.Strict} /> {t.forms.managedConfigStrictMode}</label>
              <p className="wide-field muted">{t.settings.managedStrictHelp}</p>
              <button><Save size={15} /> {t.forms.save}</button>
            </form>
            <section className="panel settings-info">
              <h3>{t.settings.runtimeInfo}</h3>
              <div className="info-list">
                <div className="info-row"><strong>{t.settings.cachePolicy}</strong><span>{t.settings.cachePolicyValue}</span></div>
              </div>
            </section>
          </section>
        )}
      </section>
    </main>
  );
}

type TaskUpdateInput = {
  name?: string;
  source_url?: string;
  refresh_interval_seconds?: number;
  enabled?: boolean;
  merge_default_pinned_nodes?: boolean;
  include_global_rules?: boolean;
  custom_rules_text?: string;
  rule_merge_mode?: RuleMergeMode;
  final_rule_policy?: string;
  custom_groups_text?: string;
  vless_relay_mode?: VLESSRelayMode;
  managed_config_mode?: TriStateMode;
  managed_config_url_mode?: ManagedURLMode;
  managed_config_custom_url?: string;
  managed_config_interval_mode?: ManagedIntervalMode;
  managed_config_interval_seconds?: number;
  managed_config_strict_mode?: TriStateMode;
};

function LanguageSwitch({ language, onChange }: { language: Language; onChange: (language: Language) => void }) {
  return (
    <div className="language-switch" aria-label="Language">
      {languages.map((option) => (
        <button key={option.code} type="button" className={language === option.code ? "active" : ""} onClick={() => onChange(option.code)}>
          {option.label}
        </button>
      ))}
    </div>
  );
}

function AccountMenu({ user, t, onChangePassword, onLogout }: { user: CurrentUser | null; t: typeof translations[Language]; onChangePassword: () => void; onLogout: () => void }) {
  return (
    <div className="account-menu">
      <span className="account-name"><UserRound size={15} /> {user?.username || t.account.unknownUser}</span>
      <button type="button" className="secondary-button" onClick={onChangePassword}><KeyRound size={15} /> {t.account.changePassword}</button>
      <button type="button" className="secondary-button" onClick={onLogout}><LogOut size={15} /> {t.account.logout}</button>
    </div>
  );
}

function ChangePasswordDialog({ t, onSubmit, onClose }: { t: typeof translations[Language]; onSubmit: (event: React.FormEvent<HTMLFormElement>) => void; onClose: () => void }) {
  return (
    <div className="account-dialog-overlay" role="dialog" aria-modal="true" aria-label={t.account.changePassword}>
      <form className="account-dialog" onSubmit={onSubmit}>
        <header>
          <h3>{t.account.changePassword}</h3>
          <button type="button" className="secondary-button icon-button" onClick={onClose} aria-label={t.forms.cancel}><X size={15} /></button>
        </header>
        <label>{t.account.currentPassword}<input name="current_password" type="password" autoComplete="current-password" /></label>
        <label>{t.account.newPassword}<input name="new_password" type="password" autoComplete="new-password" /></label>
        <label>{t.account.confirmPassword}<input name="confirm_password" type="password" autoComplete="new-password" /></label>
        <footer>
          <button type="button" className="secondary-button" onClick={onClose}>{t.forms.cancel}</button>
          <button><Save size={15} /> {t.forms.save}</button>
        </footer>
      </form>
    </div>
  );
}

function ValidationErrorDialog({ title, message, closeLabel, onClose }: { title: string; message: string; closeLabel: string; onClose: () => void }) {
  return (
    <div className="validation-error-overlay" role="dialog" aria-modal="true" aria-label={title}>
      <section className="validation-error-dialog">
        <header>
          <h3>{title}</h3>
          <button type="button" className="secondary-button icon-button" onClick={onClose} aria-label={closeLabel}><X size={15} /></button>
        </header>
        <pre>{formatValidationMessage(message)}</pre>
        <footer>
          <button type="button" onClick={onClose}>{closeLabel}</button>
        </footer>
      </section>
    </div>
  );
}

function formatValidationMessage(message: string) {
  return message.replaceAll("。", "。\n").trim();
}

function Metric({ icon, value, label }: { icon: React.ReactNode; value: number; label: string }) {
  return (
    <div className="metric">
      {icon}
      <span>{value}</span>
      <label>{label}</label>
    </div>
  );
}

function TaskList({
  tasks,
  t,
  managedConfigDefaults,
  previewContent,
  subscriptionInfoKeywordsText,
  busyTaskID,
  onCopy,
  onUpdate,
  onDelete,
  onGenerate,
  onPreview,
  onPreviewDraft,
  onLoadCachedPreview,
}: {
  tasks: Task[];
  t: typeof translations[Language];
  managedConfigDefaults: ManagedConfigDefaults;
  previewContent: string;
  subscriptionInfoKeywordsText: string;
  busyTaskID: number | null;
  onCopy: (message: string) => void;
  onUpdate: (id: number, input: TaskUpdateInput) => Promise<void>;
  onDelete: (id: number) => Promise<void>;
  onGenerate: (id: number) => Promise<void>;
  onPreview: (id: number) => Promise<void>;
  onPreviewDraft: (id: number, input: TaskUpdateInput) => Promise<void>;
  onLoadCachedPreview: (id: number) => Promise<string>;
}) {
  return (
    <section className="panel">
      <h3>{t.taskList.title}</h3>
      <div className="list">
        {tasks.length ? tasks.map((task) => (
          <TaskRow key={task.ID} task={task} t={t} managedConfigDefaults={managedConfigDefaults} previewContent={previewContent} subscriptionInfoKeywordsText={subscriptionInfoKeywordsText} busy={busyTaskID === task.ID} onCopy={onCopy} onUpdate={onUpdate} onDelete={onDelete} onGenerate={onGenerate} onPreview={onPreview} onPreviewDraft={onPreviewDraft} onLoadCachedPreview={onLoadCachedPreview} />
        )) : <p className="muted">{t.taskList.empty}</p>}
      </div>
    </section>
  );
}

function TaskRow({
  task,
  t,
  managedConfigDefaults,
  previewContent,
  subscriptionInfoKeywordsText,
  busy,
  onCopy,
  onUpdate,
  onDelete,
  onGenerate,
  onPreview,
  onPreviewDraft,
  onLoadCachedPreview,
}: {
  task: Task;
  t: typeof translations[Language];
  managedConfigDefaults: ManagedConfigDefaults;
  previewContent: string;
  subscriptionInfoKeywordsText: string;
  busy: boolean;
  onCopy: (message: string) => void;
  onUpdate: (id: number, input: TaskUpdateInput) => Promise<void>;
  onDelete: (id: number) => Promise<void>;
  onGenerate: (id: number) => Promise<void>;
  onPreview: (id: number) => Promise<void>;
  onPreviewDraft: (id: number, input: TaskUpdateInput) => Promise<void>;
  onLoadCachedPreview: (id: number) => Promise<string>;
}) {
  const [editing, setEditing] = React.useState(false);
  const [section, setSection] = React.useState<TaskEditorSection>("basic");
  const [draft, setDraft] = React.useState<TaskUpdateInput>(() => taskDraftInput(task));
  const [managedPreviewValues, setManagedPreviewValues] = React.useState<ManagedPreviewValues>(() => initialManagedPreviewValues(task));
  const status = taskStatus(task, busy, t);
  const editorSections = taskEditorSections(t);
  const managedConfigPreview = managedHeaderPreviewFromValues(managedPreviewValues, task.SubscriptionURL, managedConfigDefaults, location.origin);
  const managedConfigEnabledPreview = enabledManagedHeaderPreviewFromValues(managedPreviewValues, task.SubscriptionURL, managedConfigDefaults, location.origin);
  const managedModeLabel = managedPreviewValues.ManagedConfigMode === "global"
    ? t.taskEditor.global
    : t.managedConfigModes[managedPreviewValues.ManagedConfigMode];
  const asideMode = taskEditorAsideMode(section);
  const draftMergeDefaultPinnedNodes = draft.merge_default_pinned_nodes || false;
  const draftIncludeGlobalRules = draft.include_global_rules || false;
  const draftRuleMergeMode = draft.rule_merge_mode || "custom_first";
  const draftFinalRulePolicy = draft.final_rule_policy || "";
  const draftVLESSRelayMode = draft.vless_relay_mode || "global";

  React.useEffect(() => {
    if (!editing) {
      setDraft(taskDraftInput(task));
      setManagedPreviewValues(initialManagedPreviewValues(task));
    }
  }, [editing, task]);

  React.useEffect(() => {
    if (editing) {
      void loadCachedPreview(task.ID);
    }
  }, [editing, task.ID]);

  async function copyURL() {
    const copied = await copyTextToClipboard(absoluteSubscriptionURL(task.SubscriptionURL));
    onCopy(copied ? t.copied : t.copyFailed);
  }

  async function save(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    await onUpdate(task.ID, draft);
    setEditing(false);
  }

  function updateDraft<Key extends keyof TaskUpdateInput>(key: Key, value: TaskUpdateInput[Key]) {
    setDraft((current) => {
      const next = { ...current, [key]: value };
      void onPreviewDraft(task.ID, next);
      return next;
    });
  }

  async function loadCachedPreview(id: number) {
    if (previewContent) return previewContent;
    return onLoadCachedPreview(id);
  }

  function updateManagedPreviewValue<Key extends keyof ManagedPreviewValues>(key: Key, value: ManagedPreviewValues[Key]) {
    setManagedPreviewValues((current) => ({ ...current, [key]: value }));
  }

  function updateManagedField<Key extends keyof ManagedPreviewValues, DraftKey extends keyof TaskUpdateInput>(
    key: Key,
    value: ManagedPreviewValues[Key],
    draftKey: DraftKey,
  ) {
    updateManagedPreviewValue(key, value);
    updateDraft(draftKey, value as TaskUpdateInput[DraftKey]);
  }

  return (
    <div className="task-card">
      <div className="task-main">
        <div>
          <strong>{task.Name}</strong>
          <span>{task.SourceURL}</span>
        </div>
        <span className="badge">{`${task.InputType} ${t.taskList.route} ${task.OutputType}`}</span>
        <span className={status.className}>{status.label}</span>
        <div className="task-actions">
          <button type="button" disabled={busy} onClick={() => onGenerate(task.ID)}><Play size={15} /> {busy ? t.taskList.running : t.taskList.generate}</button>
          <button type="button" disabled={busy} onClick={() => onPreview(task.ID)}><Eye size={15} /> {t.taskList.preview}</button>
          <button type="button" onClick={copyURL}><Copy size={15} /> {t.taskList.copy}</button>
          <button type="button" onClick={() => setEditing(!editing)}><Pencil size={15} /> {editing ? t.forms.cancel : t.taskList.edit}</button>
          <button type="button" className="danger-button" disabled={busy} onClick={() => onDelete(task.ID)}><Trash2 size={15} /> {t.taskList.delete}</button>
        </div>
        <p className="task-meta">{task.LastSuccessAt ? `${t.taskList.lastSuccess}: ${formatTime(task.LastSuccessAt)}` : t.taskList.idle}{task.LastErrorAt ? ` · ${t.taskList.lastError}: ${formatTime(task.LastErrorAt)}` : ""}</p>
        {task.LastErrorMessage && <p className="row-error">{task.LastErrorMessage}</p>}
      </div>
      {editing && (
        <form className="task-editor" onSubmit={save}>
          <header className="task-editor-header">
            <div>
              <span>{t.taskEditor.editing}</span>
              <h3>{t.taskEditor.editTitle}</h3>
              <p>{`${task.Name} · ${t.taskEditor.guidance}`}</p>
            </div>
            <button type="button" className="secondary-button" onClick={() => setEditing(false)}>{t.taskEditor.collapse}</button>
          </header>

          <div className="task-editor-tabs" role="tablist" aria-label={t.taskEditor.editTitle}>
            {editorSections.map((item) => (
              <button key={item.id} type="button" className={`task-editor-tab ${section === item.id ? "active" : ""}`} onClick={() => setSection(item.id)}>
                {item.label}
              </button>
            ))}
          </div>

          <section className="task-editor-shell">
            <div className="task-editor-body">
              <div className={section === "basic" ? "editor-pane active" : "editor-pane"}>
                <EditorSubsection title={t.taskEditor.sections.basic} description={t.taskEditor.intro.basic}>
                  <div className="editor-grid">
                    <label>{t.forms.name}<input name="name" value={draft.name || ""} onChange={(event) => updateDraft("name", event.currentTarget.value)} /></label>
                    <label>{t.forms.refreshSeconds}<input name="refresh_interval_seconds" type="number" value={draft.refresh_interval_seconds || 3600} onChange={(event) => updateDraft("refresh_interval_seconds", Number(event.currentTarget.value))} /></label>
                    <label className="editor-switch"><input name="enabled" type="checkbox" checked={draft.enabled || false} onChange={(event) => updateDraft("enabled", event.currentTarget.checked)} /> {t.forms.enabled}</label>
                    <label className="wide-field">{t.forms.clashURL}<input name="source_url" value={draft.source_url || ""} onChange={(event) => updateDraft("source_url", event.currentTarget.value)} /></label>
                  </div>
                </EditorSubsection>
              </div>

              <div className={section === "conversion" ? "editor-pane active" : "editor-pane"}>
                <EditorSubsection title={t.taskEditor.subsections.nodes} description={t.taskEditor.descriptions.nodes}>
                  <div className="setting-row">
                    <label className="editor-switch"><input name="merge_default_pinned_nodes" type="checkbox" checked={draft.merge_default_pinned_nodes || false} onChange={(event) => updateDraft("merge_default_pinned_nodes", event.currentTarget.checked)} /> {t.forms.mergeDefaults}</label>
                    <p className="field-note">{t.taskEditor.descriptions.mergeDefaults}</p>
                  </div>
                </EditorSubsection>
                <EditorSubsection title={t.taskEditor.subsections.rules} description={t.taskEditor.descriptions.rules}>
                  <div className="setting-row">
                    <label className="editor-switch"><input name="include_global_rules" type="checkbox" checked={draft.include_global_rules || false} onChange={(event) => updateDraft("include_global_rules", event.currentTarget.checked)} /> {t.forms.includeGlobalRules}</label>
                    <p className="field-note">{t.taskEditor.descriptions.includeGlobalRules}</p>
                  </div>
                  <div className="setting-row">
                    <label>{t.forms.ruleMergeMode}<select name="rule_merge_mode" value={draft.rule_merge_mode || "custom_first"} onChange={(event) => updateDraft("rule_merge_mode", event.currentTarget.value as RuleMergeMode)}>{ruleMergeOptions(t)}</select></label>
                    <p className="field-note">{t.taskEditor.descriptions.ruleMergeMode}</p>
                  </div>
                  <div className="setting-row">
                    <label>{t.forms.finalRulePolicy}<input name="final_rule_policy" value={draft.final_rule_policy || ""} onChange={(event) => updateDraft("final_rule_policy", event.currentTarget.value)} placeholder={t.placeholders.finalRulePolicy} /></label>
                    <p className="field-note">{t.taskEditor.descriptions.finalRulePolicy}</p>
                  </div>
                </EditorSubsection>
                <EditorSubsection title={t.taskEditor.subsections.vless} description={t.taskEditor.descriptions.vless}>
                  <div className="setting-row">
                    <label>{t.forms.vlessRelayMode}<select name="vless_relay_mode" value={draft.vless_relay_mode || "global"} onChange={(event) => updateDraft("vless_relay_mode", event.currentTarget.value as VLESSRelayMode)}>{vlessRelayModeOptions(t)}</select></label>
                    <p className="field-note">{t.taskEditor.descriptions.vless}</p>
                  </div>
                </EditorSubsection>
              </div>

              <div className={section === "managed" ? "editor-pane active" : "editor-pane"}>
                <EditorSubsection title={t.taskEditor.subsections.managedMode}>
                  <div className="editor-grid">
                    <label>{t.forms.managedConfigMode}<select name="managed_config_mode" value={managedPreviewValues.ManagedConfigMode} onChange={(event) => updateManagedField("ManagedConfigMode", event.currentTarget.value as TriStateMode, "managed_config_mode")}>{triStateOptions(t.managedConfigModes)}</select></label>
                    <label>{t.forms.managedConfigStrictMode}<select name="managed_config_strict_mode" value={managedPreviewValues.ManagedConfigStrictMode} onChange={(event) => updateManagedField("ManagedConfigStrictMode", event.currentTarget.value as TriStateMode, "managed_config_strict_mode")}>{triStateOptions(t.managedConfigStrictModes)}</select></label>
                  </div>
                </EditorSubsection>
                <EditorSubsection title={t.taskEditor.subsections.managedURL}>
                  <div className="editor-grid">
                    <label>{t.forms.managedConfigURLMode}<select name="managed_config_url_mode" value={managedPreviewValues.ManagedConfigURLMode} onChange={(event) => updateManagedField("ManagedConfigURLMode", event.currentTarget.value as ManagedURLMode, "managed_config_url_mode")}>{managedURLModeOptions(t)}</select></label>
                    <label>{t.forms.managedConfigCustomURL}<input name="managed_config_custom_url" value={managedPreviewValues.ManagedConfigCustomURL} onChange={(event) => updateManagedField("ManagedConfigCustomURL", event.currentTarget.value, "managed_config_custom_url")} placeholder="https://profiles.example.com/main.conf" /></label>
                  </div>
                </EditorSubsection>
                <EditorSubsection title={t.taskEditor.subsections.managedRefresh}>
                  <div className="editor-grid">
                    <label>{t.forms.managedConfigIntervalMode}<select name="managed_config_interval_mode" value={managedPreviewValues.ManagedConfigIntervalMode} onChange={(event) => updateManagedField("ManagedConfigIntervalMode", event.currentTarget.value as ManagedIntervalMode, "managed_config_interval_mode")}>{managedIntervalModeOptions(t)}</select></label>
                    <label>{t.forms.managedConfigIntervalCustom}<input name="managed_config_interval_seconds" type="number" min="60" value={managedPreviewValues.ManagedConfigIntervalSeconds} onChange={(event) => updateManagedField("ManagedConfigIntervalSeconds", Number(event.currentTarget.value), "managed_config_interval_seconds")} /></label>
                  </div>
                </EditorSubsection>
              </div>

              <div className={section === "custom" ? "editor-pane active" : "editor-pane"}>
                <EditorSubsection title={t.forms.customRules} description={t.taskEditor.descriptions.customRules}>
                  <textarea name="custom_rules_text" rows={7} value={draft.custom_rules_text || ""} onChange={(event) => updateDraft("custom_rules_text", event.currentTarget.value)} placeholder={t.placeholders.customRules} />
                </EditorSubsection>
                <EditorSubsection title={t.forms.customGroups} description={t.taskEditor.descriptions.customGroups}>
                  <PolicyGroupEditor value={draft.custom_groups_text || ""} previewContent={previewContent} subscriptionInfoKeywordsText={subscriptionInfoKeywordsText} t={t} onNeedPreview={() => loadCachedPreview(task.ID)} onChange={(value) => updateDraft("custom_groups_text", value)} />
                </EditorSubsection>
              </div>
            </div>

            <aside className="task-editor-aside">
              {asideMode === "basic" && (
                <EditorSummaryCard title={t.taskEditor.summaryTitle}>
                  <SummaryList items={[
                    [t.taskEditor.taskStatus, status.label],
                    [t.taskEditor.outputType, `${task.InputType} ${t.taskList.route} ${task.OutputType}`],
                    [t.forms.refreshSeconds, String(task.RefreshIntervalSeconds)],
                    [t.taskList.copy, absoluteSubscriptionURL(task.SubscriptionURL)],
                  ]} />
                </EditorSummaryCard>
              )}
              {asideMode === "conversion" && (
                <EditorSummaryCard title={t.taskEditor.summaryTitle}>
                  <SummaryList items={[
                    [t.forms.mergeDefaults, draftMergeDefaultPinnedNodes ? t.taskList.enabled : t.taskList.disabled],
                    [t.forms.includeGlobalRules, draftIncludeGlobalRules ? t.taskList.enabled : t.taskList.disabled],
                    [t.forms.ruleMergeMode, t.ruleMergeModes[draftRuleMergeMode]],
                    [t.forms.finalRulePolicy, draftFinalRulePolicy || t.taskEditor.autoFinalPolicy],
                    [t.forms.vlessRelayMode, t.vlessRelayModes[draftVLESSRelayMode]],
                  ]} />
                </EditorSummaryCard>
              )}
              {asideMode === "managed" && (
                <>
                  <EditorSummaryCard title={t.taskEditor.summaryTitle}>
                    <SummaryList items={[
                      [t.taskEditor.managedState, managedModeLabel],
                      [t.forms.managedConfigURLMode, t.managedConfigURLModes[managedPreviewValues.ManagedConfigURLMode]],
                      [t.forms.managedConfigIntervalMode, t.managedConfigIntervalModes[managedPreviewValues.ManagedConfigIntervalMode]],
                      [t.forms.managedConfigStrictMode, t.managedConfigStrictModes[managedPreviewValues.ManagedConfigStrictMode]],
                    ]} />
                  </EditorSummaryCard>
                  <EditorSummaryCard title={t.taskEditor.previewTitle}>
                    <pre className="inline-preview">{managedConfigPreview}</pre>
                    {managedConfigPreview === "MANAGED-CONFIG disabled" && (
                      <>
                        <h4>{t.taskEditor.enabledPreviewTitle}</h4>
                        <pre className="inline-preview">{managedConfigEnabledPreview}</pre>
                      </>
                    )}
                  </EditorSummaryCard>
                </>
              )}
              {asideMode === "custom" && (
                <EditorSummaryCard title={t.taskEditor.summaryTitle}>
                  <SummaryList items={[
                    [t.forms.customRules, String(lineCount(draft.custom_rules_text || ""))],
                    [t.forms.customGroups, String(lineCount(draft.custom_groups_text || ""))],
                    [t.forms.ruleMergeMode, t.ruleMergeModes[draftRuleMergeMode]],
                  ]} />
                </EditorSummaryCard>
              )}
            </aside>
          </section>

          <footer className="task-editor-footer">
            <button type="button" className="secondary-button" onClick={() => setEditing(false)}><X size={15} /> {t.forms.cancel}</button>
            <button disabled={busy}><Save size={15} /> {busy ? t.saving : t.forms.save}</button>
          </footer>
        </form>
      )}
    </div>
  );
}

function taskEditorSections(t: typeof translations[Language]): Array<{ id: TaskEditorSection; label: string }> {
  return (["basic", "conversion", "managed", "custom"] as TaskEditorSection[]).map((id) => ({
    id,
    label: t.taskEditor.sections[id],
  }));
}

function EditorSummaryCard({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <section className="editor-summary-card">
      <h4>{title}</h4>
      {children}
    </section>
  );
}

function SummaryList({ items }: { items: Array<[string, string]> }) {
  return (
    <div className="summary-list">
      {items.map(([label, value]) => (
        <div key={label}><span>{label}</span><strong>{value}</strong></div>
      ))}
    </div>
  );
}

function PolicyGroupEditor({
  value,
  previewContent,
  subscriptionInfoKeywordsText,
  t,
  onNeedPreview,
  onChange,
}: {
  value: string;
  previewContent: string;
  subscriptionInfoKeywordsText: string;
  t: typeof translations[Language];
  onNeedPreview: () => Promise<string>;
  onChange: (value: string) => void;
}) {
  const parsedGroups = React.useMemo(() => parsePolicyGroupsText(value), [value]);
  const nodeCandidates = React.useMemo(() => parseProxyNodeCandidates(previewContent, subscriptionInfoKeywordsText), [previewContent, subscriptionInfoKeywordsText]);
  const importGroups = React.useMemo(() => parseProxyGroupCandidates(previewContent), [previewContent]);
  const [advancedMode, setAdvancedMode] = React.useState(() => Boolean(value.trim() && parsedGroups.length === 0));
  const [groups, setGroups] = React.useState<PolicyGroup[]>(() => parsedGroups);
  const [batchMembers, setBatchMembers] = React.useState("");
  const [targetGroupID, setTargetGroupID] = React.useState(() => parsedGroups[0]?.id || "");
  const [importGroupName, setImportGroupName] = React.useState("");
  const [selectorGroupID, setSelectorGroupID] = React.useState("");
  const [memberDrafts, setMemberDrafts] = React.useState<Record<string, string>>(() => memberDraftsFromGroups(parsedGroups));
  const lastStructuredValue = React.useRef(policyGroupsText(groups));

  React.useEffect(() => {
    if (value === lastStructuredValue.current) return;
    const next = parsePolicyGroupsText(value);
    if (next.length) {
      setGroups(next);
      setMemberDrafts(memberDraftsFromGroups(next));
      setTargetGroupID((current) => next.some((group) => group.id === current) ? current : next[0].id);
      lastStructuredValue.current = value;
      setAdvancedMode(false);
    } else if (value.trim()) {
      setAdvancedMode(true);
    }
  }, [value]);

  React.useEffect(() => {
    if (!importGroups.length) {
      setImportGroupName("");
      return;
    }
    setImportGroupName((current) => importGroups.some((group) => group.name === current) ? current : importGroups[0].name);
  }, [importGroups]);

  function commit(nextGroups: PolicyGroup[]) {
    const nextValue = policyGroupsText(nextGroups);
    lastStructuredValue.current = nextValue;
    setGroups(nextGroups);
    setMemberDrafts((current) => reconcileMemberDrafts(current, nextGroups));
    setTargetGroupID((current) => nextGroups.some((group) => group.id === current) ? current : (nextGroups[0]?.id || ""));
    onChange(nextValue);
  }

  function updateGroup(id: string, patch: Partial<PolicyGroup>) {
    commit(groups.map((group) => group.id === id ? { ...group, ...patch } : group));
  }

  function addGroup(type: PolicyGroupType) {
    const nextGroup = createPolicyGroup(type);
    commit([...groups, nextGroup]);
    setTargetGroupID(nextGroup.id);
  }

  function removeGroup(id: string) {
    commit(groups.filter((group) => group.id !== id));
  }

  function addBatchMembers(group: PolicyGroup) {
    const nextMembers = [...group.members, ...membersFromText(batchMembers)];
    setBatchMembers("");
    updateGroup(group.id, { members: nextMembers });
  }

  function importGroupMembers(group: PolicyGroup, members: string[], mode: "append" | "replace") {
    updateGroup(group.id, { members: mode === "replace" ? members : mergeMembers(group.members, members) });
  }

  function handleMemberDraftChange(group: PolicyGroup, text: string) {
    setMemberDrafts((current) => ({ ...current, [group.id]: text }));
    updateGroup(group.id, { members: membersFromText(text) });
  }

  const targetGroup = groups.find((group) => group.id === targetGroupID) || groups[0];
  const selectedImportGroup = importGroups.find((group) => group.name === importGroupName) || importGroups[0];
  const selectorGroup = groups.find((group) => group.id === selectorGroupID);
  const generatedPreview = policyGroupsText(groups);

  if (advancedMode) {
    return (
      <div className="policy-editor">
        <div className="policy-editor-toolbar">
          <div>
            <strong>{t.policyGroups.advancedTitle}</strong>
            <span>{t.policyGroups.advancedHint}</span>
          </div>
          <button type="button" className="secondary-button" onClick={() => setAdvancedMode(false)}>{t.policyGroups.structuredMode}</button>
        </div>
        <textarea name="custom_groups_text" rows={8} value={value} onChange={(event) => onChange(event.currentTarget.value)} placeholder={t.placeholders.customGroups} />
      </div>
    );
  }

  return (
    <div className="policy-editor">
      <div className="policy-editor-toolbar">
        <div>
          <strong>{t.policyGroups.structuredTitle}</strong>
          <span>{t.policyGroups.structuredHint}</span>
        </div>
        <button type="button" className="secondary-button" onClick={() => setAdvancedMode(true)}>{t.policyGroups.advancedMode}</button>
      </div>

      <div className="policy-editor-grid">
        <div className="policy-group-list">
          {groups.map((group, index) => (
            <section className="policy-group-card" key={group.id}>
              <header>
                <div>
                  <span>{t.policyGroups.groupLabel} {index + 1}</span>
                  <strong>{group.name || t.policyGroups.unnamed}</strong>
                </div>
                <button type="button" className="secondary-button icon-button" onClick={() => removeGroup(group.id)} aria-label={t.policyGroups.removeGroup}><Trash2 size={15} /></button>
              </header>
              <div className="editor-grid">
                <label>{t.policyGroups.name}<input value={group.name} onChange={(event) => updateGroup(group.id, { name: event.currentTarget.value })} /></label>
                <label>{t.policyGroups.type}<select value={group.type} onChange={(event) => updateGroup(group.id, { type: event.currentTarget.value as PolicyGroupType })}>{policyGroupTypeOptions(t)}</select></label>
              </div>
              {group.type === "smart" && (
                <label className="editor-switch"><input type="checkbox" checked={group.includeAllProxies} onChange={(event) => updateGroup(group.id, { includeAllProxies: event.currentTarget.checked })} /> {t.policyGroups.includeAllProxies}</label>
              )}
              <label>{t.policyGroups.members}<textarea rows={5} value={memberDrafts[group.id] ?? group.members.join("\n")} onChange={(event) => handleMemberDraftChange(group, event.currentTarget.value)} placeholder={t.policyGroups.membersPlaceholder} /></label>
              <button type="button" className="secondary-button" onClick={() => {
                if (!previewContent) void onNeedPreview();
                setSelectorGroupID(group.id);
              }}>{t.policyGroups.selectMembers}</button>
              <pre className="inline-preview policy-line-preview">{policyGroupLine(group) || t.policyGroups.emptyPreview}</pre>
            </section>
          ))}
          {!groups.length && <p className="policy-empty">{t.policyGroups.emptyState}</p>}
          <div className="policy-add-row">
            {(["smart", "select", "url-test", "fallback", "load-balance"] as PolicyGroupType[]).map((type) => (
              <button type="button" className="secondary-button" key={type} onClick={() => addGroup(type)}><Plus size={14} /> {t.policyGroupTypes[type]}</button>
            ))}
          </div>
        </div>

        <aside className="policy-helper">
          <h4>{t.policyGroups.batchTitle}</h4>
          <p>{t.policyGroups.batchHint}</p>
          <label>{t.policyGroups.batchTarget}<select value={targetGroup?.id || ""} onChange={(event) => setTargetGroupID(event.currentTarget.value)} disabled={!groups.length}>
            {groups.map((group, index) => (
              <option key={group.id} value={group.id}>{group.name || `${t.policyGroups.groupLabel} ${index + 1}`}</option>
            ))}
          </select></label>
          <textarea rows={6} value={batchMembers} onChange={(event) => setBatchMembers(event.currentTarget.value)} placeholder={t.policyGroups.batchPlaceholder} />
          <button type="button" disabled={!targetGroup || !batchMembers.trim()} onClick={() => targetGroup && addBatchMembers(targetGroup)}><Plus size={14} /> {t.policyGroups.addToTargetGroup}</button>
          <h4>{t.policyGroups.importTitle}</h4>
          <p>{t.policyGroups.importHint}</p>
          <label>{t.policyGroups.importSource}<select value={selectedImportGroup?.name || ""} onChange={(event) => setImportGroupName(event.currentTarget.value)} disabled={!importGroups.length}>
            {importGroups.map((group) => (
              <option key={group.name} value={group.name}>{`${group.name} (${group.type}, ${group.members.length})`}</option>
            ))}
          </select></label>
          <div className="policy-import-actions">
            <button type="button" className="secondary-button" disabled={!targetGroup || !selectedImportGroup} onClick={() => targetGroup && selectedImportGroup && importGroupMembers(targetGroup, selectedImportGroup.members, "append")}>{t.policyGroups.importAppend}</button>
            <button type="button" className="secondary-button" disabled={!targetGroup || !selectedImportGroup} onClick={() => targetGroup && selectedImportGroup && importGroupMembers(targetGroup, selectedImportGroup.members, "replace")}>{t.policyGroups.importReplace}</button>
          </div>
          {!importGroups.length && <p className="field-note">{t.policyGroups.noPreviewGroups}</p>}
          <h4>{t.policyGroups.generatedTitle}</h4>
          <pre className="inline-preview">{generatedPreview || t.policyGroups.emptyPreview}</pre>
        </aside>
      </div>
      {selectorGroup && (
        <PolicyMemberSelector
          group={selectorGroup}
          candidates={nodeCandidates}
          t={t}
          onClose={() => setSelectorGroupID("")}
          onAppend={(members) => {
            updateGroup(selectorGroup.id, { members: mergeMembers(selectorGroup.members, members) });
            setSelectorGroupID("");
          }}
          onReplace={(members) => {
            updateGroup(selectorGroup.id, { members });
            setSelectorGroupID("");
          }}
          onRemoveSubscriptionInfoMembers={(members) => updateGroup(selectorGroup.id, { members })}
        />
      )}
    </div>
  );
}

function PolicyMemberSelector({
  group,
  candidates,
  t,
  onClose,
  onAppend,
  onReplace,
  onRemoveSubscriptionInfoMembers,
}: {
  group: PolicyGroup;
  candidates: ProxyNodeCandidate[];
  t: typeof translations[Language];
  onClose: () => void;
  onAppend: (members: string[]) => void;
  onReplace: (members: string[]) => void;
  onRemoveSubscriptionInfoMembers: (members: string[]) => void;
}) {
  const [query, setQuery] = React.useState("");
  const [keywordFilter, setKeywordFilter] = React.useState("");
  const [memberCategoryFilter, setMemberCategoryFilter] = React.useState<"all" | ProxyNodeCategory | "selected" | "unselected">("all");
  const [selected, setSelected] = React.useState<string[]>([]);
  const normalizedQuery = query.trim().toLowerCase();
  const keywordFilters = keywordFilter.toLowerCase().split(/\s+/).map((keyword) => keyword.trim()).filter(Boolean);
  const visibleCandidates = candidates
    .filter((candidate) => candidate.name.toLowerCase().includes(normalizedQuery))
    .filter((candidate) => keywordFilters.every((keyword) => candidate.name.toLowerCase().includes(keyword)))
    .filter((candidate) => {
      if (memberCategoryFilter === "regular" || memberCategoryFilter === "subscription_info") return candidate.category === memberCategoryFilter;
      if (memberCategoryFilter === "selected") return selected.includes(candidate.name);
      if (memberCategoryFilter === "unselected") return !selected.includes(candidate.name);
      return true;
    });
  const regularNodes = visibleCandidates.filter((candidate) => candidate.category === "regular");
  const infoNodes = visibleCandidates.filter((candidate) => candidate.category === "subscription_info");

  function toggle(name: string) {
    setSelected((current) => current.includes(name) ? current.filter((item) => item !== name) : [...current, name]);
  }

  function selectRegularNodes() {
    setSelected((current) => mergeMembers(current, regularNodes.map((node) => node.name)));
  }

  function selectVisibleNodes() {
    setSelected((current) => mergeMembers(current, visibleCandidates.map((node) => node.name)));
  }

  function removeSubscriptionInfoMembers() {
    const infoNames = new Set(candidates.filter((candidate) => candidate.category === "subscription_info").map((candidate) => candidate.name));
    onRemoveSubscriptionInfoMembers(group.members.filter((member) => !infoNames.has(member)));
  }

  return (
    <div className="policy-member-overlay" role="dialog" aria-modal="true" aria-label={t.policyGroups.memberSelectorTitle}>
      <section className="policy-member-dialog">
        <header>
          <div>
            <span>{t.policyGroups.memberSelectorTitle}</span>
            <h4>{group.name || t.policyGroups.unnamed}</h4>
          </div>
          <button type="button" className="secondary-button icon-button" onClick={onClose} aria-label={t.forms.cancel}><X size={15} /></button>
        </header>
        <div className="policy-member-tools">
          <label>{t.policyGroups.searchMembers}<input value={query} onChange={(event) => setQuery(event.currentTarget.value)} placeholder={t.policyGroups.searchMembersPlaceholder} /></label>
          <label>{t.policyGroups.keywordFilter}<input value={keywordFilter} onChange={(event) => setKeywordFilter(event.currentTarget.value)} placeholder={t.policyGroups.keywordFilterPlaceholder} /></label>
          <label>{t.policyGroups.memberFilter}<select value={memberCategoryFilter} onChange={(event) => setMemberCategoryFilter(event.currentTarget.value as typeof memberCategoryFilter)}>
            <option value="all">{t.policyGroups.filterAll}</option>
            <option value="regular">{t.policyGroups.regularNodes}</option>
            <option value="subscription_info">{t.policyGroups.subscriptionInfoNodes}</option>
            <option value="selected">{t.policyGroups.filterSelected}</option>
            <option value="unselected">{t.policyGroups.filterUnselected}</option>
          </select></label>
        </div>
        <div className="policy-member-actions">
          <button type="button" className="secondary-button" disabled={!visibleCandidates.length} onClick={selectVisibleNodes}>{t.policyGroups.selectVisibleNodes}</button>
          <button type="button" className="secondary-button" disabled={!regularNodes.length} onClick={selectRegularNodes}>{t.policyGroups.selectRegularNodes}</button>
          <button type="button" className="secondary-button" disabled={!candidates.some((candidate) => candidate.category === "subscription_info")} onClick={removeSubscriptionInfoMembers}>{t.policyGroups.removeSubscriptionInfoMembers}</button>
        </div>
        {!candidates.length && <p className="policy-empty">{t.policyGroups.noPreviewNodes}</p>}
        {candidates.length > 0 && (
          <div className="policy-member-list">
            <PolicyMemberGroup title={t.policyGroups.regularNodes} emptyText={t.policyGroups.noMatchingNodes} nodes={regularNodes} selected={selected} onToggle={toggle} />
            <PolicyMemberGroup title={t.policyGroups.subscriptionInfoNodes} emptyText={t.policyGroups.noMatchingNodes} nodes={infoNodes} selected={selected} onToggle={toggle} />
          </div>
        )}
        <footer>
          <span>{t.policyGroups.selectedCount.replace("{count}", String(selected.length))}</span>
          <button type="button" className="secondary-button" onClick={onClose}>{t.forms.cancel}</button>
          <button type="button" disabled={!selected.length} onClick={() => onReplace(selected)}>{t.policyGroups.replaceSelected}</button>
          <button type="button" disabled={!selected.length} onClick={() => onAppend(selected)}>{t.policyGroups.appendSelected}</button>
        </footer>
      </section>
    </div>
  );
}

function PolicyMemberGroup({ title, emptyText, nodes, selected, onToggle }: { title: string; emptyText: string; nodes: ProxyNodeCandidate[]; selected: string[]; onToggle: (name: string) => void }) {
  return (
    <section className="policy-member-group">
      <h5>{title}</h5>
      {nodes.length ? nodes.map((node) => (
        <label key={node.name} className="policy-member-option">
          <input type="checkbox" checked={selected.includes(node.name)} onChange={() => onToggle(node.name)} />
          <span>{node.name}</span>
        </label>
      )) : <p>{emptyText}</p>}
    </section>
  );
}

function mergeMembers(current: string[], next: string[]) {
  return [...current, ...next].filter((member, index, members) => member.trim() && members.indexOf(member) === index);
}

function memberDraftsFromGroups(groups: PolicyGroup[]) {
  return Object.fromEntries(groups.map((group) => [group.id, group.members.join("\n")]));
}

function reconcileMemberDrafts(current: Record<string, string>, groups: PolicyGroup[]) {
  return Object.fromEntries(groups.map((group) => {
    const nextText = group.members.join("\n");
    return [group.id, current[group.id] === undefined || membersFromText(current[group.id]).join("\n") !== nextText ? nextText : current[group.id]];
  }));
}

function policyGroupTypeOptions(t: typeof translations[Language]) {
  return (["smart", "select", "url-test", "fallback", "load-balance"] as PolicyGroupType[]).map((type) => (
    <option key={type} value={type}>{t.policyGroupTypes[type]}</option>
  ));
}

function lineCount(value: string) {
  return value.trim() ? value.trim().split(/\r?\n/).length : 0;
}

function EditorSubsection({ title, description, children }: { title: string; description?: string; children: React.ReactNode }) {
  return (
    <section className="editor-subsection">
      <header>
        <h4>{title}</h4>
        {description && <p>{description}</p>}
      </header>
      {children}
    </section>
  );
}

function initialManagedPreviewValues(task: Task): ManagedPreviewValues {
  return {
    ManagedConfigMode: task.ManagedConfigMode || "global",
    ManagedConfigURLMode: task.ManagedConfigURLMode === "custom" ? "custom" : "task_subscription",
    ManagedConfigCustomURL: task.ManagedConfigCustomURL || "",
    ManagedConfigIntervalMode: task.ManagedConfigIntervalMode || "global",
    ManagedConfigIntervalSeconds: task.ManagedConfigIntervalSeconds || 86400,
    ManagedConfigStrictMode: task.ManagedConfigStrictMode || "global",
  };
}

function taskDraftInput(task: Task): TaskUpdateInput {
  return {
    name: task.Name,
    source_url: task.SourceURL,
    refresh_interval_seconds: task.RefreshIntervalSeconds || 3600,
    enabled: task.Enabled,
    merge_default_pinned_nodes: task.MergeDefaultPinnedNodes,
    include_global_rules: task.IncludeGlobalRules,
    custom_rules_text: task.CustomRulesText || "",
    rule_merge_mode: task.RuleMergeMode || "custom_first",
    final_rule_policy: task.FinalRulePolicy || "",
    custom_groups_text: task.CustomGroupsText || "",
    vless_relay_mode: task.VLESSRelayMode || "global",
    managed_config_mode: task.ManagedConfigMode || "global",
    managed_config_url_mode: task.ManagedConfigURLMode === "custom" ? "custom" : "task_subscription",
    managed_config_custom_url: task.ManagedConfigCustomURL || "",
    managed_config_interval_mode: task.ManagedConfigIntervalMode || "global",
    managed_config_interval_seconds: task.ManagedConfigIntervalSeconds || 86400,
    managed_config_strict_mode: task.ManagedConfigStrictMode || "global",
  };
}

function ruleMergeOptions(t: typeof translations[Language]) {
  return (["custom_first", "upstream_first", "custom_first_dedupe", "upstream_first_dedupe"] as RuleMergeMode[]).map((mode) => (
    <option key={mode} value={mode}>{t.ruleMergeModes[mode]}</option>
  ));
}

function triStateOptions(labels: Record<TriStateMode, string>) {
  return (["global", "enabled", "disabled"] as TriStateMode[]).map((mode) => (
    <option key={mode} value={mode}>{labels[mode]}</option>
  ));
}

function vlessRelayModeOptions(t: typeof translations[Language]) {
  return (["global", "enabled", "disabled"] as VLESSRelayMode[]).map((mode) => (
    <option key={mode} value={mode}>{t.vlessRelayModes[mode]}</option>
  ));
}

function managedURLModeOptions(t: typeof translations[Language]) {
  return taskManagedURLModes().map((mode) => (
    <option key={mode} value={mode}>{t.managedConfigURLModes[mode]}</option>
  ));
}

function globalManagedURLModeOptions(t: typeof translations[Language]) {
  return (["task_subscription", "custom"] as GlobalManagedURLMode[]).map((mode) => (
    <option key={mode} value={mode}>{t.globalManagedConfigURLModes[mode]}</option>
  ));
}

function managedIntervalModeOptions(t: typeof translations[Language]) {
  return (["global", "custom"] as ManagedIntervalMode[]).map((mode) => (
    <option key={mode} value={mode}>{t.managedConfigIntervalModes[mode]}</option>
  ));
}

function managedIntervalPresetOptions(t: typeof translations[Language]) {
  return (["3600", "21600", "43200", "86400", "custom"] as const).map((value) => (
    <option key={value} value={value}>{t.managedConfigIntervals[value]}</option>
  ));
}

function intervalPresetValue(value: number) {
  return [3600, 21600, 43200, 86400].includes(value) ? String(value) : "custom";
}

function taskStatus(task: Task, busy: boolean, t: typeof translations[Language]) {
  if (busy) return { label: t.taskList.running, className: "status-running" };
  if (!task.Enabled) return { label: t.taskList.disabled, className: "" };
  if (task.LastErrorMessage) return { label: t.taskList.failed, className: "status-error" };
  if (task.LastSuccessAt) return { label: t.taskList.success, className: "status-success" };
  return { label: t.taskList.idle, className: "" };
}

function formatTime(value: string) {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  return date.toLocaleString();
}

type NodeFilter = "all" | "enabled" | "disabled";

function NodeManagement({ nodes, t, placeholder, onImport, onDelete, onBatch, onCopy, importToast }: { nodes: PinnedNode[]; t: typeof translations[Language]; placeholder: string; onImport: (event: React.FormEvent<HTMLFormElement>) => void; onDelete: (id: number) => void; onBatch: (action: NodeBatchAction, ids: number[]) => Promise<void>; onCopy: (message: string) => void; importToast: string }) {
  const [draftText, setDraftText] = React.useState("");
  const [query, setQuery] = React.useState("");
  const [filter, setFilter] = React.useState<NodeFilter>("all");
  const [selectedNodeIDs, setSelectedNodeIDs] = React.useState<number[]>([]);
  React.useEffect(() => {
    if (importToast === t.nodeList.imported) setDraftText("");
  }, [importToast, t.nodeList.imported]);
  React.useEffect(() => {
    const existing = new Set(nodes.map((node) => node.ID));
    setSelectedNodeIDs((current) => current.filter((id) => existing.has(id)));
  }, [nodes]);
  const importPreview = React.useMemo(() => previewNodeImportLines(draftText), [draftText]);
  const protocolCount = new Set(nodes.map((node) => node.Protocol)).size;
  const filteredNodes = nodes.filter((node) => {
    const matchesQuery = !query.trim() || [node.Name, node.Protocol, node.Server, String(node.Port)].some((value) => value.toLowerCase().includes(query.trim().toLowerCase()));
    const matchesFilter = filter === "all" || (filter === "enabled" ? node.Enabled : !node.Enabled);
    return matchesQuery && matchesFilter;
  });

  async function copyNodeName(name: string) {
    const copied = await copyTextToClipboard(name);
    onCopy(copied ? t.nodeList.copiedName : t.copyFailed);
  }

  function toggleNode(id: number) {
    setSelectedNodeIDs((current) => current.includes(id) ? current.filter((item) => item !== id) : [...current, id]);
  }

  function toggleVisibleNodes() {
    const visibleIDs = filteredNodes.map((node) => node.ID);
    const selected = new Set(selectedNodeIDs);
    const allVisibleSelected = visibleIDs.length > 0 && visibleIDs.every((id) => selected.has(id));
    setSelectedNodeIDs((current) => allVisibleSelected ? current.filter((id) => !visibleIDs.includes(id)) : Array.from(new Set([...current, ...visibleIDs])));
  }

  async function applyBatch(action: NodeBatchAction) {
    await onBatch(action, selectedNodeIDs);
    setSelectedNodeIDs([]);
  }

  return (
    <section className="node-management">
      <div className="node-metrics">
        <Metric icon={<ShieldCheck />} value={nodes.length} label={t.nodeList.total} />
        <Metric icon={<Server />} value={nodes.filter((node) => node.Enabled).length} label={t.nodeList.mergeable} />
        <Metric icon={<Layers />} value={protocolCount} label={t.nodeList.protocols} />
      </div>
      <form className="panel node-import-panel" onSubmit={onImport}>
        <div>
          <h3>{t.nodeList.importTitle}</h3>
          <p className="muted">{t.nodeList.importHint}</p>
        </div>
        <label>{t.forms.proxyURIs}<textarea name="text" rows={7} value={draftText} onChange={(event) => setDraftText(event.currentTarget.value)} placeholder={placeholder} /></label>
        {importPreview.length > 0 && (
          <div className="node-import-preview">
            <strong>{t.nodeList.importPreview}</strong>
            {importPreview.map((item, index) => (
              <div className="node-import-preview-row" key={`${item.name}-${index}`}>
                <span className={item.detected ? "node-preview-mark" : "node-preview-mark review"}>{item.detected ? "OK" : "!"}</span>
                <div>
                  <strong>{item.name}</strong>
                  <span>{item.detail}</span>
                </div>
                <span className="badge">{item.detected ? t.nodeList.detected : t.nodeList.needsReview}</span>
              </div>
            ))}
          </div>
        )}
        <button><Plus size={16} /> {t.forms.importNodes}</button>
      </form>
      <NodeList
        nodes={filteredNodes}
        totalNodes={nodes.length}
        t={t}
        query={query}
        filter={filter}
        selectedNodeIDs={selectedNodeIDs}
        onQueryChange={setQuery}
        onFilterChange={setFilter}
        onToggleNode={toggleNode}
        onToggleVisibleNodes={toggleVisibleNodes}
        onBatch={applyBatch}
        onCopyName={copyNodeName}
        onDelete={onDelete}
      />
    </section>
  );
}

function NodeList({ nodes, totalNodes, t, query, filter, selectedNodeIDs, onQueryChange, onFilterChange, onToggleNode, onToggleVisibleNodes, onBatch, onCopyName, onDelete }: { nodes: PinnedNode[]; totalNodes: number; t: typeof translations[Language]; query: string; filter: NodeFilter; selectedNodeIDs: number[]; onQueryChange: (value: string) => void; onFilterChange: (value: NodeFilter) => void; onToggleNode: (id: number) => void; onToggleVisibleNodes: () => void; onBatch: (action: NodeBatchAction) => void; onCopyName: (name: string) => void; onDelete: (id: number) => void }) {
  const visibleIDs = nodes.map((node) => node.ID);
  const allVisibleSelected = visibleIDs.length > 0 && visibleIDs.every((id) => selectedNodeIDs.includes(id));
  const hasSelection = selectedNodeIDs.length > 0;
  return (
    <section className="panel node-library">
      <div className="node-library-header">
        <div>
          <h3>{t.nodeList.title}</h3>
          <p className="muted">{t.nodeList.libraryHint}</p>
        </div>
        <span className="badge">{nodes.length}/{totalNodes}</span>
      </div>
      <div className="node-library-tools">
        <div className="node-filter-tools">
          <label className="node-search"><Search size={15} /><input value={query} onChange={(event) => onQueryChange(event.currentTarget.value)} placeholder={t.nodeList.search} /></label>
          <div className="node-filter-tabs">
            {(["all", "enabled", "disabled"] as NodeFilter[]).map((item) => (
              <button key={item} type="button" className={filter === item ? "active" : ""} onClick={() => onFilterChange(item)}>{nodeFilterLabel(item, t)}</button>
            ))}
          </div>
        </div>
        <div className="node-batch-actions">
          <span className={hasSelection ? "node-selected-count active" : "node-selected-count"}>{t.nodeList.selectedCount.replace("{count}", String(selectedNodeIDs.length))}</span>
          <button type="button" className="secondary-button" disabled={!hasSelection} onClick={() => onBatch("enable")}>{t.nodeList.batchEnable}</button>
          <button type="button" className="secondary-button" disabled={!hasSelection} onClick={() => onBatch("disable")}>{t.nodeList.batchDisable}</button>
          <button type="button" className="danger-button" disabled={!hasSelection} onClick={() => onBatch("delete")}>{t.nodeList.batchDelete}</button>
        </div>
      </div>
      <div className="node-table">
        <div className="node-table-head">
          <span><input type="checkbox" checked={allVisibleSelected} onChange={onToggleVisibleNodes} aria-label={t.nodeList.selectVisible} /></span>
          <span>{t.nodeList.title}</span>
          <span>{t.nodeList.protocols}</span>
          <span>{t.nodeList.enabled}</span>
          <span>{t.forms.save}</span>
        </div>
        {nodes.length ? nodes.map((node) => (
          <div className="node-table-row" key={node.ID}>
            <span><input type="checkbox" checked={selectedNodeIDs.includes(node.ID)} onChange={() => onToggleNode(node.ID)} aria-label={node.Name} /></span>
            <div className="node-name-cell">
              <strong><span className={node.Enabled ? "node-dot" : "node-dot disabled"} />{node.Name}</strong>
              <span>{node.Server}:{node.Port}</span>
            </div>
            <span className="badge">{node.Protocol}</span>
            <span className={node.Enabled ? "status-success" : ""}>{node.Enabled ? t.nodeList.enabled : t.nodeList.disabled}</span>
            <div className="node-row-actions">
              <button type="button" className="node-action-button" title={t.nodeList.copyName} onClick={() => onCopyName(node.Name)}><Copy size={15} /></button>
              <button type="button" className="node-action-button danger" title={t.nodeList.delete} onClick={() => onDelete(node.ID)}><Trash2 size={15} /></button>
            </div>
          </div>
        )) : <p className="muted">{t.nodeList.empty}</p>}
      </div>
    </section>
  );
}

function nodeFilterLabel(filter: NodeFilter, t: typeof translations[Language]) {
  if (filter === "enabled") return t.nodeList.enabledOnly;
  if (filter === "disabled") return t.nodeList.disabledOnly;
  return t.nodeList.all;
}

function previewNodeImportLines(text: string) {
  return text.split(/\r?\n/).map((line) => line.trim()).filter(Boolean).slice(0, 4).map((line) => {
    const surgeMatch = line.match(/^(.+?)\s*=\s*([a-z0-9-]+)\s*,\s*([^,\s]+)\s*,\s*(\d+)/i);
    const uriMatch = line.match(/^([a-z0-9+.-]+):\/\//i);
    if (surgeMatch) {
      return { detected: true, name: surgeMatch[1].replace(/^"|"$/g, ""), detail: `${surgeMatch[2]} / ${surgeMatch[3]}:${surgeMatch[4]}` };
    }
    if (uriMatch) {
      const hashName = decodeURIComponent(line.split("#")[1] || uriMatch[1]);
      return { detected: true, name: hashName, detail: uriMatch[1] };
    }
    return { detected: false, name: line.slice(0, 40), detail: "可能不是代理节点" };
  });
}

function absoluteSubscriptionURL(url: string) {
  return buildAbsoluteSubscriptionURL(url, location.origin);
}

const defaultManagedConfigDefaults: ManagedConfigDefaults = {
  Enabled: false,
  URLMode: "task_subscription",
  CustomURL: "",
  IntervalSeconds: 86400,
  Strict: false,
};

createRoot(document.getElementById("root")!).render(<App />);
