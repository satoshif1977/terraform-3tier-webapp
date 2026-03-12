variable "project" {
  description = "プロジェクト名"
  type        = string
}

variable "environment" {
  description = "環境名"
  type        = string
}

variable "vpc_id" {
  description = "EC2 を配置する VPC の ID"
  type        = string
}

variable "vpc_cidr" {
  description = "VPC の CIDR（SSH 許可ルールに使用）"
  type        = string
}

variable "private_subnet_ids" {
  description = "EC2 を配置するプライベートサブネット ID リスト（2 つ以上必要）"
  type        = list(string)
}

variable "alb_sg_id" {
  description = "ALB セキュリティグループ ID（EC2 へのアクセスを ALB からのみに制限）"
  type        = string
}

variable "target_group_arn" {
  description = "ALB ターゲットグループ ARN（EC2 を登録する先）"
  type        = string
}

variable "instance_type" {
  description = "EC2 インスタンスタイプ"
  type        = string
  default     = "t3.micro"
}

variable "key_name" {
  description = "SSH キーペア名（空文字の場合は SSH 無効）"
  type        = string
  default     = ""
}

variable "sns_topic_arn" {
  description = "SNS トピック ARN（Part 2: CPU 監視メッセージの送信先）"
  type        = string
}

variable "aws_region" {
  description = "AWS リージョン（SNS 送信に使用）"
  type        = string
  default     = "ap-northeast-1"
}
