# DigitalOcean Deployment

This repo maps cleanly to a single DigitalOcean App Platform app with three pieces:

1. `control-plane` as an App Platform web service.
2. `web` as an App Platform static site.
3. `mission-db` as a Postgres database component or an external DigitalOcean Managed PostgreSQL cluster.

The edge agent is not deployed on App Platform. It runs on each tenant's infrastructure, but you can publish its Docker image to DigitalOcean Container Registry and point onboarding at that image with `EDGE_AGENT_IMAGE`.

## Recommended topology

- Keep the frontend and control-plane on the same App Platform app and same origin.
- Use path-based ingress rules to send `/api`, `/healthz`, and Connect-RPC service paths to the control-plane.
- Route `/` to the static site.
- Use a managed Postgres instance in the same region as the app.
- Store `CLERK_SECRET_KEY`, `ANTHROPIC_API_KEY`, and `OPENAI_API_KEY` as encrypted App Platform environment variables.

Repo note: the frontend currently uses relative `/healthz`, `/api`, and Connect-RPC paths, so a same-origin deployment is the lowest-friction production setup.

## Prerequisites

- A DigitalOcean account and project.
- A GitHub repo connected to App Platform, or a workflow that pushes images to DigitalOcean Container Registry.
- `doctl` installed locally if you want to manage the app spec from the CLI.
- Clerk credentials if auth is enabled in production.
- At least one LLM provider key for semantic-layer drafting and question answering.

## Build and publish the edge-agent image

Use an immutable tag instead of `latest` for production rollouts.

```bash
doctl auth init
doctl registry login

EDGE_AGENT_IMAGE_REPOSITORY=registry.digitalocean.com/<registry>/edge-agent \
EDGE_AGENT_VERSION=v0.1.0 \
./scripts/release_edge_agent.sh
```

Set `EDGE_AGENT_IMAGE_REPOSITORY=registry.digitalocean.com/<registry>/edge-agent` and `EDGE_AGENT_VERSION=v0.1.0` on the control-plane service so onboarding shows the pinned install image. `EDGE_AGENT_IMAGE` still works as a legacy full-image override when needed.

## Create the App Platform components

### Control-plane service

- Component type: `Web Service`
- Source: this repo
- Source directory: `/`
- Dockerfile path: `Dockerfile.control-plane`
- HTTP port: `8080`
- Protocol: `HTTP2`
- Health check path: `/healthz`

`HTTP2` matters here because the control-plane serves Connect-RPC traffic and the agent command stream uses server-streaming over h2c.

### Web static site

- Component type: `Static Site`
- Source: this repo
- Source directory: `web`
- Build command: `npm ci && npm run build`
- Output directory: `dist`

## Environment variables

Set these on the `control-plane` service:

- `ENV=production`
- `HTTP_PORT=8080`
- `LOG_LEVEL=info`
- `DATABASE_URL=${mission-db.DATABASE_PRIVATE_URL}` if the database is an App Platform database component or a managed database in the same VPC
- `CLERK_SECRET_KEY=<secret>`
- `ANTHROPIC_API_KEY=<secret>` or `OPENAI_API_KEY=<secret>`
- `DEFAULT_LLM_PROVIDER=anthropic` or `openai`
- `SEMANTIC_LAYER_MODEL=<model-name>`
- `QUERY_MODEL=<model-name>`
- `EDGE_AGENT_IMAGE=registry.digitalocean.com/<registry>/edge-agent:<tag>`

Set this on the `web` static site as a build-time variable:

- `VITE_CLERK_PUBLISHABLE_KEY=<publishable-key>`

If you attach a database component directly to the app, App Platform can inject bindable variables such as `${mission-db.DATABASE_URL}` and `${mission-db.DATABASE_PRIVATE_URL}` for you.

## Ingress rules

Because the frontend expects same-origin API calls, route these prefixes to the `control-plane` component and preserve the full path:

- `/healthz`
- `/api`
- `/tenant.v1.TenantService`
- `/semantic.v1.SemanticLayerService`
- `/query.v1.QueryService`
- `/starter.v1.StarterQuestionsService`
- `/onboarding.v1.OnboardingService`
- `/agent.v1.AgentService`

Route `/` to the `web` static site.

The DigitalOcean control panel is fine for creating the components, but preserving full request paths for services is easiest via the app spec. A minimal ingress section looks like this:

```yaml
ingress:
  rules:
    - component:
        name: control-plane
        preserve_path_prefix: true
      match:
        path:
          prefix: /healthz
    - component:
        name: control-plane
        preserve_path_prefix: true
      match:
        path:
          prefix: /api
    - component:
        name: control-plane
        preserve_path_prefix: true
      match:
        path:
          prefix: /tenant.v1.TenantService
    - component:
        name: control-plane
        preserve_path_prefix: true
      match:
        path:
          prefix: /semantic.v1.SemanticLayerService
    - component:
        name: control-plane
        preserve_path_prefix: true
      match:
        path:
          prefix: /query.v1.QueryService
    - component:
        name: control-plane
        preserve_path_prefix: true
      match:
        path:
          prefix: /starter.v1.StarterQuestionsService
    - component:
        name: control-plane
        preserve_path_prefix: true
      match:
        path:
          prefix: /onboarding.v1.OnboardingService
    - component:
        name: control-plane
        preserve_path_prefix: true
      match:
        path:
          prefix: /agent.v1.AgentService
    - component:
        name: web
      match:
        path:
          prefix: /
```

If you create the app in the control panel first, download the generated app spec, add `protocol: HTTP2` and the ingress rules above, then apply it with:

```bash
doctl apps update <app-id> --spec app.yaml
```

## Database options

You can use either of these approaches:

1. Add a Postgres database component directly to the App Platform app and bind `DATABASE_URL` from it.
2. Create a standalone DigitalOcean Managed PostgreSQL cluster and set `DATABASE_URL` manually from that cluster's connection string.

For production, prefer a managed database with backups and keep it in the same region as the app.

## Custom domain and TLS

- Add your production domain in the App Platform Networking tab after the app is live.
- Keep the frontend and backend under the same app/domain so the current browser clients continue to use relative paths without CORS changes.

## Verification checklist

After the first deploy:

1. Open `https://<your-app-domain>/healthz` and confirm the service reports healthy status.
2. Open the app root and confirm the frontend loads.
3. Sign in through Clerk and verify authenticated pages can reach the control-plane.
4. Issue a tenant token and start an edge agent against `https://<your-app-domain>`.
5. Complete onboarding and run a test query against a tenant database.

## References

- [DigitalOcean App Platform: deploy from container images](https://docs.digitalocean.com/products/app-platform/how-to/deploy-from-container-images/)
- [DigitalOcean App Platform: use environment variables](https://docs.digitalocean.com/products/app-platform/how-to/use-environment-variables/)
- [DigitalOcean App Platform: configure HTTP/2](https://docs.digitalocean.com/products/app-platform/how-to/configure-http2/)
- [DigitalOcean App Platform: manage domains and routing rules](https://docs.digitalocean.com/products/app-platform/how-to/manage-domains/)
- [DigitalOcean App Platform: app spec reference](https://docs.digitalocean.com/products/app-platform/reference/app-spec/)
- [DigitalOcean Container Registry quickstart](https://docs.digitalocean.com/products/container-registry/getting-started/quickstart/)
