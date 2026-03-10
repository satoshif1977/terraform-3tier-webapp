output "sns_topic_arn" {
  description = "SNS トピック ARN（EC2 の cpu_monitor.sh がメッセージを送る先）"
  value       = aws_sns_topic.cpu_monitor.arn
}

output "sqs_queue_url" {
  description = "SQS キュー URL（sqs_poller.sh がポーリングする対象）"
  value       = aws_sqs_queue.cpu_monitor.url
}

output "sqs_queue_arn" {
  description = "SQS キュー ARN"
  value       = aws_sqs_queue.cpu_monitor.arn
}
