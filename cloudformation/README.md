# CloudFormation テンプレート

Terraform版（`terraform/`）と**同じ3層構成**をCloudFormationで実装した比較用テンプレートです。

## 構成リソース

```
Internet
  │
  ▼
ALB（パブリックサブネット × 2AZ）
  │
  ▼
EC2 / Auto Scaling Group（プライベートサブネット × 2AZ）
  │
  ▼
RDS MySQL 8.0（プライベートサブネット × 2AZ）
```

| リソース | CloudFormation | Terraform |
|---|---|---|
| VPC / サブネット | `AWS::EC2::VPC` | `module.vpc` |
| ALB | `AWS::ElasticLoadBalancingV2::LoadBalancer` | `module.alb` |
| EC2（ASG） | `AWS::AutoScaling::AutoScalingGroup` | `module.ec2` |
| RDS | `AWS::RDS::DBInstance` | `module.rds` |
| CloudWatch | `AWS::CloudWatch::Alarm` | `module.monitoring` |

## デプロイ手順

### 前提条件

- AWS CLI インストール済み・認証設定済み
- ターゲットリージョン: `ap-northeast-1`（東京）

### 1. スタック作成

```bash
aws cloudformation create-stack \
  --stack-name webapp-dev \
  --template-body file://template.yaml \
  --parameters \
    ParameterKey=ProjectName,ParameterValue=webapp \
    ParameterKey=Environment,ParameterValue=dev \
    ParameterKey=DBPassword,ParameterValue=YourSecurePassword123 \
  --capabilities CAPABILITY_NAMED_IAM \
  --region ap-northeast-1
```

### 2. デプロイ状況確認

```bash
aws cloudformation describe-stacks \
  --stack-name webapp-dev \
  --query 'Stacks[0].StackStatus'
```

### 3. URL 確認

```bash
aws cloudformation describe-stacks \
  --stack-name webapp-dev \
  --query 'Stacks[0].Outputs[?OutputKey==`ALBURL`].OutputValue' \
  --output text
```

### 4. スタック削除

```bash
aws cloudformation delete-stack --stack-name webapp-dev
```

## Terraform との比較ポイント

| 観点 | CloudFormation | Terraform |
|---|---|---|
| State 管理 | AWS が自動管理 | S3 + DynamoDB が必要 |
| ドリフト検出 | コンソールから確認可能 | `terraform plan` で確認 |
| ロールバック | 自動（デフォルト） | 手動対応が必要 |
| マルチクラウド | AWS のみ | 対応可能 |
| 学習コスト | AWS のみ学習 | HCL 言語学習が必要 |
| エコシステム | AWS ネイティブ | サードパーティプロバイダ豊富 |
