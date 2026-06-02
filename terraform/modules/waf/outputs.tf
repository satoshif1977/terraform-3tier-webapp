output "web_acl_arn" {
  description = "WAF Web ACL の ARN"
  value       = aws_wafv2_web_acl.this.arn
}

output "web_acl_id" {
  description = "WAF Web ACL の ID"
  value       = aws_wafv2_web_acl.this.id
}
