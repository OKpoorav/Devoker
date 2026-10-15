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

variable "kubernetes_version" {
  description = "EKS Kubernetes version. null lets AWS pick its current default, so the lab never pins a version that drifts into paid extended support."
  type        = string
  default     = null
}

variable "node_instance_type" {
  description = "EC2 instance type for the managed worker nodes."
  type        = string
  default     = "t3.small"
}

variable "node_count" {
  description = "Desired and maximum number of worker nodes."
  type        = number
  default     = 2
}

variable "cluster_public_access_cidrs" {
  description = "CIDRs allowed to reach the public EKS API endpoint. Narrow to your laptop's IP (x.x.x.x/32) when you can."
  type        = list(string)
  default     = ["0.0.0.0/0"]
}
