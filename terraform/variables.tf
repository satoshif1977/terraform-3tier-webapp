# =============================================================
# AWS 3層 Web アーキテクチャ - 入力変数定義
# =============================================================

variable "aws_region" {
  description = "使用する AWS リージョン"
  type        = string
  default     = "ap-northeast-1"
}

variable "project" {
  description = "プロジェクト名（リソース命名に使用）"
  type        = string
  default     = "webapp"
}

variable "environment" {
  description = "環境名（dev / stg / prod）"
  type        = string
  default     = "dev"
}

# ── ネットワーク ──────────────────────────────────────────────

variable "vpc_cidr" {
  description = "VPC の CIDR ブロック"
  type        = string
  default     = "10.0.0.0/16"
}

variable "availability_zones" {
  description = "使用するアベイラビリティゾーン（2 つ以上でマルチ AZ 構成）"
  type        = list(string)
  default     = ["ap-northeast-1a", "ap-northeast-1c"]
}

variable "public_subnet_cidrs" {
  description = "パブリックサブネット CIDR リスト（ALB を配置）"
  type        = list(string)
  default     = ["10.0.1.0/24", "10.0.2.0/24"]
}

variable "private_subnet_cidrs" {
  description = "プライベートサブネット CIDR リスト（EC2・RDS を配置）"
  type        = list(string)
  default     = ["10.0.11.0/24", "10.0.12.0/24"]
}

# ── EC2 ──────────────────────────────────────────────────────

variable "instance_type" {
  description = "EC2 インスタンスタイプ"
  type        = string
  default     = "t3.micro"
}

variable "key_name" {
  description = "EC2 キーペア名（SSH 接続用。空文字の場合は SSH 接続不可）"
  type        = string
  default     = ""
}

# ── RDS ──────────────────────────────────────────────────────

variable "db_name" {
  description = "データベース名"
  type        = string
  default     = "appdb"
}

variable "db_username" {
  description = "データベース管理者ユーザー名"
  type        = string
  default     = "admin"
}

variable "db_password" {
  description = "データベース管理者パスワード（terraform.tfvars に設定してください）"
  type        = string
  sensitive   = true
}
