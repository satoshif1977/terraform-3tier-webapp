# =============================================================
# 監視モジュール（Part 2）
# SNS → SQS のメッセージングパイプラインを構築
# EC2 が CPU 使用率を SNS に送信 → SQS が受信 → 別の EC2 がポーリング
# =============================================================

# SNS トピック: メッセージの送信先となるトピック（パブリッシャーはここへ送る）
resource "aws_sns_topic" "cpu_monitor" {
  name = "${var.project}-${var.environment}-cpu-monitor"

  tags = {
    Name = "${var.project}-${var.environment}-cpu-monitor"
  }
}

# SQS キュー: SNS からのメッセージを溜めておくキュー（サブスクライバーがポーリングして取得）
resource "aws_sqs_queue" "cpu_monitor" {
  name = "${var.project}-${var.environment}-cpu-monitor"

  # メッセージ保持期間: キューに届いたメッセージを何秒保持するか（デフォルト4日）
  message_retention_seconds = 86400 # 1 日

  # 可視性タイムアウト: メッセージを受信後、他のワーカーに見えなくする時間
  visibility_timeout_seconds = 60

  tags = {
    Name = "${var.project}-${var.environment}-cpu-monitor"
  }
}

# SQS キューポリシー: SNS がこのキューにメッセージを送ることを許可
resource "aws_sqs_queue_policy" "cpu_monitor" {
  queue_url = aws_sqs_queue.cpu_monitor.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect    = "Allow"
      Principal = { Service = "sns.amazonaws.com" }
      Action    = "sqs:SendMessage"
      Resource  = aws_sqs_queue.cpu_monitor.arn
      Condition = {
        ArnEquals = {
          "aws:SourceArn" = aws_sns_topic.cpu_monitor.arn
        }
      }
    }]
  })
}

# SNS サブスクリプション: SNS トピックに届いたメッセージを SQS に転送する設定
resource "aws_sns_topic_subscription" "sqs" {
  topic_arn = aws_sns_topic.cpu_monitor.arn
  protocol  = "sqs"
  endpoint  = aws_sqs_queue.cpu_monitor.arn

  # raw_message_delivery: true にすると SNS のラッパーなしの生メッセージを SQS に配信
  raw_message_delivery = false
}
