# Cloud registry for the two images: the AWS alternative to GHCR, with scan-on-push.
resource "aws_ecr_repository" "app" {
  for_each             = toset(["backend", "frontend"])
  name                 = "${var.project}-${each.key}"
  image_tag_mutability = "IMMUTABLE" # a SHA tag always means the same image
  force_delete         = true        # lets `terraform destroy` remove repos that still hold images (fine for a lab, not prod)

  image_scanning_configuration {
    scan_on_push = true
  }
}

resource "aws_ecr_lifecycle_policy" "app" {
  for_each   = aws_ecr_repository.app
  repository = each.value.name
  policy = jsonencode({
    rules = [{
      rulePriority = 1
      description  = "Keep the last ${var.images_to_keep} images"
      selection    = { tagStatus = "any", countType = "imageCountMoreThan", countNumber = var.images_to_keep }
      action       = { type = "expire" }
    }]
  })
}

# Bucket for remote Terraform state / build artifacts. Bucket names are global, hence the suffix.
resource "random_id" "suffix" {
  byte_length = 4
}

resource "aws_s3_bucket" "artifacts" {
  bucket        = "${var.project}-artifacts-${random_id.suffix.hex}"
  force_destroy = true
}

resource "aws_s3_bucket_versioning" "artifacts" {
  bucket = aws_s3_bucket.artifacts.id
  versioning_configuration {
    status = "Enabled"
  }
}

resource "aws_s3_bucket_server_side_encryption_configuration" "artifacts" {
  bucket = aws_s3_bucket.artifacts.id
  rule {
    apply_server_side_encryption_by_default {
      sse_algorithm = "AES256"
    }
  }
}

resource "aws_s3_bucket_public_access_block" "artifacts" {
  bucket                  = aws_s3_bucket.artifacts.id
  block_public_acls       = true
  block_public_policy     = true
  ignore_public_acls      = true
  restrict_public_buckets = true
}
