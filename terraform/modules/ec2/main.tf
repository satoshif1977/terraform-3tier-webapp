# =============================================================
# EC2 モジュール - Web サーバー × 2 台
# 異なる AZ に配置してマルチ AZ 可用性を確保
# =============================================================

# EC2 セキュリティグループ: ALB からの HTTP のみ許可（直接アクセス禁止）
resource "aws_security_group" "ec2" {
  name        = "${var.project}-${var.environment}-ec2-sg"
  description = "EC2 Web サーバーのセキュリティグループ"
  vpc_id      = var.vpc_id

  # HTTP: ALB セキュリティグループからのみ許可（直接アクセスを防ぐ）
  ingress {
    description     = "HTTP from ALB only"
    from_port       = 80
    to_port         = 80
    protocol        = "tcp"
    security_groups = [var.alb_sg_id]
  }

  # SSH: VPC 内部からのみ許可（踏み台経由アクセス用）
  ingress {
    description = "SSH from VPC"
    from_port   = 22
    to_port     = 22
    protocol    = "tcp"
    cidr_blocks = [var.vpc_cidr]
  }

  # アウトバウンド: すべて許可（yum・SNS API・SSM 通信に必要）
  egress {
    description = "Allow all outbound"
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }

  tags = {
    Name = "${var.project}-${var.environment}-ec2-sg"
  }
}

# =============================================================
# IAM ロール（課題2: PowerUser 権限で SNS/SQS 操作を許可）
# EC2 にアタッチすることで、アクセスキー不要で AWS API を呼び出せる
# =============================================================

resource "aws_iam_role" "ec2" {
  name        = "${var.project}-${var.environment}-ec2-role"
  description = "EC2 用 IAM ロール（PowerUser 権限）"

  # EC2 サービスがこのロールを引き受けることを許可
  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect    = "Allow"
      Principal = { Service = "ec2.amazonaws.com" }
      Action    = "sts:AssumeRole"
    }]
  })

  tags = {
    Name = "${var.project}-${var.environment}-ec2-role"
  }
}

# PowerUserAccess: IAM 管理以外のすべての AWS サービスを操作できる権限
# SNS publish・SQS receive・CloudWatch など課題2 の要件を満たす
resource "aws_iam_role_policy_attachment" "power_user" {
  role       = aws_iam_role.ec2.name
  policy_arn = "arn:aws:iam::aws:policy/PowerUserAccess"
}

# SSM Session Manager: SSH キーなしでコンソールから安全にアクセスするための権限
resource "aws_iam_role_policy_attachment" "ssm" {
  role       = aws_iam_role.ec2.name
  policy_arn = "arn:aws:iam::aws:policy/AmazonSSMManagedInstanceCore"
}

# インスタンスプロファイル: IAM ロールを EC2 にアタッチするための媒介オブジェクト
resource "aws_iam_instance_profile" "ec2" {
  name = "${var.project}-${var.environment}-ec2-profile"
  role = aws_iam_role.ec2.name
}

# =============================================================
# 最新の Amazon Linux 2023 AMI を自動取得
# ハードコードを避けることでリージョン・時期によらず最新版を使用できる
# =============================================================

data "aws_ami" "amazon_linux_2023" {
  most_recent = true
  owners      = ["amazon"]

  filter {
    name   = "name"
    values = ["al2023-ami-*-x86_64"]
  }

  filter {
    name   = "state"
    values = ["available"]
  }
}

# =============================================================
# EC2 インスタンス × 2 台（マルチ AZ 配置）
# =============================================================

resource "aws_instance" "web" {
  count = 2 # 2 台 = 2 AZ に 1 台ずつ

  ami                    = data.aws_ami.amazon_linux_2023.id
  instance_type          = var.instance_type
  subnet_id              = var.private_subnet_ids[count.index] # 各 AZ のプライベートサブネット
  vpc_security_group_ids = [aws_security_group.ec2.id]
  iam_instance_profile   = aws_iam_instance_profile.ec2.name
  key_name               = var.key_name != "" ? var.key_name : null

  # User Data: インスタンス起動時に自動実行されるスクリプト
  user_data = templatefile("${path.module}/userdata.sh", {
    sns_topic_arn = var.sns_topic_arn
    aws_region    = var.aws_region
  })

  # ルートボリューム設定
  root_block_device {
    volume_type           = "gp3"
    volume_size           = 20
    encrypted             = true # ストレージ暗号化（セキュリティ要件）
    delete_on_termination = true
  }

  tags = {
    Name = "${var.project}-${var.environment}-web-${count.index + 1}"
    Role = count.index == 0 ? "web-primary" : "web-secondary"
  }
}

# ターゲットグループへの EC2 登録: ALB がこの 2 台にトラフィックを振り分ける
resource "aws_lb_target_group_attachment" "web" {
  count            = 2
  target_group_arn = var.target_group_arn
  target_id        = aws_instance.web[count.index].id
  port             = 80
}
