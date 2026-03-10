# =============================================================
# VPC モジュール - 出力値定義
# =============================================================

output "vpc_id" {
  description = "VPC の ID"
  value       = aws_vpc.this.id
}

output "vpc_cidr" {
  description = "VPC の CIDR ブロック"
  value       = aws_vpc.this.cidr_block
}

output "public_subnet_ids" {
  description = "パブリックサブネットの ID リスト（ALB・NAT GW を置く場所）"
  value       = aws_subnet.public[*].id
}

output "private_subnet_ids" {
  description = "プライベートサブネットの ID リスト（EC2・RDS を置く場所）"
  value       = aws_subnet.private[*].id
}

output "internet_gateway_id" {
  description = "Internet Gateway の ID"
  value       = aws_internet_gateway.this.id
}

output "nat_gateway_id" {
  description = "NAT Gateway の ID"
  value       = var.enable_nat_gateway ? aws_nat_gateway.this[0].id : null
}

output "flow_log_group_name" {
  description = "VPC Flow Logs の CloudWatch Log Group 名"
  value       = var.enable_flow_logs ? aws_cloudwatch_log_group.flow_log[0].name : null
}
