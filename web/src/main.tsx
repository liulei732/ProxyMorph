import React from "react";
import { createRoot } from "react-dom/client";
import { AlertCircle, Copy, KeyRound, Plus, RefreshCw, Server, ShieldCheck } from "lucide-react";
import { api } from "./api";
import "./styles.css";

type Task = {
  ID: number;
  Name: string;
  InputType: string;
  OutputType: string;
  SourceURL: string;
  Enabled: boolean;
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
  const [tab, setTab] = React.useState("overview");
  const [tasks, setTasks] = React.useState<Task[]>([]);
  const [nodes, setNodes] = React.useState<PinnedNode[]>([]);
  const [error, setError] = React.useState("");

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
      setError("Login failed.");
    }
  }

  async function createTask(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const form = new FormData(event.currentTarget);
    const payload = Object.fromEntries(form);
    payload.refresh_interval_seconds = Number(payload.refresh_interval_seconds || 3600) as unknown as FormDataEntryValue;
    await api("/api/tasks", { method: "POST", body: JSON.stringify(payload) });
    event.currentTarget.reset();
    await refresh();
  }

  async function importNodes(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const payload = Object.fromEntries(new FormData(event.currentTarget));
    await api("/api/nodes/import", { method: "POST", body: JSON.stringify(payload) });
    event.currentTarget.reset();
    await refresh();
  }

  if (!loggedIn) {
    return (
      <main className="login">
        <section className="identity">
          <div className="logo">PM</div>
          <div>
            <h1>ProxyMorph</h1>
            <p>Convert Clash subscriptions, compose fixed nodes, ship Surge 6 output.</p>
          </div>
        </section>
        <form className="panel login-panel" onSubmit={login}>
          <label>Username<input name="username" defaultValue="admin" autoComplete="username" /></label>
          <label>Password<input name="password" type="password" autoComplete="current-password" /></label>
          <button><KeyRound size={17} /> Sign in</button>
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
            <button key={item} className={tab === item ? "active" : ""} onClick={() => setTab(item)}>{item}</button>
          ))}
        </nav>
      </aside>
      <section className="workspace">
        <header className="topbar">
          <div>
            <h2>{title(tab)}</h2>
            <p>{subtitle(tab)}</p>
          </div>
          <button onClick={refresh}><RefreshCw size={16} /> Refresh</button>
        </header>

        {tab === "overview" && (
          <section className="stack">
            <div className="metrics">
              <Metric icon={<Server />} value={tasks.length} label="Tasks" />
              <Metric icon={<ShieldCheck />} value={nodes.length} label="Pinned Nodes" />
              <Metric icon={<AlertCircle />} value={errors.length} label="Errors" />
            </div>
            <TaskList tasks={tasks} />
          </section>
        )}

        {tab === "tasks" && (
          <section className="stack">
            <form className="panel grid-form" onSubmit={createTask}>
              <label>Name<input name="name" placeholder="Main subscription" /></label>
              <label>Clash URL<input name="source_url" placeholder="https://example.com/clash.yaml" /></label>
              <label>Refresh seconds<input name="refresh_interval_seconds" type="number" defaultValue="3600" /></label>
              <button><Plus size={16} /> Create</button>
            </form>
            <TaskList tasks={tasks} />
          </section>
        )}

        {tab === "nodes" && (
          <section className="stack">
            <form className="panel" onSubmit={importNodes}>
              <label>Proxy URIs<textarea name="text" rows={6} placeholder="vless://uuid@example.com:443?security=tls&sni=edge.example.com#Edge" /></label>
              <button><Plus size={16} /> Import Nodes</button>
            </form>
            <NodeList nodes={nodes} />
          </section>
        )}

        {tab === "preview" && <section className="panel"><h3>Conversion Preview</h3><pre>No preview loaded.</pre></section>}
        {tab === "settings" && <section className="panel grid-form"><label>Cache Policy<input readOnly value="Refresh on request with last-good fallback" /></label><label>VLESS Helper<input readOnly value="sing-box bundled in Docker" /></label></section>}
      </section>
    </main>
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

function TaskList({ tasks }: { tasks: Task[] }) {
  return (
    <section className="panel">
      <h3>Conversion Tasks</h3>
      <div className="list">
        {tasks.length ? tasks.map((task) => (
          <div className="row" key={task.ID}>
            <div>
              <strong>{task.Name}</strong>
              <span>{task.SourceURL}</span>
            </div>
            <span className="badge">{`${task.InputType} to ${task.OutputType}`}</span>
            <span>{task.Enabled ? "Enabled" : "Disabled"}</span>
            <button onClick={() => navigator.clipboard.writeText(`${location.origin}/sub/token-${task.ID}`)}>
              <Copy size={15} /> Copy
            </button>
          </div>
        )) : <p className="muted">No conversion tasks yet.</p>}
      </div>
    </section>
  );
}

function NodeList({ nodes }: { nodes: PinnedNode[] }) {
  return (
    <section className="panel">
      <h3>Pinned Node Library</h3>
      <div className="list">
        {nodes.length ? nodes.map((node) => (
          <div className="row" key={node.ID}>
            <div>
              <strong>{node.Name}</strong>
              <span>{`${node.Server}:${node.Port}`}</span>
            </div>
            <span className="badge">{node.Protocol}</span>
            <span>{node.DefaultInclude ? "Default" : "Manual"}</span>
            <span>{node.Enabled ? "Enabled" : "Disabled"}</span>
          </div>
        )) : <p className="muted">No pinned nodes yet.</p>}
      </div>
    </section>
  );
}

function title(tab: string) {
  return ({ overview: "Overview", tasks: "Tasks", nodes: "Pinned Nodes", preview: "Preview", settings: "Settings" } as Record<string, string>)[tab];
}

function subtitle(tab: string) {
  return ({ overview: "Track conversion health.", tasks: "Manage Clash to Surge 6 outputs.", nodes: "Import fixed SS, Trojan, and VLESS nodes.", preview: "Inspect generated output.", settings: "Runtime and deployment details." } as Record<string, string>)[tab];
}

createRoot(document.getElementById("root")!).render(<App />);
