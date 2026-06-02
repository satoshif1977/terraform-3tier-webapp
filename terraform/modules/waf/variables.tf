variable "project" {
  description = "プロジェクト名（リソース命名に使用）"
  type        = string
}

variable "environment" {
  description = "環境名（dev / stg / prod）"
  type        = string
}

variable "alb_arn" {
  description = "WAF を関連付ける ALB の ARN"
  type        = string
}
