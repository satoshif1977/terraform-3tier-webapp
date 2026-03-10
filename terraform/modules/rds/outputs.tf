output "rds_endpoint" {
  description = "RDS のエンドポイント（EC2 からの接続先: hostname:3306）"
  value       = aws_db_instance.this.endpoint
  sensitive   = true
}

output "rds_id" {
  description = "RDS インスタンス ID"
  value       = aws_db_instance.this.id
}

output "db_name" {
  description = "データベース名"
  value       = aws_db_instance.this.db_name
}
