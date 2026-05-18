#!/bin/bash
# =============================================================
# Terraform Remote Backend ブートストラップスクリプト
#
# 用途: S3 バケット（tfstate 保管）と DynamoDB テーブル（ロック）を作成する
# 実行: terraform init の前に一度だけ実行する
#
# 使い方:
#   ./scripts/setup-backend.sh [project] [environment] [region]
#
# 例:
#   ./scripts/setup-backend.sh webapp dev ap-northeast-1
# =============================================================
set -euo pipefail

PROJECT="${1:-webapp}"
ENVIRONMENT="${2:-dev}"
REGION="${3:-ap-northeast-1}"

# AWS アカウント ID を自動取得（グローバルユニーク性を担保）
ACCOUNT_ID=$(aws sts get-caller-identity --query Account --output text)
BUCKET_NAME="${ACCOUNT_ID}-${PROJECT}-${ENVIRONMENT}-tfstate"
TABLE_NAME="${PROJECT}-${ENVIRONMENT}-tflock"

echo "=== Terraform Remote Backend セットアップ ==="
echo "バケット名    : ${BUCKET_NAME}"
echo "テーブル名    : ${TABLE_NAME}"
echo "リージョン    : ${REGION}"
echo ""

# ── S3 バケット作成 ──────────────────────────────────────────
echo "[1/5] S3 バケット作成中..."
if aws s3api head-bucket --bucket "${BUCKET_NAME}" 2>/dev/null; then
  echo "  既存バケットを使用します: ${BUCKET_NAME}"
else
  if [ "${REGION}" = "us-east-1" ]; then
    aws s3api create-bucket \
      --bucket "${BUCKET_NAME}" \
      --region "${REGION}"
  else
    aws s3api create-bucket \
      --bucket "${BUCKET_NAME}" \
      --region "${REGION}" \
      --create-bucket-configuration LocationConstraint="${REGION}"
  fi
  echo "  作成完了: ${BUCKET_NAME}"
fi

# ── バージョニング有効化 ─────────────────────────────────────
echo "[2/5] バージョニング有効化中..."
aws s3api put-bucket-versioning \
  --bucket "${BUCKET_NAME}" \
  --versioning-configuration Status=Enabled
echo "  完了"

# ── サーバーサイド暗号化（AES-256）────────────────────────────
echo "[3/5] 暗号化（SSE-S3）設定中..."
aws s3api put-bucket-encryption \
  --bucket "${BUCKET_NAME}" \
  --server-side-encryption-configuration '{
    "Rules": [{
      "ApplyServerSideEncryptionByDefault": {
        "SSEAlgorithm": "AES256"
      },
      "BucketKeyEnabled": true
    }]
  }'
echo "  完了"

# ── パブリックアクセスブロック ───────────────────────────────
echo "[4/5] パブリックアクセスブロック設定中..."
aws s3api put-public-access-block \
  --bucket "${BUCKET_NAME}" \
  --public-access-block-configuration \
    BlockPublicAcls=true,IgnorePublicAcls=true,BlockPublicPolicy=true,RestrictPublicBuckets=true
echo "  完了"

# ── DynamoDB テーブル作成（ステートロック用）────────────────
echo "[5/5] DynamoDB テーブル作成中..."
if aws dynamodb describe-table --table-name "${TABLE_NAME}" --region "${REGION}" 2>/dev/null; then
  echo "  既存テーブルを使用します: ${TABLE_NAME}"
else
  aws dynamodb create-table \
    --table-name "${TABLE_NAME}" \
    --attribute-definitions AttributeName=LockID,AttributeType=S \
    --key-schema AttributeName=LockID,KeyType=HASH \
    --billing-mode PAY_PER_REQUEST \
    --region "${REGION}"
  echo "  作成完了: ${TABLE_NAME}"
fi

# ── 完了メッセージ ───────────────────────────────────────────
echo ""
echo "=============================================="
echo "セットアップ完了！"
echo "=============================================="
echo ""
echo "次のステップ: terraform/backend.tf を以下の値で更新してください"
echo ""
echo "  bucket         = \"${BUCKET_NAME}\""
echo "  dynamodb_table = \"${TABLE_NAME}\""
echo ""
echo "その後、以下のコマンドでローカル tfstate を S3 へ移行:"
echo ""
echo "  cd terraform"
echo "  terraform init -migrate-state"
echo ""
