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
