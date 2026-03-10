variable "project" {
  description = "プロジェクト名"
  type        = string
}

variable "environment" {
  description = "環境名"
  type        = string
}

variable "vpc_id" {
  description = "ALB を配置する VPC の ID"
  type        = string
}

variable "public_subnet_ids" {
  description = "ALB を配置するパブリックサブネット ID リスト（複数 AZ）"
  type        = list(string)
}
