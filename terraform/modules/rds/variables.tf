variable "project" {
  description = "プロジェクト名"
  type        = string
}

variable "environment" {
  description = "環境名"
  type        = string
}

variable "vpc_id" {
  description = "RDS を配置する VPC の ID"
  type        = string
}

variable "private_subnet_ids" {
  description = "RDS を配置するプライベートサブネット ID リスト（Multi-AZ のため 2 つ必要）"
  type        = list(string)
}

variable "ec2_sg_id" {
  description = "EC2 セキュリティグループ ID（MySQL アクセスを許可するソース）"
  type        = string
}

variable "instance_class" {
  description = "RDS インスタンスクラス"
  type        = string
  default     = "db.t3.micro"
}

variable "db_name" {
  description = "データベース名"
  type        = string
  default     = "appdb"
}

variable "db_username" {
  description = "データベース管理者ユーザー名"
  type        = string
}

variable "db_password" {
  description = "データベース管理者パスワード"
  type        = string
  sensitive   = true
}
