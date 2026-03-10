output "instance_ids" {
  description = "EC2 インスタンス ID リスト"
  value       = aws_instance.web[*].id
}

output "ec2_sg_id" {
  description = "EC2 セキュリティグループ ID（RDS SG のインバウンドルールに使用）"
  value       = aws_security_group.ec2.id
}

output "iam_role_name" {
  description = "EC2 IAM ロール名"
  value       = aws_iam_role.ec2.name
}
