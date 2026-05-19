# =============================================================
# Terraform Remote Backend 設定（S3 + DynamoDB）
#
# 【事前準備】scripts/setup-backend.sh を実行して
#   S3 バケットと DynamoDB テーブルを作成してください。
#
# 【切り替え手順】
#   1. bucket を setup-backend.sh の出力値に書き換える
#   2. terraform init -migrate-state
#   3. ローカルの terraform.tfstate を削除
# =============================================================

terraform {
  backend "s3" {
    # setup-backend.sh 実行後に出力されるバケット名に書き換えること
    # 例: "123456789012-webapp-dev-tfstate"
    bucket = "580983239795-webapp-dev-tfstate"

    key    = "terraform-3tier-webapp/terraform.tfstate"
    region = "ap-northeast-1"

    # tfstate の暗号化（必須）
    encrypt = true

    # 同時実行防止のステートロック
    dynamodb_table = "webapp-dev-tflock"
  }
}
