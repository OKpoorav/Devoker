# 5. HPA shows cpu: <unknown>/50%

**Planted.** Removed `resources` from the frontend container with `kubectl patch`.

**Symptom.**

```
NAME       REFERENCE             TARGETS              MINPODS   MAXPODS   REPLICAS
frontend   Deployment/frontend   cpu: <unknown>/50%   2         5         5
```

The HPA can't scale in either direction; it stays at whatever count it had.

**Investigation.** `kubectl -n devoker describe hpa frontend`:

```
ScalingActive   False   FailedGetResourceMetric  the HPA was unable to compute the replica count:
failed to get cpu utilization: missing request for cpu in container frontend of Pod frontend-776fb6f48b-ptvpv
```

**Root cause.** CPU utilization is usage ÷ request. With no request there is nothing to divide by.

**Fix.** Redeploy the chart (requests come from `values.yaml`).

**Verify.** `cpu: 5%/50%` within a minute, and the HPA resumed scaling down to 2.

**Prevent.** Keep requests in the chart's values, and let Argo CD's `selfHeal` undo manual edits
like this one. Trivy's misconfiguration scan does report missing requests, but only at LOW (KSV-0015),
which is below the CI gate. Raising the gate for that one check would catch it before merge.
