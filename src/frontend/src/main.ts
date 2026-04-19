import "./style.css";

const app = document.querySelector<HTMLDivElement>("#app");
const defaultBackendUrl = "http://localhost:8000";
const configuredBackendUrl = import.meta.env.VITE_BACKEND_URL ?? defaultBackendUrl;
const backendUrl = configuredBackendUrl.endsWith("/")
    ? configuredBackendUrl.slice(0, -1)
    : configuredBackendUrl;

if (!app) {
    throw new Error("Frontend app root was not found.");
}

app.innerHTML = `
  <main class="page-shell">
    <section class="hero-card">
      <p class="eyebrow">Frontend sample</p>
      <h1>Bun + Vite + TypeScript starter</h1>
      <p class="lede">
        This demo page is the browser-facing companion to the FastAPI template.
        Keep the existing document workflow in <code>src/backend</code> and use
        <code>src/frontend</code> as a lightweight starting point for UI work.
      </p>
      <div class="hero-actions">
        <a href="${backendUrl}/" target="_blank" rel="noreferrer">
          Open FastAPI sample
        </a>
        <a href="${backendUrl}/health" target="_blank" rel="noreferrer">
          Check backend health
        </a>
      </div>
    </section>

    <section class="info-grid" aria-label="Template starter details">
      <article class="info-card">
        <p class="card-label">Frontend code</p>
        <h2>src/frontend</h2>
        <p>
          Single-page starter built with Bun, Vite, and TypeScript. The page is
          intentionally domain-neutral so contributors can replace it with their
          own UI without undoing template assumptions.
        </p>
      </article>

      <article class="info-card">
        <p class="card-label">Backend sample</p>
        <h2>src/backend</h2>
        <p>
          FastAPI still owns the existing document CRUD example, health check,
          and repository layering. Nothing in this frontend starter changes that
          sample flow.
        </p>
      </article>

      <article class="info-card">
        <p class="card-label">Local commands</p>
        <h2>Start and build</h2>
        <p><code>bun install</code></p>
        <p><code>bun run dev</code></p>
        <p><code>bun run build</code></p>
      </article>
    </section>
  </main>
`;
