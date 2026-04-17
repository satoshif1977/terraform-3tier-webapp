# =============================================================
# AWS 3層 Web アーキテクチャ - Terraform メイン設定
# Part 1: VPC + ALB + EC2 × 2 + RDS Multi-AZ
# Part 2: SNS + SQS + EC2 監視基盤
# =============================================================

terraform {
  required_version = ">= 1.5"

  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
  }
}

provider "aws" {
  region = var.aws_region

  # すべてのリソースに共通タグを自動付与
  default_tags {
    tags = {
      Project     = var.project
      Environment = var.environment
      ManagedBy   = "Terraform"
    }
  }
}

locals {
  project     = var.project
  environment = var.environment
}

# ── VPC ──────────────────────────────────────────────────────
# 仮想プライベートネットワーク: すべてのリソースを格納する論理的なネットワーク空間
module "vpc" {
  source = "./modules/vpc"

  project     = local.project
  environment = local.environment

  vpc_cidr                = var.vpc_cidr
  availability_zones      = var.availability_zones
  public_subnet_cidrs     = var.public_subnet_cidrs
  private_subnet_cidrs    = var.private_subnet_cidrs
  enable_nat_gateway      = true # Private Subnet からのインターネットアクセスを許可（SSM・yum等に必要）
  enable_flow_logs        = true # セキュリティ監査用にネットワークログを記録
  flow_log_retention_days = 30

  tags = {
    Project     = local.project
    Environment = local.environment
    ManagedBy   = "Terraform"
  }
}

# ── ALB ──────────────────────────────────────────────────────
# Application Load Balancer: インターネットからのリクエストを EC2 に分散する入口
module "alb" {
  source = "./modules/alb"

  project     = local.project
  environment = local.environment

  vpc_id            = module.vpc.vpc_id
  public_subnet_ids = module.vpc.public_subnet_ids
}

# ── EC2 (Web サーバー × 2) ───────────────────────────────────
# 2 台の Web サーバー: 異なる AZ に配置して可用性を確保
module "ec2" {
  source = "./modules/ec2"

  project     = local.project
  environment = local.environment

  vpc_id             = module.vpc.vpc_id
  vpc_cidr           = module.vpc.vpc_cidr
  private_subnet_ids = module.vpc.private_subnet_ids
  alb_sg_id          = module.alb.alb_sg_id
  target_group_arn   = module.alb.target_group_arn
  instance_type      = var.instance_type
  key_name           = var.key_name
  sns_topic_arn      = module.monitoring.sns_topic_arn
  aws_region         = var.aws_region
}

# ── RDS (MySQL Multi-AZ) ─────────────────────────────────────
# Multi-AZ RDS: 2 つの AZ にまたがるマネージド MySQL データベース
module "rds" {
  source = "./modules/rds"

  project     = local.project
  environment = local.environment

  vpc_id             = module.vpc.vpc_id
  private_subnet_ids = module.vpc.private_subnet_ids
  ec2_sg_id          = module.ec2.ec2_sg_id
  db_name            = var.db_name
  db_username        = var.db_username
  db_password        = var.db_password
}

# ── 監視基盤（Part 2: SNS + SQS） ──────────────────────────────
# SNS → SQS 連携: CPU 使用率のメッセージングパイプライン
module "monitoring" {
  source = "./modules/monitoring"

  project     = local.project
  environment = local.environment
}
