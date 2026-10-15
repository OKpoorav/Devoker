output "ecr_repository_urls" {
  description = "Push targets, e.g. docker push <url>:<tag>"
  value       = { for k, r in aws_ecr_repository.app : k => r.repository_url }
}

output "artifacts_bucket" {
  value = aws_s3_bucket.artifacts.bucket
}

output "cluster_name" {
  value = aws_eks_cluster.main.name
}

output "kubeconfig_command" {
  description = "Run this after apply, then `kubectl get nodes`."
  value       = "aws eks update-kubeconfig --region ${var.aws_region} --name ${aws_eks_cluster.main.name}"
}

output "vpc_id" {
  value = aws_vpc.main.id
}

output "public_subnet_ids" {
  value = aws_subnet.public[*].id
}
