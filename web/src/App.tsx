import { useEffect, useState } from "react";

type Health = { status: string; database: string };

export default function App() {
  const [health, setHealth] = useState<Health | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    fetch("/healthz")
      .then((r) => r.json())
      .then(setHealth)
      .catch((e) => setError(String(e)));
  }, []);

  return (
    <main className="min-h-screen bg-slate-50 flex items-center justify-center p-6">
      <div className="max-w-lg w-full bg-white rounded-2xl shadow-sm border border-slate-200 p-8 space-y-4">
        <h1 className="text-2xl font-semibold text-slate-900">Mission — control plane</h1>
        <p className="text-sm text-slate-600">
          Week 1.1 hello-world. Frontend talks to the Go backend at
          <code className="mx-1 px-1.5 py-0.5 rounded bg-slate-100 text-slate-800">/healthz</code>.
        </p>
        <div className="rounded-lg bg-slate-100 p-4 font-mono text-xs text-slate-800">
          {error ? (
            <span className="text-rose-700">error: {error}</span>
          ) : health ? (
            <pre>{JSON.stringify(health, null, 2)}</pre>
          ) : (
            <span className="text-slate-500">loading…</span>
          )}
        </div>
      </div>
    </main>
  );
}
