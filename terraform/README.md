# Terraform: AWS lab stack

Creates, in `us-east-1` by default:

- Two ECR repositories (`devoker-backend`, `devoker-frontend`) with scan-on-push and a lifecycle policy.
- A versioned, encrypted, private S3 bucket for artifacts.
- A VPC with two public subnets in two AZs, an internet gateway and a public route table. There is no NAT gateway.
- An EKS cluster (API endpoint public, standard support only) with one managed node group of 2 x `t3.small` on-demand nodes in the public subnets.

## Cost

The cluster is meant to live about an hour for screenshots, then be destroyed.

| Item | $/hour |
| --- | --- |
| EKS control plane | 0.10 |
| 2 x t3.small | ~0.042 |
| Public IPv4 on the 2 nodes, 0.005 each | 0.01 |
| NAT gateway | 0 (none) |
| **Total** | **~0.15** |

ECR and S3 cost cents per month at this size. `upgrade_policy` is `STANDARD`, so the cluster can never bill extended-support rates ($0.60/h).

**Leaving the cluster up costs about $110-150/month. Destroy it when you are done.**

## Run it

```sh
cp terraform.tfvars.example terraform.tfvars   # optional; edit region, node type, your IP
terraform init
terraform plan
terraform apply                                # ~15 minutes, mostly EKS
$(terraform output -raw kubeconfig_command)    # aws eks update-kubeconfig --region ... --name ...
kubectl get nodes                              # expect 2 nodes in Ready
# take the screenshots
terraform destroy                              # ~10 minutes
```

Credentials come from your AWS CLI profile or environment, never from files in this directory. `terraform.tfvars` is gitignored.
