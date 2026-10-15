# 1. Backend stuck in CreateContainerConfigError

**Symptom.** The first `kubectl apply -f kubernetes/`: frontend came up, backend never did.

```
NAME                        READY   STATUS                       RESTARTS   AGE
backend-7f9b866bcc-7jwsg    0/1     CreateContainerConfigError   0          2m10s
frontend-5d88dbcf7d-2zt7j   1/1     Running                      0          2m10s
```

**Investigation.** `kubectl -n devoker describe pod -l app=backend`, Events section:

```
Warning  Failed  12s (x11 over 2m10s)  kubelet  Error: container has runAsNonRoot and image has
non-numeric user (nonroot), cannot verify user is non-root
```

**Root cause.** The Dockerfile said `USER nonroot:nonroot`. The pod sets `runAsNonRoot: true`, and the
kubelet checks that by UID. A name can't be checked without reading `/etc/passwd` inside the image,
so the kubelet refuses to start the container.

**Fix.** `docker/backend.Dockerfile` now says `USER 65532:65532` (the same user distroless calls
`nonroot`). Rebuilt, reloaded into minikube, deleted the pod.

**Verify.** `deployment "backend" successfully rolled out`; `scripts/smoke.py` played a hand.

**Prevent.** Always use a numeric `USER` in images that run under `runAsNonRoot`. Docker alone
never shows this error, so only a Kubernetes run catches it: the cluster deploy is the test.
