#!/bin/bash
# =============================================================
# SQS ポーリングスクリプト（課題2）
# SQS キューを継続的に監視し、届いたメッセージをコマンドラインに表示する
#
# 使い方:
#   chmod +x sqs_poller.sh
#   ./sqs_poller.sh <SQS_QUEUE_URL> [AWS_REGION]
#
# または環境変数で指定:
#   export SQS_QUEUE_URL="https://sqs.ap-northeast-1.amazonaws.com/..."
#   ./sqs_poller.sh
# =============================================================
set -euo pipefail

# ── 引数・環境変数の処理 ──────────────────────────────────────
SQS_QUEUE_URL="${1:-${SQS_QUEUE_URL:-}}"
AWS_REGION="${2:-${AWS_REGION:-ap-northeast-1}}"

if [ -z "$SQS_QUEUE_URL" ]; then
  echo "[ERROR] SQS_QUEUE_URL が指定されていません"
  echo "使い方: $0 <SQS_QUEUE_URL> [AWS_REGION]"
  exit 1
fi

echo "=============================================="
echo " SQS ポーリング開始"
echo " キュー: $SQS_QUEUE_URL"
echo " リージョン: $AWS_REGION"
echo " Ctrl+C で停止"
echo "=============================================="

# ── ポーリングループ ──────────────────────────────────────────
while true; do
  # SQS からメッセージを受信（Long Polling: 20秒待機）
  # Long Polling にすることでポーリングコストを削減し、レイテンシーも改善
  RESPONSE=$(aws sqs receive-message \
    --queue-url "$SQS_QUEUE_URL" \
    --region "$AWS_REGION" \
    --max-number-of-messages 10 \
    --wait-time-seconds 20 \
    --output json 2>/dev/null || echo '{}')

  # メッセージが存在する場合のみ処理
  if echo "$RESPONSE" | grep -q '"Messages"'; then
    TIMESTAMP=$(date '+%Y-%m-%d %H:%M:%S')

    # 各メッセージを処理
    echo "$RESPONSE" | python3 -c "
import sys, json

data = json.load(sys.stdin)
messages = data.get('Messages', [])

for msg in messages:
    body_str = msg.get('Body', '{}')
    receipt_handle = msg.get('ReceiptHandle', '')

    # SNS でラップされている場合は中身を取り出す
    try:
        body = json.loads(body_str)
        if 'Message' in body:
            inner = json.loads(body['Message'])
        else:
            inner = body
    except:
        inner = {'raw': body_str}

    instance_id = inner.get('instance_id', 'unknown')
    cpu_usage   = inner.get('cpu_usage_percent', inner.get('cpu_usage', 'unknown'))
    az          = inner.get('availability_zone', 'unknown')
    timestamp   = inner.get('timestamp', 'unknown')

    print(f'[{timestamp}] Instance: {instance_id} | AZ: {az} | CPU: {cpu_usage}%')
    print(f'  ReceiptHandle: {receipt_handle[:40]}...')
" 2>/dev/null || echo "[$TIMESTAMP] メッセージを受信: $RESPONSE"

    # 受信したメッセージを SQS から削除（再処理を防ぐ）
    RECEIPT_HANDLES=$(echo "$RESPONSE" | python3 -c "
import sys, json
data = json.load(sys.stdin)
for msg in data.get('Messages', []):
    print(msg['ReceiptHandle'])
" 2>/dev/null)

    while IFS= read -r HANDLE; do
      if [ -n "$HANDLE" ]; then
        aws sqs delete-message \
          --queue-url "$SQS_QUEUE_URL" \
          --receipt-handle "$HANDLE" \
          --region "$AWS_REGION" \
          > /dev/null 2>&1
      fi
    done <<< "$RECEIPT_HANDLES"
  fi
done
