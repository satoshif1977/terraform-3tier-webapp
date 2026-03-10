#!/bin/bash
# =============================================================
# EC2 起動時自動実行スクリプト（User Data）
# Web サーバーのセットアップ + 課題2 の CPU 監視 cron 設定
# =============================================================
set -euo pipefail

# ── パッケージ更新・Web サーバーインストール ──────────────────
dnf update -y
dnf install -y httpd

# ── Apache 起動・自動起動設定 ─────────────────────────────────
systemctl start httpd
systemctl enable httpd

# ── テスト用 Web ページ作成 ───────────────────────────────────
# インスタンス ID・AZ をメタデータから取得して表示（動作確認用）
INSTANCE_ID=$(curl -s http://169.254.169.254/latest/meta-data/instance-id)
AZ=$(curl -s http://169.254.169.254/latest/meta-data/placement/availability-zone)

cat > /var/www/html/index.html <<EOF
<!DOCTYPE html>
<html>
<head>
  <meta charset="UTF-8">
  <title>Web Server</title>
  <style>
    body { font-family: Arial, sans-serif; text-align: center; padding: 50px; background: #f0f8ff; }
    .box { background: white; padding: 30px; border-radius: 10px; display: inline-block; box-shadow: 0 2px 10px rgba(0,0,0,0.1); }
    h1 { color: #232f3e; }
    p { color: #555; font-size: 1.1em; }
  </style>
</head>
<body>
  <div class="box">
    <h1>Hello from AWS!</h1>
    <p><strong>Instance ID:</strong> $INSTANCE_ID</p>
    <p><strong>Availability Zone:</strong> $AZ</p>
    <p>このページは ALB 経由でアクセスされています</p>
  </div>
</body>
</html>
EOF

# ── 課題2: CPU 監視スクリプトをインストール ───────────────────
SNS_TOPIC_ARN="${sns_topic_arn}"
AWS_REGION="${aws_region}"

cat > /usr/local/bin/cpu_monitor.sh <<'SCRIPT'
#!/bin/bash
# CPU 使用率を取得して SNS に通知するスクリプト

INSTANCE_ID=$(curl -s http://169.254.169.254/latest/meta-data/instance-id)
AZ=$(curl -s http://169.254.169.254/latest/meta-data/placement/availability-zone)

# CPU 使用率を取得（idle 率を 100 から引いてビジー率を計算）
CPU_IDLE=$(top -bn1 | grep "Cpu(s)" | awk '{print $8}' | cut -d'%' -f1)
CPU_USAGE=$(echo "100 - $CPU_IDLE" | bc)

TIMESTAMP=$(date -u +"%Y-%m-%dT%H:%M:%SZ")

MESSAGE=$(cat <<JSON
{
  "instance_id": "$INSTANCE_ID",
  "availability_zone": "$AZ",
  "cpu_usage_percent": "$CPU_USAGE",
  "timestamp": "$TIMESTAMP"
}
JSON
)

aws sns publish \
  --topic-arn "SNS_PLACEHOLDER" \
  --message "$MESSAGE" \
  --subject "CPU Monitor: $INSTANCE_ID" \
  --region "REGION_PLACEHOLDER"

echo "[$TIMESTAMP] CPU: $CPU_USAGE% を SNS に送信しました"
SCRIPT

# 環境変数をスクリプトに埋め込む
sed -i "s|SNS_PLACEHOLDER|$SNS_TOPIC_ARN|g" /usr/local/bin/cpu_monitor.sh
sed -i "s|REGION_PLACEHOLDER|$AWS_REGION|g" /usr/local/bin/cpu_monitor.sh
chmod +x /usr/local/bin/cpu_monitor.sh

# ── cron 設定: 1 分ごとに CPU 監視スクリプトを実行 ────────────
echo "* * * * * root /usr/local/bin/cpu_monitor.sh >> /var/log/cpu_monitor.log 2>&1" \
  > /etc/cron.d/cpu_monitor
chmod 644 /etc/cron.d/cpu_monitor

echo "セットアップ完了: $(date)" >> /var/log/user_data.log
