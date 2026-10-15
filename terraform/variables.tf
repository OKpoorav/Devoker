variable "aws_region" {
  description = "AWS region to deploy into."
  type        = string
  default     = "us-east-1"
}

variable "project" {
  description = "Prefix for every resource name."
  type        = string
  default     = "devoker"
}

variable "images_to_keep" {
  description = "ECR keeps this many most recent images per repository and expires older ones."
  type        = number
  default     = 10
}
