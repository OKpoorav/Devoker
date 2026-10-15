# Load test: frontend HPA

Run on minikube (4 CPU, 6 GB), 2026-10-16, through the Ingress so the path matches real players.

```bash
kubectl -n devoker run load --rm -i --restart=Never --image=fortio/fortio -- \
  load -quiet -qps 0 -c 50 -t 150s -H "Host: devoker.local" http://ingress-nginx-controller.ingress-nginx.svc/
kubectl -n devoker get hpa frontend -w   # in a second terminal
```

| | Value |
|---|---|
| Requests | 2,606,625, all HTTP 200 |
| Throughput | 17,375 req/s, 2.9 ms average |
| Frontend replicas | 2 before; 5 (the max) within 20 s of the load starting; back to 2 afterwards |
| Backend replicas | 1 throughout, by design |

The target is `/` (static files), not `/api/.../events`: each event stream stays open for the whole
game, so loading it measures open connections rather than CPU.

Scale-down back to 2 takes about 5 minutes after the load stops: that is the HPA's default
`scaleDown.stabilizationWindowSeconds: 300`, which avoids flapping when load is bursty.
