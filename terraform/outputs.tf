# =============================================================
# AWS 3層 Web アーキテクチャ - 出力値定義
# =============================================================

output "alb_dns_name" {
  description = "ALB の DNS 名（ブラウザでアクセスするURL: http://<この値>）"
  value       = module.alb.alb_dns_name
}

output "rds_endpoint" {
  description = "RDS のエンドポイント（EC2 からの DB 接続先アドレス）"
  value       = module.rds.rds_endpoint
  sensitive   = true
}

output "ec2_instance_ids" {
  description = "EC2 インスタンス ID リスト（Part 2 で SNS 通知確認に使用）"
  value       = module.ec2.instance_ids
}

output "sns_topic_arn" {
  description = "SNS トピック ARN（Part 2: CPU 監視メッセージの送信先）"
  value       = module.monitoring.sns_topic_arn
}

output "sqs_queue_url" {
  description = "SQS キュー URL（Part 2: ポーリング用 URL）"
  value       = module.monitoring.sqs_queue_url
}
