output "ecr_repository_urls" {
  description = "Push targets, e.g. docker push <url>:<tag>"
  value       = { for k, r in aws_ecr_repository.app : k => r.repository_url }
}

output "artifacts_bucket" {
  value = aws_s3_bucket.artifacts.bucket
}
