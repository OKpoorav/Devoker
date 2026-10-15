# 2. Players randomly get "room not found"

**Planted.** `kubectl -n devoker scale deploy/backend --replicas=2`

**Symptom.** A table is created, then a friend tries to join the same code several times:

```
created FACWP
join attempt 1: 200 {"room":"FACWP","token":"..."}
join attempt 2: 200 {"room":"FACWP","token":"..."}
join attempt 3: 404 {"error":"room not found"}
join attempt 4: 404 {"error":"room not found"}
join attempt 5: 200 {"room":"FACWP","token":"..."}
join attempt 6: 404 {"error":"room not found"}
```

The smoke test, which needs about 10 requests in a row to reach the same table, failed 10 out of 10.

**Investigation.** Pods were all Running and Ready, so nothing looked broken. The Service had two endpoints:

```
$ kubectl -n devoker get endpointslices -l kubernetes.io/service-name=backend ...
10.244.0.17 backend-75788fcddd-sddx9
10.244.0.36 backend-75788fcddd-dl7s8
```

"Works sometimes" with healthy pods means state split across replicas.

**Root cause.** Rooms live in the memory of one Go process. The Service load-balances every request,
so the room exists on one pod and is "not found" on the other.

**Fix.** Redeploy the chart, which hard-codes `replicas: 1`. Helm 4 refused at first:

```
Error: UPGRADE FAILED: conflict occurred while applying object devoker/backend apps/v1, Kind=Deployment:
Apply failed with 1 conflict: conflict with "kubectl" with subresource "scale" using apps/v1: .spec.replicas
```

Helm 4 uses server-side apply, and the manual `kubectl scale` had taken ownership of the field.
`helm upgrade ... --force-conflicts` took it back. Under Argo CD with `selfHeal`, the manual scale
is reverted automatically within minutes.

**Verify.** `backend 1/1`, then the smoke test passed 3 out of 3.

**Prevent.** `replicas` is deliberately not a Helm value, with a comment explaining why. The real
fix, if the backend must scale, is moving room state to Redis or a database.

**Update: fixed at the root.** The backend was rewritten in Python with every room stored in
PostgreSQL and changed under a row lock (`SELECT ... FOR UPDATE`). The chart now runs 2 backend
replicas on purpose. The same smoke test through the Ingress passed 10 out of 10 with both replicas serving.
