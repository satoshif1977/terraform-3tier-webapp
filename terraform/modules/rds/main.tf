# =============================================================
# RDS モジュール - MySQL Multi-AZ データベース
# 2 つの AZ にまたがるマネージド DB（自動フェイルオーバー付き）
# =============================================================

# RDS セキュリティグループ: EC2 からの MySQL 接続のみ許可
resource "aws_security_group" "rds" {
  name        = "${var.project}-${var.environment}-rds-sg"
  description = "RDS MySQL へのアクセスを EC2 からのみ許可"
  vpc_id      = var.vpc_id

  # MySQL ポート: EC2 セキュリティグループからのみ許可
  ingress {
    description     = "MySQL from EC2"
    from_port       = 3306
    to_port         = 3306
    protocol        = "tcp"
    security_groups = [var.ec2_sg_id]
  }

  # アウトバウンド: 不要だが明示的に制限（RDS は送信側になる必要なし）
  egress {
    description = "Allow all outbound"
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }

  tags = {
    Name = "${var.project}-${var.environment}-rds-sg"
  }
}

# DB サブネットグループ: RDS を配置するプライベートサブネットの集合
# Multi-AZ のために最低 2 つの異なる AZ のサブネットが必要
resource "aws_db_subnet_group" "this" {
  name        = "${var.project}-${var.environment}-db-subnet-group"
  description = "RDS 用プライベートサブネットグループ（マルチ AZ）"
  subnet_ids  = var.private_subnet_ids

  tags = {
    Name = "${var.project}-${var.environment}-db-subnet-group"
  }
}

# RDS パラメータグループ: MySQL の設定をカスタマイズするグループ
resource "aws_db_parameter_group" "this" {
  family      = "mysql8.0"
  name        = "${var.project}-${var.environment}-mysql-params"
  description = "MySQL 8.0 カスタムパラメータグループ"

  # 文字コードを UTF-8 に統一（日本語データを正しく扱うために必要）
  parameter {
    name  = "character_set_server"
    value = "utf8mb4"
  }

  parameter {
    name  = "character_set_client"
    value = "utf8mb4"
  }

  tags = {
    Name = "${var.project}-${var.environment}-mysql-params"
  }
}

# RDS インスタンス本体
resource "aws_db_instance" "this" {
  # 基本設定
  identifier     = "${var.project}-${var.environment}-mysql"
  engine         = "mysql"
  engine_version = "8.0"
  instance_class = var.instance_class

  # ストレージ設定
  allocated_storage     = 20      # 初期ストレージ 20GB
  max_allocated_storage = 100     # オートスケーリング上限 100GB
  storage_type          = "gp3"
  storage_encrypted     = true    # 保存データの暗号化（セキュリティ要件）

  # 認証情報
  db_name  = var.db_name
  username = var.db_username
  password = var.db_password

  # ネットワーク設定
  db_subnet_group_name   = aws_db_subnet_group.this.name
  vpc_security_group_ids = [aws_security_group.rds.id]
  publicly_accessible    = false  # プライベートサブネット内のみ（外部公開しない）

  # Multi-AZ 設定: スタンバイインスタンスを別 AZ に自動作成
  # プライマリに障害が発生すると自動でスタンバイに切り替わる（フェイルオーバー）
  multi_az = true

  # バックアップ設定
  backup_retention_period = 7           # 7 日間の自動バックアップを保持
  backup_window           = "03:00-04:00" # バックアップ実行時間帯（UTC）
  maintenance_window      = "Mon:04:00-Mon:05:00"

  # パラメータグループ
  parameter_group_name = aws_db_parameter_group.this.name

  # 削除設定（検証環境用）
  deletion_protection   = false   # 本番では true 推奨
  skip_final_snapshot   = true    # 削除時の最終スナップショットをスキップ（検証用）
  # 本番では以下を設定:
  # skip_final_snapshot       = false
  # final_snapshot_identifier = "${var.project}-${var.environment}-final-snapshot"

  tags = {
    Name = "${var.project}-${var.environment}-mysql"
  }
}
