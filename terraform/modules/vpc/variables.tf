# =============================================================
# VPC モジュール - 入力変数定義
# =============================================================

variable "project" {
  description = "プロジェクト名（リソースの命名に使用）"
  type        = string
}

variable "environment" {
  description = "環境名（dev / stg / prod）"
  type        = string
}

variable "vpc_cidr" {
  description = "VPC の CIDR ブロック（例: 10.0.0.0/16）"
  type        = string
  default     = "10.0.0.0/16"
}

variable "availability_zones" {
  description = "使用するアベイラビリティゾーンのリスト（マルチ AZ のため 2 つ以上推奨）"
  type        = list(string)
  default     = ["ap-northeast-1a", "ap-northeast-1c"]
}

variable "public_subnet_cidrs" {
  description = "パブリックサブネットの CIDR ブロックリスト"
  type        = list(string)
  default     = ["10.0.1.0/24", "10.0.2.0/24"]
}

variable "private_subnet_cidrs" {
  description = "プライベートサブネットの CIDR ブロックリスト"
  type        = list(string)
  default     = ["10.0.11.0/24", "10.0.12.0/24"]
}

variable "enable_nat_gateway" {
  description = "NAT Gateway を作成するか"
  type        = bool
  default     = true
}

variable "enable_flow_logs" {
  description = "VPC Flow Logs を有効化するか"
  type        = bool
  default     = true
}

variable "flow_log_retention_days" {
  description = "VPC Flow Logs の CloudWatch Logs 保持期間（日数）"
  type        = number
  default     = 30
}

variable "tags" {
  description = "全リソースに付与する共通タグ"
  type        = map(string)
  default     = {}
}
