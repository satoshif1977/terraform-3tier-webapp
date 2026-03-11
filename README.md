# AWS エンジニア面接課題

## アーキテクチャ概要

```
インターネット
      │
  [ALB] ←── パブリックサブネット（AZ-a / AZ-c）
      │
  ┌───┴───┐
[EC2-1]  [EC2-2] ←── プライベートサブネット（AZ-a / AZ-c）
  │    Apache  │
  └───┬───┘
      │
   [RDS MySQL Multi-AZ] ←── プライベートサブネット（AZ-a / AZ-c）
```

### 課題2 メッセージングアーキテクチャ

```
[EC2-1] ─ cron 1分ごと ─→ [SNS Topic]
                                  │
                          [SQS Queue（サブスクリプション）]
                                  │
              [EC2-2] ─ ポーリング ─→ CLI 表示
```

---

## 構成ファイル

```
.
├── terraform/
│   ├── main.tf                    # モジュール呼び出し・全体設定
│   ├── variables.tf               # 入力変数定義
│   ├── outputs.tf                 # 出力値定義
│   ├── terraform.tfvars.example   # 設定例（コピーして使用）
│   └── modules/
│       ├── vpc/       # VPC・サブネット・NAT GW・Flow Logs
│       ├── alb/       # Application Load Balancer
│       ├── ec2/       # EC2 × 2台・IAM ロール・User Data
│       ├── rds/       # RDS MySQL Multi-AZ
│       └── monitoring/# SNS・SQS（課題2）
├── scripts/
│   └── sqs_poller.sh  # SQS ポーリングスクリプト（課題2 EC2-B 用）
└── docs/
    ├── security.md    # 追加セキュリティ対策
    └── availability.md# 可用性確保の構成
```

---

## 当日の手順

### 前提条件

- AWS CLI がインストール・設定済みであること
- Terraform がインストール済みであること（`terraform version` で確認）

### Step 1: 事前準備（5分）

```bash
# リポジトリをクローン
git clone https://github.com/<your-account>/interview-challenge.git
cd interview-challenge/terraform

# terraform.tfvars を作成
cp terraform.tfvars.example terraform.tfvars

# terraform.tfvars を編集（db_password を変更）
# db_password = "your-secure-password"
```

### Step 2: 初期化（2分）

```bash
terraform init
```

### Step 3: 変更内容を確認（3分）

```bash
terraform plan
# 作成されるリソース数を確認（約 30〜35 リソース）
```

### Step 4: デプロイ（約 15〜20分）

```bash
terraform apply
# "yes" を入力して実行
# ※ RDS Multi-AZ の作成に 10〜15 分かかります
```

### Step 5: 動作確認（課題1）

```bash
# ALB の DNS 名を確認
terraform output alb_dns_name

# ブラウザでアクセス
# http://<alb_dns_name>
# → インスタンス ID と AZ が表示されれば成功
# → リロードで異なるインスタンス ID が表示される（ラウンドロビン）

# EC2 から RDS への接続確認（SSM Session Manager 経由）
# AWSコンソール → EC2 → インスタンスを選択 → 接続 → Session Manager
mysql -h <rds_endpoint> -u admin -p appdb
```

### Step 6: 課題2 の動作確認

```bash
# SQS キュー URL を確認
terraform output sqs_queue_url

# EC2-B（2台目）で SQS ポーリングを実行
# SSM Session Manager で EC2-2 に接続してから:
chmod +x /path/to/sqs_poller.sh
./sqs_poller.sh <SQS_QUEUE_URL>

# EC2-1 の cron が 1 分以内に CPU データを送信
# EC2-2 のターミナルにメッセージが表示されれば成功
```

### Step 7: 後片付け（重要）

```bash
# 課題終了後、必ずリソースを削除してコストを止める
terraform destroy
# "yes" を入力
```

---

## 主要リソースとその役割

| リソース | 役割 | なぜこの設計か |
|---------|------|---------------|
| VPC | ネットワーク空間の分離 | セキュリティの基盤 |
| パブリックサブネット | ALB・NAT GW を配置 | インターネットからアクセスが必要なリソースのみ |
| プライベートサブネット | EC2・RDS を配置 | 直接インターネット公開しない（セキュリティ） |
| ALB | リクエスト分散 | EC2 の増減に対応、ヘルスチェックで自動切替 |
| EC2 × 2（異なる AZ） | Web サーバー | 1 台が落ちても継続稼働（可用性） |
| RDS Multi-AZ | データベース | AZ 障害時に自動フェイルオーバー（約 60 秒） |
| NAT Gateway | Private → Internet | アウトバウンドのみ許可（セキュアな外部通信） |
| SNS | メッセージブローカー | Pub/Sub パターンで送受信を疎結合に |
| SQS | メッセージキュー | 非同期処理・バッファリング |

---

## 推定コスト（東京リージョン / 1時間）

| リソース | コスト/時間 |
|---------|-----------|
| EC2 t3.micro × 2 | $0.027 |
| RDS db.t3.micro Multi-AZ | $0.040 |
| ALB | $0.024 |
| NAT Gateway | $0.062 |
| **合計** | **約 $0.15/時間（約 22円）** |

> ⚠️ 使い終わったら `terraform destroy` で必ず削除してください

---

## 動作確認スクリーンショット

### 1. terraform apply 完了

![terraform apply 完了](docs/screenshots/01_terraform_apply_complete.png)

ALB・EC2・RDS・SNS・SQS を含む全リソースのデプロイが完了した状態。

### 2. Hello from AWS!（ALB 経由でアクセス成功）

![Hello from AWS](docs/screenshots/06_hello_from_aws.png)

ALB の DNS 名にブラウザでアクセスし、EC2 からレスポンスが返ってきた画面。

### 3. EC2 インスタンス 2台 実行中

![EC2 インスタンス](docs/screenshots/05_ec2_instances_running.png)

AZ-a / AZ-c の異なるアベイラビリティゾーンに 2台が正常稼働中。

### 4. ターゲットグループ モニタリング

![ターゲットグループ](docs/screenshots/04_target_group_monitoring.png)

ALB のターゲットグループにリクエストが分散されている様子。

### 5. RDS コンソール（利用可能）

![RDS コンソール](docs/screenshots/02_rds_available.png)

interview-dev-mysql が「利用可能」ステータスで起動中。

### 6. RDS 詳細（Multi-AZ 有効）

![RDS 詳細](docs/screenshots/03_rds_detail_multaz.png)

db.t3.micro・Multi-AZ 有効・VPC 内プライベートサブネットに配置された状態。
