# Google Cloud Single-Origin Deployment

This repo maps cleanly to one public Cloud Run service that serves both the built SPA and the Go control-plane API, backed privately by Cloud SQL for PostgreSQL 16.

That production shape mirrors the current local stack in [docker-compose.yaml](./docker-compose.yaml):

- `web` is a dev server only; in production its built assets are bundled into the control-plane image
- `control-plane` remains the only public application service
- `postgres` remains private to the backend
- `mysql` stays local/dev-only and is not part of the Google Cloud control-plane deployment

## Recommended topology

- Build a single container image from [Dockerfile.control-plane](./Dockerfile.control-plane).
- Deploy that image to Cloud Run with end-to-end HTTP/2 enabled.
- Serve browser traffic, static assets, Connect-RPC, `/api/*`, and `/healthz` from the same Cloud Run origin.
- Run Cloud SQL for PostgreSQL 16 in the same region as Cloud Run.
- Connect Cloud Run to Cloud SQL over private IP using Direct VPC egress.
- Keep Cloud Run at `min-instances=1` and `max-instances=1` for now because edge-agent sessions are stored in process memory.

This single-origin setup preserves the frontend’s current same-origin transport model, so there is no CORS or explicit API base URL work.

## Prerequisites

- A Google Cloud project with billing enabled.
- `gcloud` installed and authenticated.
- A Google Cloud region selected for both Cloud Run and Cloud SQL.
- Clerk credentials for production auth.
- At least one LLM provider key.
- Docker installed locally for image builds.

Enable the required APIs:

```bash
gcloud services enable \
  artifactregistry.googleapis.com \
  cloudbuild.googleapis.com \
  run.googleapis.com \
  sqladmin.googleapis.com \
  compute.googleapis.com \
  servicenetworking.googleapis.com
```

## 1. Create Artifact Registry

```bash
export PROJECT_ID="your-project"
export REGION="us-central1"
export REPOSITORY="mission"

gcloud artifacts repositories create "${REPOSITORY}" \
  --project "${PROJECT_ID}" \
  --repository-format docker \
  --location "${REGION}" \
  --description "Mission application images"
```

Example image target:

```bash
export IMAGE_URL="${REGION}-docker.pkg.dev/${PROJECT_ID}/${REPOSITORY}/control-plane:$(git rev-parse --short HEAD)"
```

## 2. Create Cloud SQL For PostgreSQL

Create PostgreSQL 16 in the same region as Cloud Run and attach a private IP address.

```bash
export INSTANCE_NAME="mission-db"
export DB_NAME="mission"
export DB_USER="mission"

gcloud sql instances create "${INSTANCE_NAME}" \
  --project "${PROJECT_ID}" \
  --database-version=POSTGRES_16 \
  --region "${REGION}" \
  --network=projects/${PROJECT_ID}/global/networks/default \
  --no-assign-ip

gcloud sql databases create "${DB_NAME}" \
  --project "${PROJECT_ID}" \
  --instance "${INSTANCE_NAME}"

gcloud sql users create "${DB_USER}" \
  --project "${PROJECT_ID}" \
  --instance "${INSTANCE_NAME}" \
  --password 'REPLACE_ME'
```

Get the private IP address:

```bash
gcloud sql instances describe "${INSTANCE_NAME}" \
  --project "${PROJECT_ID}" \
  --format='value(ipAddresses[0].ipAddress)'
```

Use that private IP in `DATABASE_URL`. For this codebase, the simplest direct private-IP form is:

```text
postgres://DB_USER:DB_PASSWORD@CLOUD_SQL_PRIVATE_IP:5432/DB_NAME?sslmode=disable
```

The application still uses the same `DATABASE_URL` contract as local compose. Only the hostname changes from `postgres` to the Cloud SQL private IP.

## 3. Build And Push The Single-Origin Image

The control-plane Dockerfile now:

- builds `web/dist`
- builds the Go binary
- copies both into the final distroless image

Build-time frontend values are supplied with Docker build args:

- `VITE_CLERK_PUBLISHABLE_KEY`
- `VITE_SENTRY_DSN`
- `VITE_SENTRY_ENVIRONMENT`
- `VITE_SENTRY_RELEASE`

Build and push:

```bash
gcloud auth configure-docker "${REGION}-docker.pkg.dev"

docker build \
  -f Dockerfile.control-plane \
  -t "${IMAGE_URL}" \
  --build-arg VITE_CLERK_PUBLISHABLE_KEY="pk_live_replace_me" \
  --build-arg VITE_SENTRY_DSN="" \
  --build-arg VITE_SENTRY_ENVIRONMENT="production" \
  --build-arg VITE_SENTRY_RELEASE="$(git rev-parse --short HEAD)" \
  .

docker push "${IMAGE_URL}"
```

## 4. Prepare Runtime Identity And Networking

Create a dedicated service account for Cloud Run:

```bash
export SERVICE_ACCOUNT="mission-control-plane@${PROJECT_ID}.iam.gserviceaccount.com"

gcloud iam service-accounts create mission-control-plane \
  --project "${PROJECT_ID}" \
  --display-name "Mission control-plane"
```

Grant it the minimum role needed for Cloud SQL API access and operational logging:

```bash
gcloud projects add-iam-policy-binding "${PROJECT_ID}" \
  --member "serviceAccount:${SERVICE_ACCOUNT}" \
  --role roles/cloudsql.client
```

For private IP connectivity, attach Cloud Run to a VPC/subnet with Direct VPC egress. The checked-in sample manifest at [deploy/cloudrun/control-plane.yaml](./deploy/cloudrun/control-plane.yaml) uses:

- `run.googleapis.com/network-interfaces`
- `run.googleapis.com/vpc-access-egress: private-ranges-only`

`private-ranges-only` is the recommended default here because:

- Cloud SQL private IP traffic goes through the VPC
- internet egress to Clerk, Anthropic, OpenAI, and Sentry continues to work without forcing all egress through Cloud NAT

## 5. Deploy Cloud Run

You can deploy directly with `gcloud` or replace the checked-in service YAML.

### Option A: deploy from CLI

```bash
export CONTROL_PLANE_DOMAIN="https://YOUR_CLOUD_RUN_URL_OR_CUSTOM_DOMAIN"
export DATABASE_URL="postgres://DB_USER:DB_PASSWORD@CLOUD_SQL_PRIVATE_IP:5432/DB_NAME?sslmode=disable"

gcloud run deploy control-plane \
  --project "${PROJECT_ID}" \
  --region "${REGION}" \
  --image "${IMAGE_URL}" \
  --service-account "${SERVICE_ACCOUNT}" \
  --allow-unauthenticated \
  --use-http2 \
  --timeout 3600 \
  --concurrency 10 \
  --min-instances 1 \
  --max-instances 1 \
  --network default \
  --subnet default \
  --vpc-egress private-ranges-only \
  --set-env-vars "ENV=production,LOG_LEVEL=info,DATABASE_URL=${DATABASE_URL},PUBLIC_CONTROL_PLANE_URL=${CONTROL_PLANE_DOMAIN},DEFAULT_LLM_PROVIDER=anthropic,SEMANTIC_LAYER_MODEL=claude-sonnet-4-6,QUERY_MODEL=claude-sonnet-4-6,EDGE_AGENT_IMAGE=${REGION}-docker.pkg.dev/${PROJECT_ID}/${REPOSITORY}/edge-agent:TAG,SENTRY_ENVIRONMENT=production" \
  --set-secrets "CLERK_SECRET_KEY=CLERK_SECRET_KEY:latest,ANTHROPIC_API_KEY=ANTHROPIC_API_KEY:latest,OPENAI_API_KEY=OPENAI_API_KEY:latest,SENTRY_DSN=SENTRY_DSN:latest"
```

### Option B: replace the sample YAML

Start from [deploy/cloudrun/control-plane.yaml](./deploy/cloudrun/control-plane.yaml), replace the placeholders, then apply:

```bash
gcloud run services replace deploy/cloudrun/control-plane.yaml \
  --project "${PROJECT_ID}" \
  --region "${REGION}"
```

## 6. Public URL And Edge-Agent Behavior

After deploy, get the Cloud Run URL:

```bash
gcloud run services describe control-plane \
  --project "${PROJECT_ID}" \
  --region "${REGION}" \
  --format='value(status.url)'
```

Set `PUBLIC_CONTROL_PLANE_URL` to that URL or to your custom domain before the production deploy. The onboarding flow uses it to render the tenant-facing edge-agent install command.

This app’s query and onboarding handlers use 90-second application timeouts, but the Cloud Run service timeout is intentionally set to 3600 seconds because the edge-agent command stream is long-lived. The agent already reconnects when that stream ends, so a 60-minute Cloud Run cap is acceptable for the current design.

## 7. Why Cloud Run Is Single-Instance For Now

Keep these settings for v1:

- `min-instances=1`
- `max-instances=1`
- `concurrency=10`

The control-plane currently stores active edge-agent sessions in memory. Multiple Cloud Run instances would allow follow-up requests to land on a different instance than the one holding the open agent stream. Until session state is externalized, single-instance deployment is the safe production posture.

## Runtime Interaction Model

Once deployed:

- the browser loads the SPA from the same Cloud Run origin
- static assets are served from the bundled `web/dist`
- Clerk JWTs are sent by the browser to same-origin backend endpoints
- the Go control-plane verifies those JWTs with `CLERK_SECRET_KEY`
- the control-plane talks privately to Cloud SQL through `DATABASE_URL`
- edge agents connect back to the same public control-plane URL for `agent.v1.AgentService`

## Verification Checklist

After the first production deploy:

1. Open the app root and confirm the SPA loads.
2. Refresh a deep link such as `/chat` or `/onboarding/<tenant-id>/step-2` and confirm it does not 404.
3. Open `https://YOUR_DOMAIN/healthz` and confirm the response reports `{"status":"ok","database":"ok"}`.
4. Sign in with Clerk and verify authenticated pages can call the backend.
5. Create or retrieve an onboarding token and start an edge agent against `PUBLIC_CONTROL_PLANE_URL`.
6. Confirm the onboarding flow detects the live agent session.
7. Configure the tenant database and capture schema.
8. Run a test query and confirm the control-plane still reaches the connected agent successfully.

## References

- [Cloud Run HTTP/2 configuration](https://cloud.google.com/run/docs/configuring/http2)
- [Cloud Run request timeouts](https://cloud.google.com/run/docs/configuring/request-timeout)
- [Cloud Run minimum instances](https://cloud.google.com/run/docs/configuring/min-instances)
- [Cloud Run maximum instances](https://cloud.google.com/run/docs/configuring/max-instances)
- [Cloud Run concurrency](https://cloud.google.com/run/docs/configuring/concurrency)
- [Cloud Run direct VPC egress](https://cloud.google.com/run/docs/configuring/vpc-direct-vpc)
- [Cloud SQL from Cloud Run](https://cloud.google.com/sql/docs/postgres/connect-run)
