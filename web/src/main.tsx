import React from "react";
import { createRoot } from "react-dom/client";
import { AlertCircle, Copy, KeyRound, Plus, RefreshCw, Server, ShieldCheck } from "lucide-react";
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
  SubscriptionURL: string;
  LastErrorMessage: string;
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
  const [error, setError] = React.useState("");
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

  async function refresh() {
    const [nextTasks, nextNodes] = await Promise.all([
      api<Task[]>("/api/tasks").catch(() => []),
      api<PinnedNode[]>("/api/nodes").catch(() => []),
    ]);
    setTasks(nextTasks);
    setNodes(nextNodes);
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
    } catch {
      setError(t.createFailed);
    }
  }

  async function importNodes(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const payload = Object.fromEntries(new FormData(event.currentTarget));
    await api("/api/nodes/import", { method: "POST", body: JSON.stringify(payload) });
    event.currentTarget.reset();
    await refresh();
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
            <button onClick={refresh}><RefreshCw size={16} /> {t.refresh}</button>
          </div>
        </header>

        {tab === "overview" && (
          <section className="stack">
            <div className="metrics">
              <Metric icon={<Server />} value={tasks.length} label={t.metrics.tasks} />
              <Metric icon={<ShieldCheck />} value={nodes.length} label={t.metrics.pinnedNodes} />
              <Metric icon={<AlertCircle />} value={errors.length} label={t.metrics.errors} />
            </div>
            <TaskList tasks={tasks} t={t} />
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
            <TaskList tasks={tasks} t={t} />
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

        {tab === "preview" && <section className="panel"><h3>{t.preview.title}</h3><pre>{t.preview.empty}</pre></section>}
        {tab === "settings" && <section className="panel grid-form"><label>{t.settings.cachePolicy}<input readOnly value={t.settings.cachePolicyValue} /></label><label>{t.settings.vlessHelper}<input readOnly value={t.settings.vlessHelperValue} /></label></section>}
      </section>
    </main>
  );
}

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

function TaskList({ tasks, t }: { tasks: Task[]; t: typeof translations[Language] }) {
  return (
    <section className="panel">
      <h3>{t.taskList.title}</h3>
      <div className="list">
        {tasks.length ? tasks.map((task) => (
          <div className="row" key={task.ID}>
            <div>
              <strong>{task.Name}</strong>
              <span>{task.SourceURL}</span>
            </div>
            <span className="badge">{`${task.InputType} ${t.taskList.route} ${task.OutputType}`}</span>
            <span className={task.LastErrorMessage ? "status-error" : ""}>{task.LastErrorMessage ? t.taskList.failed : task.Enabled ? t.taskList.enabled : t.taskList.disabled}</span>
            <button onClick={() => navigator.clipboard.writeText(absoluteSubscriptionURL(task.SubscriptionURL))}>
              <Copy size={15} /> {t.taskList.copy}
            </button>
            {task.LastErrorMessage && <p className="row-error">{task.LastErrorMessage}</p>}
          </div>
        )) : <p className="muted">{t.taskList.empty}</p>}
      </div>
    </section>
  );
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
