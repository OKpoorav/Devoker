# 4. Every API call returns 502

**Planted.** The backend readiness probe path changed from `/health` to `/healthz`:
`kubectl -n devoker patch deploy backend --type=json -p='[{"op":"replace","path":"/spec/template/spec/containers/0/readinessProbe/httpGet/path","value":"/healthz"}]'`

**Symptom.** Creating a table fails:

```
$ curl -XPOST -H 'Host: devoker.local' localhost:8081/api/rooms -d '{"name":"x"}'
<html><head><title>502 Bad Gateway</title></head> ... nginx/1.30.6 -> 502
```

and the access log on the PVC shows `"POST /api/rooms" 502 157`.

**Investigation.**

```
$ kubectl -n devoker get pods -l app=backend
backend-55fddc797b-d55wz   0/1     Running   0          30s
$ kubectl -n devoker describe pod -l app=backend
Warning  Unhealthy  4s (x6 over 29s)  kubelet  Readiness probe failed: HTTP probe failed with statuscode: 404
$ kubectl -n devoker get endpointslices -l kubernetes.io/service-name=backend ...
10.244.0.42 ready=false
```

The process is running fine; Kubernetes just won't send it traffic.

**Root cause.** `/healthz` doesn't exist (the app serves `/health`), so the pod is never Ready, and a
Service only routes to Ready pods. Because the backend uses `strategy: Recreate`, the old healthy pod
was stopped *before* the new one started. With a rolling update this would have been a stuck rollout
with the old pod still serving. Here it is a full outage.

**Fix.** Redeploy the chart (probe back to `/health`).

**Verify.** The smoke test passed.

**Prevent.** This is the cost of `Recreate`, which the in-memory state forces. Mitigations: a CI
step that renders the chart and checks probe paths against the app's routes, or a post-deploy smoke
test (`scripts/smoke.py`) that fails the pipeline.
