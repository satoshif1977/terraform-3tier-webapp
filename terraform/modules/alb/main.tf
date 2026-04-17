# =============================================================
# ALB モジュール - Application Load Balancer
# インターネットからのリクエストを EC2 に振り分ける負荷分散装置
# =============================================================

# ALB セキュリティグループ: インターネットからの HTTP/HTTPS を許可
resource "aws_security_group" "alb" {
  name        = "${var.project}-${var.environment}-alb-sg"
  description = "Security group for ALB - allow HTTP and HTTPS from internet"
  vpc_id      = var.vpc_id

  # HTTP を許可（インターネット全体から）
  ingress {
    description = "HTTP from Internet"
    from_port   = 80
    to_port     = 80
    protocol    = "tcp"
    cidr_blocks = ["0.0.0.0/0"]
  }

  # HTTPS を許可（インターネット全体から）
  ingress {
    description = "HTTPS from Internet"
    from_port   = 443
    to_port     = 443
    protocol    = "tcp"
    cidr_blocks = ["0.0.0.0/0"]
  }

  # アウトバウンドはすべて許可（EC2 への転送のため）
  egress {
    description = "Allow all outbound"
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }

  tags = {
    Name = "${var.project}-${var.environment}-alb-sg"
  }
}

# ALB 本体: パブリックサブネットの 2 つの AZ に配置
resource "aws_lb" "this" {
  name               = "${var.project}-${var.environment}-alb"
  internal           = false # インターネット向け（false = external）
  load_balancer_type = "application"
  security_groups    = [aws_security_group.alb.id]
  subnets            = var.public_subnet_ids # 複数 AZ のパブリックサブネットに配置

  # 削除保護: 本番では true 推奨（誤削除防止）。今回は検証のため false
  enable_deletion_protection = false

  tags = {
    Name = "${var.project}-${var.environment}-alb"
  }
}

# ターゲットグループ: ALB がリクエストを転送する EC2 の集合
resource "aws_lb_target_group" "web" {
  name     = "${var.project}-${var.environment}-web-tg"
  port     = 80
  protocol = "HTTP"
  vpc_id   = var.vpc_id

  # ヘルスチェック: EC2 が正常かどうかを定期確認する設定
  health_check {
    enabled             = true
    path                = "/" # ルートパスに HTTP GET リクエストを送る
    port                = "traffic-port"
    healthy_threshold   = 2 # 2 回成功で「正常」と判定
    unhealthy_threshold = 3 # 3 回失敗で「異常」と判定
    timeout             = 5
    interval            = 30
    matcher             = "200" # HTTP 200 が返れば正常
  }

  tags = {
    Name = "${var.project}-${var.environment}-web-tg"
  }
}

# リスナー: ALB がポート 80 で受け取ったリクエストをターゲットグループに転送
resource "aws_lb_listener" "http" {
  load_balancer_arn = aws_lb.this.arn
  port              = 80
  protocol          = "HTTP"

  default_action {
    type             = "forward"
    target_group_arn = aws_lb_target_group.web.arn
  }
}
