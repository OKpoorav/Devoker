# Devoker

Texas Hold'em with friends: a Go backend and a React frontend. Delivered by a pipeline that runs
from `git push` to a running, monitored Kubernetes deployment.

On every push to `main`, GitHub Actions runs the tests and four kinds of security scan, then builds
both images and scans them again. Only images with no fixable HIGH/CRITICAL vulnerabilities reach
GHCR. The pipeline then commits the new image tag to the Helm chart, and Argo CD, running inside the
cluster, deploys it. Prometheus and Grafana watch the pods, and the frontend scales on CPU.

## Contents

[Architecture](#architecture) · [Technologies](#technologies) · [Run the app](#run-the-app) ·
[Docker](#docker) · [Kubernetes](#kubernetes) · [Helm](#helm) · [CI/CD](#cicd) ·
[DevSecOps](#devsecops) · [GitOps](#gitops) · [Monitoring](#monitoring) · [Terraform](#terraform) ·
[Troubleshooting](#troubleshooting) · [Screenshots](#screenshots) · [Lessons learned](#lessons-learned) ·
[API](#api)

## Architecture

```
┌──────────────────── Kubernetes (minikube) · namespace "devoker" ────────────────────┐
│                                                                                     │
│  browser ─► devoker.local ─► Ingress ─► Service frontend ─► frontend pods (nginx)   │
│                                                              2..5 replicas, HPA     │
│                                                              serves React build     │
│                                                              proxies /api/* ──┐     │
│                                                                               ▼     │
│                                         Service backend ─► backend pod (Go)         │
│                                                            exactly 1 replica        │
│                                                                                     │
│  ConfigMap (blinds, chips) ─► backend    Secret (GHCR pull) ─► both                 │
│  PVC (nginx access log)   ─► frontend                                               │
└─────────────────────────────────────────────────────────────────────────────────────┘

git push ─► GitHub Actions: tests ─► SAST · SCA · secrets ─► build ─► Trivy gate
                            ─► push to GHCR (:sha) ─► bot commits new tag to helm values.yaml
                                                                    │
Argo CD (inside the cluster) polls git, sees the new tag, syncs ◄────┘

Prometheus + Grafana (kube-prometheus-stack) watch pods, CPU, the HPA and two alert rules
Terraform (on the side) creates AWS ECR repos + an encrypted S3 bucket
```

### The constraint that shapes the design

Rooms, players, chips and the deck live in the memory of one Go process. There is no database.
Four decisions follow from that, and each was tested on the cluster (see [Troubleshooting](#troubleshooting)):

| Decision | Why |
|---|---|
| Backend runs **exactly 1 replica**, not a Helm value | With 2, the Service splits requests between pods and players get "room not found" ([incident 2](troubleshooting/02-split-brain.md)). |
| Backend uses **`strategy: Recreate`** | A rolling update briefly runs two backends with different tables. The cost is a short outage on deploy, and a bad probe becomes a full outage ([incident 4](troubleshooting/04-readiness-outage.md)). |
| The **HPA is on the frontend** (nginx), not the backend | nginx is stateless. The backend can't scale until state moves to Redis or a database. |
| Every proxy has **buffering off and a 1 h read timeout** | Players get live updates over Server-Sent Events, a long-lived HTTP response. The server sends a ping every 25 s. |

Deploying or restarting the backend ends every game in progress. A `PrometheusRule` alerts on it.

## Technologies

| Area | Tools |
|---|---|
| App | Go 1.26 (standard library only), React 19, Vite 8 |
| Containers | Docker multi-stage builds, distroless (backend), nginx-unprivileged (frontend), Docker Compose |
| Orchestration | Kubernetes (minikube), ingress-nginx, metrics-server, Helm 4 |
| CI/CD | GitHub Actions, GHCR |
| Security | gosec (SAST), govulncheck + npm audit + Trivy fs (SCA and misconfig), Gitleaks (secrets), Trivy image |
| GitOps | Argo CD |
| Monitoring | kube-prometheus-stack (Prometheus, Grafana, kube-state-metrics), PrometheusRule alerts, fortio load test |
| Infrastructure as code | Terraform with the AWS provider (ECR, S3) |

## Repository layout

| Folder | Contents |
|---|---|
| `backend/`, `frontend/` | The application and its tests (the brief's `application/` folder) |
| `docker/` | Both Dockerfiles, `nginx.conf`, `compose.yaml` |
| `kubernetes/` | Plain manifests, numbered in apply order |
| `helm/devoker/` | The Helm chart; `helm/values-local.yaml` for locally built images |
| `.github/workflows/ci.yaml` | The pipeline |
| `security/` | `.trivyignore` with the reason for each accepted finding |
| `gitops/` | The Argo CD `Application` |
| `monitoring/` | Stack values, alert rules, load test results |
| `terraform/` | AWS resources |
| `troubleshooting/` | Five incidents with real output |
| `scripts/smoke.py` | Plays one hand through the public API; used to verify every stage |

## Run the app

Without containers, for development:

```bash
cd backend && go run ./cmd/server               # :8080
cd frontend && npm install && npm run dev       # :5173, proxies /api to :8080
cd backend && go test -race -cover ./...        # poker engine ~95% coverage
```

Configuration is by environment variable: `PORT` (8080), `STARTING_CHIPS` (1000), `SMALL_BLIND` (10), `BIG_BLIND` (20).

## Docker

```bash
docker compose -f docker/compose.yaml up --build    # http://localhost:8080
python3 scripts/smoke.py http://localhost:8080
```

| Image | Size | Base | Runs as |
|---|---|---|---|
| backend | 7.9 MB | `gcr.io/distroless/static-debian12:nonroot`, static binary (`CGO_ENABLED=0`) | UID 65532 |
| frontend | 60 MB | `nginxinc/nginx-unprivileged:stable-alpine` | UID 101 |

- **Multi-stage builds.** The Go toolchain image is 889 MB, and none of it ships.
- **Layer caching.** Dependency manifests (`go.mod`, `package-lock.json`) are copied before source.
- **The frontend's nginx** serves the React build, falls back to `index.html` for client routes, and
  proxies `/api/` to the `backend` service with buffering off. Its access log goes to the PVC and
  logs `$uri` rather than the full request, because the event-stream URL carries the player's secret token.
- **The backend's `USER` is numeric** (`65532`), because Kubernetes `runAsNonRoot` can't verify a
  user name ([incident 1](troubleshooting/01-non-numeric-user.md)).

## Kubernetes

```bash
minikube start --cpus 4 --memory 6g
minikube addons enable ingress && minikube addons enable metrics-server
docker build -f docker/backend.Dockerfile -t devoker-backend:dev .
docker build -f docker/frontend.Dockerfile -t devoker-frontend:dev .
minikube image load devoker-backend:dev && minikube image load devoker-frontend:dev
kubectl apply -f kubernetes/
kubectl -n ingress-nginx port-forward svc/ingress-nginx-controller 8081:80 &
python3 scripts/smoke.py http://localhost:8081 devoker.local
```

To use a browser, add `127.0.0.1 devoker.local` to `/etc/hosts` and run `minikube tunnel`.

| Object | Devoker's use |
|---|---|
| Namespace | `devoker` |
| Deployment ×2 | backend (1 replica, `Recreate`), frontend (count owned by the HPA) |
| Service ×2 | `backend` (the name nginx proxies to), `frontend` |
| ConfigMap | `STARTING_CHIPS`, `SMALL_BLIND`, `BIG_BLIND`, loaded with `envFrom` |
| Secret | `ghcr` image pull secret (a placeholder in git; the real one is created with kubectl) |
| Ingress | `devoker.local` → frontend, with SSE annotations |
| HPA | frontend 2–5 replicas at 50% CPU |
| PVC | 1 Gi, nginx access log at `/var/log/devoker` |
| Probes | readiness + liveness: backend `/health`, frontend `/` |

Hardening: every container drops all Linux capabilities, disallows privilege escalation and runs as
non-root with a read-only root filesystem. The frontend writes only to `/tmp` (an `emptyDir`) and the log PVC.

The PVC is `ReadWriteOnce`. RWO means one *node*, so all frontend replicas can share it on
single-node minikube. On a multi-node cluster, replicas on other nodes would stay `ContainerCreating`.
A real setup would ship logs to Loki instead.

The real pull secret, when the images come from a private GHCR package:

```bash
kubectl -n devoker create secret docker-registry ghcr --docker-server=ghcr.io \
  --docker-username=OKpoorav --docker-password=<PAT with read:packages> \
  --dry-run=client -o yaml | kubectl apply -f -
```

## Helm

```bash
kubectl delete -f kubernetes/        # if the plain manifests are installed
helm install devoker helm/devoker -n devoker --create-namespace -f helm/values-local.yaml
helm upgrade devoker helm/devoker -n devoker -f helm/values-local.yaml --set game.bigBlind=50
helm history devoker -n devoker
helm rollback devoker <revision> -n devoker
```

- **`values.yaml`** holds the image repositories and tags, resources, autoscaling, game settings,
  the ingress host and the pull secret's *name*.
- **Backend `replicas` is hard-coded to 1** with a comment. A value would invite someone to change it.
- **A `checksum/config` annotation** on the backend pod rolls it when the ConfigMap changes.
  Environment variables are read only at start, so without it a blind change would do nothing.
  Verified: the logs went from `blinds 10/20` to `25/50` on upgrade, and back on rollback.
- **The frontend Deployment omits `replicas` while the HPA is enabled**, so a sync doesn't fight the autoscaler.

## CI/CD

`.github/workflows/ci.yaml`, linted with `actionlint`:

```
           ┌─ test-backend   go vet · go test -race -cover
 push ─────┼─ test-frontend  npm ci · npm run build
           ├─ sast           gosec (fails on HIGH)
           ├─ sca            govulncheck · npm audit (HIGH) · trivy fs (vulns + misconfig)
           └─ secrets        gitleaks over the full history
                                        │ all five must pass
                     build-scan-push   matrix: backend, frontend
                       docker build → trivy image gate → push ghcr.io/okpoorav/devoker-*:<sha> (amd64 + arm64)
                                        │ main only
                     deploy            yq sets both tags in helm/devoker/values.yaml → commit → push
```

- **Images are tagged by commit SHA**, so the running version is traceable and Argo sees a change on every deploy.
- **Images are scanned before they're pushed**, so a vulnerable image never reaches the registry.
- **Images are multi-arch** (amd64 for runners and cloud clusters, arm64 for Apple Silicon minikube).
  The first amd64-only build would have crashed on the ARM minikube with `exec format error`. Go
  cross-compiles natively (`$BUILDPLATFORM`, `TARGETARCH`), and the React build runs once because its output doesn't depend on CPU.
- **Permissions are least-privilege per job.** Only build gets `packages: write`; only deploy gets `contents: write`.
- **The deploy commit doesn't loop.** It includes `[skip ci]`, and the trigger ignores `helm/devoker/values.yaml`.
- **A `concurrency` group** stops two pushes racing to commit tags.
- **The image registry is hard-coded lowercase** (`ghcr.io/okpoorav`). Image names must be lowercase,
  and the GitHub owner is `OKpoorav`.

## DevSecOps

| Scan | Tool | Looks at | Gate |
|---|---|---|---|
| SAST | gosec | our Go code | fails on HIGH |
| SCA | govulncheck | Go standard library and modules, reachable code only | fails on any |
| SCA | npm audit | frontend dependencies | fails on HIGH |
| SCA and misconfig | Trivy fs | lockfiles, Kubernetes manifests, Helm, Dockerfiles, Terraform | fails on HIGH/CRITICAL |
| Secrets | Gitleaks | every commit in history | fails on any |
| Image | Trivy image | OS packages and binaries in each image | fails on fixable HIGH/CRITICAL |

What the gates found while this was built, and what changed:

1. **gosec G404 (HIGH): the deck was shuffled with `math/rand`.** For a card game, a predictable
   shuffle is a real flaw. `Deck.Shuffle` now runs Fisher–Yates over `crypto/rand`, with a test that
   the deck still holds 52 distinct cards and is no longer in factory order.
2. **govulncheck: 8 standard-library CVEs in Go 1.26.1** (net/http, crypto/tls, net/textproto),
   fixed in 1.26.9. The image already built with 1.26.9 (`golang:1.26`). CI now pins the latest
   1.26 patch rather than `go.mod`'s minimum.
3. **Trivy KSV-0014 (HIGH): the frontend root filesystem was writable.** It's now read-only, with `/tmp` as an `emptyDir`.
4. **Trivy AWS-0132 (HIGH): the S3 bucket isn't encrypted with a customer-managed KMS key.**
   Accepted in `security/.trivyignore` with the reason written there. The bucket uses SSE-S3 (AES256),
   and a KMS key costs ~$1/month and can't be deleted for 7+ days, which is wrong for a lab destroyed the same day.

**Proving the gate.** On a branch, `cd frontend && npm i lodash@4.17.20`, then push. `npm audit`
reports lodash HIGH (GHSA-35jh-r3h4-6jhm, command injection) and the pipeline goes red. That was
checked locally (`npm audit` exits 1). Revert afterwards. To prove the secret scan, commit a fake AWS
key on a throwaway branch.

Out-of-band secrets: the GHCR pull secret and Grafana's admin password are created in the cluster,
never committed. The trade-off is that Argo CD doesn't manage them. Sealed Secrets or External Secrets would fix that.

## GitOps

```bash
kubectl create ns argocd
kubectl apply -n argocd --server-side -f https://raw.githubusercontent.com/argoproj/argo-cd/stable/manifests/install.yaml
helm uninstall devoker -n devoker          # Argo owns the release from now on
kubectl apply -f gitops/devoker-app.yaml   # the last manual apply
kubectl -n argocd get secret argocd-initial-admin-secret -o jsonpath='{.data.password}' | base64 -d
kubectl -n argocd port-forward svc/argocd-server 8443:443   # https://localhost:8443, user admin
```

The Application tracks `helm/devoker` on `main` with automated sync, `prune` (delete what git no
longer has), `selfHeal` (undo manual edits) and `CreateNamespace`. CI can't reach a laptop cluster,
so it never deploys directly. It commits the new tag, and Argo pulls it (it polls every ~3 minutes).

- **Roll back** with `git revert` of the deploy commit, not `helm rollback`. Git stays the record of what runs.
- **Drift test:** `kubectl -n devoker scale deploy/backend --replicas=2`. Argo returned it to 1 within
  2 seconds, which is also the fix for [incident 2](troubleshooting/02-split-brain.md).

## Monitoring

```bash
helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
helm install mon prometheus-community/kube-prometheus-stack -n monitoring --create-namespace -f monitoring/values.yaml
kubectl apply -f monitoring/devoker-alerts.yaml
kubectl -n monitoring get secret mon-grafana -o jsonpath='{.data.admin-password}' | base64 -d
kubectl -n monitoring port-forward svc/mon-grafana 3000:80
```

Dashboard: *Kubernetes / Compute Resources / Namespace (Pods)*, namespace `devoker`. Alert rules:

- **DevokerBackendRestarted** (critical): any backend restart, because each one ends every game.
- **DevokerFrontendAtMaxReplicas** (warning): the HPA has been at its ceiling for 10 minutes.

**Load test** ([monitoring/loadtest.md](monitoring/loadtest.md)): fortio inside the cluster, 50
connections for 150 s through the Ingress. It made 2,606,625 requests, all HTTP 200, at 17,375 req/s
and 2.9 ms average. The frontend went from 2 to 5 replicas within 20 s and back to 2 after the load
stopped; the backend stayed at 1.

Logs: `kubectl -n devoker logs deploy/backend -f`, and the nginx access log on the PVC:
`kubectl -n devoker exec deploy/frontend -- tail /var/log/devoker/access.log`.

## Terraform

`terraform/` creates, in `us-east-1` by default:

- **Two ECR repositories** (`devoker-backend`, `devoker-frontend`) from one `for_each` block, with
  scan-on-push, immutable tags and a lifecycle policy keeping the last 10 images.
- **An S3 bucket** with a random suffix, versioning, SSE-S3 encryption and all public access blocked.

```bash
cd terraform
terraform init && terraform fmt -check && terraform validate
terraform plan -out tfplan && terraform apply tfplan
terraform output
terraform destroy
```

Every resource is tagged `Project=devoker, ManagedBy=terraform`. `force_delete` and `force_destroy`
let `destroy` succeed even when the repositories and bucket hold data, which is right for a lab and
wrong for production. Left running, the cost is storage only: ECR at $0.10/GB-month and S3 at
cents. State is local; the bucket is meant for remote state with locking, which a team would need.

## Troubleshooting

Five incidents, each with real output and the same six parts: symptom, investigation, root cause,
fix, verify, prevent. See [troubleshooting/](troubleshooting/README.md).

| # | Symptom | Root cause |
|---|---|---|
| 1 | Backend `CreateContainerConfigError` | `runAsNonRoot` can't verify a named user (happened for real) |
| 2 | "room not found" at random | 2 backend replicas split the in-memory state |
| 3 | Frontend crash-loops, then requests hang | nginx resolves the backend name once, at start |
| 4 | Every API call returns 502 | Bad readiness path, made a full outage by `Recreate` |
| 5 | HPA `<unknown>` | No CPU request on the frontend |

## Screenshots

To capture into `screenshots/`:

- [ ] Two browser windows mid-hand on `devoker.local`
- [ ] `docker images` with both sizes
- [ ] `kubectl get all,ingress,pvc,hpa,cm,secret -n devoker`
- [ ] `helm history devoker -n devoker`
- [ ] Green pipeline; red pipeline from lodash 4.17.20
- [ ] GHCR packages with SHA tags
- [ ] Argo CD app tree, Synced / Healthy
- [ ] Grafana CPU during the load test; `kubectl get hpa -w` scaling 2 → 5 → 2
- [ ] Prometheus *Alerts* page showing the two Devoker rules
- [ ] `terraform plan`, `apply`, `output`, `destroy`

## Lessons learned

1. **The app's state model decides the deployment.** In-memory state forced 1 replica, `Recreate`,
   and moving the HPA to the frontend. The next step is Redis for room state, which would allow
   rolling updates and backend scaling.
2. **Security scans find real problems.** The first gosec run found a predictable card shuffle;
   govulncheck found an outdated Go toolchain; Trivy found a writable filesystem.
3. **Accepting a finding is fine if it's written down.** `.trivyignore` says why, and when to revisit.
4. **Docker passing isn't Kubernetes passing.** The `runAsNonRoot` failure only shows up in a pod.
5. **Manual changes fight automation.** A `kubectl scale` took field ownership and blocked a Helm 4
   upgrade. GitOps with `selfHeal` removes that whole class of problem.
6. **nginx resolves upstreams once.** A recreated Service leaves old pods pointing at a dead IP.

## API

| Method | Path | Body | Returns |
|---|---|---|---|
| GET | `/health` | | `ok` |
| POST | `/api/rooms` | `{"name"}` | `{"room","token"}` |
| POST | `/api/rooms/{code}/join` | `{"name"}` | `{"room","token"}` |
| POST | `/api/rooms/{code}/start` | `{"token"}` | 204 |
| POST | `/api/rooms/{code}/action` | `{"token","action":"fold\|check\|call\|raise","amount"}` | 204 |
| POST | `/api/rooms/{code}/rebuy` | `{"token"}` | 204 |
| POST | `/api/rooms/{code}/requests/{id}` | `{"token","approve"}` | 204 (host only) |
| GET | `/api/rooms/{code}/events?token=` | | SSE stream of the table as JSON |

The token is a secret per player. Only the owner's stream includes their hole cards. Whoever creates
the table is the host. Joining sends the host a request, and the player's stream reports `pending`
until the host approves (`seated`) or declines (`rejected`). A busted player can buy back in between
hands; the host's own rebuy is immediate, everyone else's needs approval. There is no turn timer, so
a player who closes their tab mid-hand stalls the table until they return.
