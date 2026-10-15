# Troubleshooting log

Every incident here happened on the minikube cluster and the output is copied from the terminal.
Incident 1 was not planted: it happened the first time the manifests were applied. Incidents 2 to 5
were planted one at a time, then fixed by redeploying the chart.

| # | Symptom | Root cause |
|---|---|---|
| [1](01-non-numeric-user.md) | Backend stuck in `CreateContainerConfigError` | `runAsNonRoot` can't verify a user given by name |
| [2](02-split-brain.md) | Players randomly get "room not found" | Backend scaled to 2 replicas; game state is per-process |
| [3](03-upstream-not-found.md) | New frontend pods crash-loop, then requests hang | nginx resolves `backend` once, at startup |
| [4](04-readiness-outage.md) | Every API call returns 502 | Readiness probe path typo plus `Recreate` strategy |
| [5](05-hpa-unknown.md) | HPA shows `cpu: <unknown>/50%` | Frontend container has no CPU request |

Toolkit used every time, in this order: `kubectl get pods` → `describe` → `logs` (`--previous`)
→ `get events --sort-by=.lastTimestamp` → `get endpointslices` → curl from inside a pod.
