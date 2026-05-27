import React from "react";
import { createRoot } from "react-dom/client";
import { AlertCircle, Copy, Eye, KeyRound, Layers, Pencil, Play, Plus, RefreshCw, Save, Server, ShieldCheck, Trash2, X } from "lucide-react";
import { api } from "./api";
import { getInitialLanguage, languageStorageKey, languages, translations, type Language } from "./i18n";
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
  CustomGroupsText: string;
  ManagedConfigEnabled: boolean;
  ManagedConfigIntervalSeconds: number;
  ManagedConfigStrict: boolean;
};

type RuleMergeMode = "custom_first" | "upstream_first" | "custom_first_dedupe" | "upstream_first_dedupe";

type RuleConfig = {
  CustomRulesText: string;
  RuleMergeMode: RuleMergeMode;
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

function App() {
  const [loggedIn, setLoggedIn] = React.useState(false);
  const [checkingSession, setCheckingSession] = React.useState(true);
  const [tab, setTab] = React.useState("overview");
  const [language, setLanguage] = React.useState<Language>(() => getInitialLanguage(localStorage.getItem(languageStorageKey)));
  const [tasks, setTasks] = React.useState<Task[]>([]);
  const [nodes, setNodes] = React.useState<PinnedNode[]>([]);
  const [ruleConfig, setRuleConfig] = React.useState<RuleConfig>({ CustomRulesText: "", RuleMergeMode: "custom_first" });
  const [error, setError] = React.useState("");
  const [toast, setToast] = React.useState("");
  const [busyTaskID, setBusyTaskID] = React.useState<number | null>(null);
  const [previewContent, setPreviewContent] = React.useState("");
  const t = translations[language];

  function changeLanguage(nextLanguage: Language) {
    setLanguage(nextLanguage);
    localStorage.setItem(languageStorageKey, nextLanguage);
  }

  React.useEffect(() => {
    let cancelled = false;
    api("/api/me")
      .then(async () => {
        if (cancelled) {
          return;
        }
        setLoggedIn(true);
        await refresh();
      })
      .catch(() => {
        if (!cancelled) {
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
    const [nextTasks, nextNodes, nextRuleConfig] = await Promise.all([
      api<Task[]>("/api/tasks").catch(() => []),
      api<PinnedNode[]>("/api/nodes").catch(() => []),
      api<RuleConfig>("/api/rule-config").catch(() => ({ CustomRulesText: "", RuleMergeMode: "custom_first" as RuleMergeMode })),
    ]);
    setTasks(nextTasks);
    setNodes(nextNodes);
    setRuleConfig(nextRuleConfig);
    if (showToast) notify(t.refreshed);
  }

  async function login(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setError("");
    const form = new FormData(event.currentTarget);
    try {
      await api("/api/login", { method: "POST", body: JSON.stringify(Object.fromEntries(form)) });
      setLoggedIn(true);
      await refresh();
    } catch {
      setError(t.loginFailed);
    }
  }

  async function createTask(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setError("");
    const form = new FormData(event.currentTarget);
    const payload = Object.fromEntries(form);
    payload.refresh_interval_seconds = Number(payload.refresh_interval_seconds || 3600) as unknown as FormDataEntryValue;
    try {
      await api("/api/tasks", { method: "POST", body: JSON.stringify(payload) });
      event.currentTarget.reset();
      await refresh();
      notify(t.taskList.saved);
    } catch {
      setError(t.createFailed);
      notify(t.createFailed);
    }
  }

  async function importNodes(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    try {
      const payload = Object.fromEntries(new FormData(event.currentTarget));
      await api("/api/nodes/import", { method: "POST", body: JSON.stringify(payload) });
      event.currentTarget.reset();
      await refresh();
      notify(t.refreshed);
    } catch {
      notify(t.createFailed);
    }
  }

  async function updateTask(id: number, input: TaskUpdateInput) {
    setBusyTaskID(id);
    try {
      await api(`/api/tasks/${id}`, { method: "PATCH", body: JSON.stringify(input) });
      await refresh();
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
          rule_merge_mode: String(form.get("rule_merge_mode") || "custom_first"),
        }),
      });
      setRuleConfig(next);
      notify(t.settings.ruleConfigSaved);
    } catch {
      notify(t.settings.ruleConfigSaveFailed);
    }
  }

  async function deleteTask(id: number) {
    if (!window.confirm(t.confirmDelete)) return;
    setBusyTaskID(id);
    try {
      await api(`/api/tasks/${id}`, { method: "DELETE" });
      await refresh();
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
      const result = await api<{ content: string }>(`/api/tasks/${id}/generate`, { method: "POST" });
      setPreviewContent(result.content);
      setTab("preview");
      await refresh();
      notify(t.taskList.generated);
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
      const result = await api<{ content: string }>(`/api/tasks/${id}/preview`);
      setPreviewContent(result.content);
      setTab("preview");
      await refresh();
      notify(t.preview.loaded);
    } catch {
      notify(t.preview.loadFailed);
    } finally {
      setBusyTaskID(null);
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
            <button onClick={() => refresh(true)}><RefreshCw size={16} /> {t.refresh}</button>
          </div>
        </header>
        {toast && <div className="toast">{toast}</div>}

        {tab === "overview" && (
          <section className="stack">
            <div className="metrics">
              <Metric icon={<Server />} value={tasks.length} label={t.metrics.tasks} />
              <Metric icon={<ShieldCheck />} value={nodes.length} label={t.metrics.pinnedNodes} />
              <Metric icon={<Layers />} value={tasks.filter((task) => task.IncludeGlobalRules).length} label={t.metrics.globalRules} />
              <Metric icon={<AlertCircle />} value={errors.length} label={t.metrics.errors} />
            </div>
            <TaskList tasks={tasks} t={t} busyTaskID={busyTaskID} onCopy={notify} onUpdate={updateTask} onDelete={deleteTask} onGenerate={generateTask} onPreview={loadPreview} />
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
            <TaskList tasks={tasks} t={t} busyTaskID={busyTaskID} onCopy={notify} onUpdate={updateTask} onDelete={deleteTask} onGenerate={generateTask} onPreview={loadPreview} />
          </section>
        )}

        {tab === "nodes" && (
          <section className="stack">
            <form className="panel" onSubmit={importNodes}>
              <label>{t.forms.proxyURIs}<textarea name="text" rows={6} placeholder={t.placeholders.proxyURIs} /></label>
              <button><Plus size={16} /> {t.forms.importNodes}</button>
            </form>
            <NodeList nodes={nodes} t={t} />
          </section>
        )}

        {tab === "preview" && <section className="panel"><h3>{t.preview.title}</h3><pre>{previewContent || t.preview.empty}</pre></section>}
        {tab === "settings" && (
          <section className="stack">
            <form className="panel settings-form" onSubmit={updateRuleConfig}>
              <h3>{t.settings.globalRuleTitle}</h3>
              <label>{t.forms.ruleMergeMode}<select name="rule_merge_mode" defaultValue={ruleConfig.RuleMergeMode}>{ruleMergeOptions(t)}</select></label>
              <label className="wide-field">{t.forms.customRules}<textarea name="custom_rules_text" rows={8} defaultValue={ruleConfig.CustomRulesText} placeholder={t.placeholders.customRules} /></label>
              <button><Save size={15} /> {t.forms.save}</button>
            </form>
            <section className="panel grid-form"><label>{t.settings.cachePolicy}<input readOnly value={t.settings.cachePolicyValue} /></label><label>{t.settings.vlessHelper}<input readOnly value={t.settings.vlessHelperValue} /></label></section>
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
  custom_groups_text?: string;
  managed_config_enabled?: boolean;
  managed_config_interval_seconds?: number;
  managed_config_strict?: boolean;
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
  busyTaskID,
  onCopy,
  onUpdate,
  onDelete,
  onGenerate,
  onPreview,
}: {
  tasks: Task[];
  t: typeof translations[Language];
  busyTaskID: number | null;
  onCopy: (message: string) => void;
  onUpdate: (id: number, input: TaskUpdateInput) => Promise<void>;
  onDelete: (id: number) => Promise<void>;
  onGenerate: (id: number) => Promise<void>;
  onPreview: (id: number) => Promise<void>;
}) {
  return (
    <section className="panel">
      <h3>{t.taskList.title}</h3>
      <div className="list">
        {tasks.length ? tasks.map((task) => (
          <TaskRow key={task.ID} task={task} t={t} busy={busyTaskID === task.ID} onCopy={onCopy} onUpdate={onUpdate} onDelete={onDelete} onGenerate={onGenerate} onPreview={onPreview} />
        )) : <p className="muted">{t.taskList.empty}</p>}
      </div>
    </section>
  );
}

function TaskRow({
  task,
  t,
  busy,
  onCopy,
  onUpdate,
  onDelete,
  onGenerate,
  onPreview,
}: {
  task: Task;
  t: typeof translations[Language];
  busy: boolean;
  onCopy: (message: string) => void;
  onUpdate: (id: number, input: TaskUpdateInput) => Promise<void>;
  onDelete: (id: number) => Promise<void>;
  onGenerate: (id: number) => Promise<void>;
  onPreview: (id: number) => Promise<void>;
}) {
  const [editing, setEditing] = React.useState(false);
  const status = taskStatus(task, busy, t);

  async function copyURL() {
    await navigator.clipboard.writeText(absoluteSubscriptionURL(task.SubscriptionURL));
    onCopy(t.copied);
  }

  async function save(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const form = new FormData(event.currentTarget);
    await onUpdate(task.ID, {
      name: String(form.get("name") || ""),
      source_url: String(form.get("source_url") || ""),
      refresh_interval_seconds: Number(form.get("refresh_interval_seconds") || 3600),
      enabled: form.get("enabled") === "on",
      merge_default_pinned_nodes: form.get("merge_default_pinned_nodes") === "on",
      include_global_rules: form.get("include_global_rules") === "on",
      custom_rules_text: String(form.get("custom_rules_text") || ""),
      rule_merge_mode: String(form.get("rule_merge_mode") || "custom_first") as RuleMergeMode,
      custom_groups_text: String(form.get("custom_groups_text") || ""),
      managed_config_enabled: form.get("managed_config_enabled") === "on",
      managed_config_interval_seconds: Number(form.get("managed_config_interval_seconds") || 86400),
      managed_config_strict: form.get("managed_config_strict") === "on",
    });
    setEditing(false);
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
        <form className="edit-form" onSubmit={save}>
          <label>{t.forms.name}<input name="name" defaultValue={task.Name} /></label>
          <label>{t.forms.clashURL}<input name="source_url" defaultValue={task.SourceURL} /></label>
          <label>{t.forms.refreshSeconds}<input name="refresh_interval_seconds" type="number" defaultValue={task.RefreshIntervalSeconds} /></label>
          <label className="check-label"><input name="enabled" type="checkbox" defaultChecked={task.Enabled} /> {t.forms.enabled}</label>
          <label className="check-label"><input name="merge_default_pinned_nodes" type="checkbox" defaultChecked={task.MergeDefaultPinnedNodes} /> {t.forms.mergeDefaults}</label>
          <label className="check-label"><input name="include_global_rules" type="checkbox" defaultChecked={task.IncludeGlobalRules} /> {t.forms.includeGlobalRules}</label>
          <label>{t.forms.ruleMergeMode}<select name="rule_merge_mode" defaultValue={task.RuleMergeMode}>{ruleMergeOptions(t)}</select></label>
          <label className="check-label"><input name="managed_config_enabled" type="checkbox" defaultChecked={task.ManagedConfigEnabled} /> {t.forms.managedConfig}</label>
          <label>{t.forms.managedInterval}<input name="managed_config_interval_seconds" type="number" min="60" defaultValue={task.ManagedConfigIntervalSeconds || 86400} /></label>
          <label className="check-label"><input name="managed_config_strict" type="checkbox" defaultChecked={task.ManagedConfigStrict} /> {t.forms.managedStrict}</label>
          <label className="wide-field">{t.forms.customRules}<textarea name="custom_rules_text" rows={5} defaultValue={task.CustomRulesText} placeholder={t.placeholders.customRules} /></label>
          <label className="wide-field">{t.forms.customGroups}<textarea name="custom_groups_text" rows={4} defaultValue={task.CustomGroupsText} placeholder={t.placeholders.customGroups} /></label>
          <button disabled={busy}><Save size={15} /> {busy ? t.saving : t.forms.save}</button>
          <button type="button" onClick={() => setEditing(false)}><X size={15} /> {t.forms.cancel}</button>
        </form>
      )}
    </div>
  );
}

function ruleMergeOptions(t: typeof translations[Language]) {
  return (["custom_first", "upstream_first", "custom_first_dedupe", "upstream_first_dedupe"] as RuleMergeMode[]).map((mode) => (
    <option key={mode} value={mode}>{t.ruleMergeModes[mode]}</option>
  ));
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

function NodeList({ nodes, t }: { nodes: PinnedNode[]; t: typeof translations[Language] }) {
  return (
    <section className="panel">
      <h3>{t.nodeList.title}</h3>
      <div className="list">
        {nodes.length ? nodes.map((node) => (
          <div className="row" key={node.ID}>
            <div>
              <strong>{node.Name}</strong>
              <span>{`${node.Server}:${node.Port}`}</span>
            </div>
            <span className="badge">{node.Protocol}</span>
            <span>{node.DefaultInclude ? t.nodeList.default : t.nodeList.manual}</span>
            <span>{node.Enabled ? t.nodeList.enabled : t.nodeList.disabled}</span>
          </div>
        )) : <p className="muted">{t.nodeList.empty}</p>}
      </div>
    </section>
  );
}

function absoluteSubscriptionURL(url: string) {
  if (url.startsWith("http://") || url.startsWith("https://")) {
    return url;
  }
  return `${location.origin}${url.startsWith("/") ? "" : "/"}${url}`;
}

createRoot(document.getElementById("root")!).render(<App />);
