output "alb_dns_name" {
  description = "ALB の DNS 名（アクセス URL）"
  value       = aws_lb.this.dns_name
}

output "alb_sg_id" {
  description = "ALB セキュリティグループ ID（EC2 SG のインバウンドルールに使用）"
  value       = aws_security_group.alb.id
}

output "target_group_arn" {
  description = "ターゲットグループ ARN（EC2 登録に使用）"
  value       = aws_lb_target_group.web.arn
}
