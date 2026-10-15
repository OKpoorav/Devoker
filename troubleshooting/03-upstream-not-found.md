# 3. New frontend pods crash-loop, then requests hang

**Planted.** `kubectl -n devoker delete svc backend`, then `kubectl -n devoker rollout restart deploy/frontend`
(the same failure as a typo in the `proxy_pass` host in `docker/nginx.conf`).

**Symptom.** The site still loaded (`site: 200`), but the new frontend pods never became Ready:

```
frontend-69949bffb9-442cb   1/1     Running            0             13m
frontend-fd56879b-4w24s     0/1     CrashLoopBackOff   3 (6s ago)    39s
frontend-fd56879b-5q2sf     0/1     Error              3 (27s ago)   40s
```

**Investigation.** `kubectl -n devoker logs <new pod> --previous`:

```
nginx: [emerg] host not found in upstream "backend" in /etc/nginx/conf.d/default.conf:16
```

**Root cause.** nginx resolves `proxy_pass` hostnames once, when it starts. With no `backend`
Service there is no DNS name, which is a fatal config error, so the container exits and restarts forever.
The rolling update protected players: old pods kept serving because new ones never went Ready.

**Fix.** Redeploy the chart, which recreates the Service.

**A second symptom after the fix.** The first API call after recovery timed out. Testing each pod
showed all the new pods worked. The timeout came from the old pods, which had resolved the *deleted*
Service's ClusterIP at startup and were sending requests to an address that no longer existed until
they were replaced. A recreated Service gets a new ClusterIP (`10.108.0.229`).

**Verify.** All 5 frontend pods answered `POST /api/rooms` from inside; the smoke test passed 3 out of 3.

**Prevent.** A readiness probe on `/` doesn't touch `/api`, so a broken backend path stays "Ready".
Options: probe through the proxy (for example, `/api/health` mapped to the backend), or use
`resolver kube-dns...` with a variable in `proxy_pass` so nginx re-resolves.
