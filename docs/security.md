# 追加セキュリティ対策

## 現在の構成で実装済みのセキュリティ

| 対策 | 実装内容 |
|------|---------|
| ネットワーク分離 | Public/Private サブネット分離、EC2・RDS はプライベートに配置 |
| 最小権限 SG | EC2 は ALB からの HTTP(80) のみ受け付け、RDS は EC2 からの 3306 のみ |
| 二層防御 | Security Group + Network ACL の 2 段階フィルタリング |
| 暗号化 | EC2 EBS・RDS ストレージを暗号化（`encrypted = true`） |
| IAM ロール | アクセスキー不要。EC2 にロールをアタッチして最小権限で AWS API を使用 |
| VPC Flow Logs | 全ネットワークトラフィックを CloudWatch に記録（インシデント調査用） |
| SSM Session Manager | SSH ポート不要。コンソールから安全にアクセス可能 |

---

## 追加で検討すべきセキュリティ対策

### 1. AWS WAF（Web Application Firewall）

**目的**: ALB の前段に配置し、Web 攻撃を防ぐ

```
Internet → WAF → ALB → EC2
```

**防御できる攻撃例**:
- SQL インジェクション
- クロスサイトスクリプティング（XSS）
- DDoS 攻撃（AWS Shield Standard と連携）
- ボット・スクレイピング

**実装方法**: `aws_wafv2_web_acl` を ALB に関連付け

---

### 2. HTTPS 化（ACM + ALB リスナー）

**目的**: 通信を暗号化し、盗聴・改ざんを防ぐ

```
現在: HTTP(80) のみ
改善: HTTPS(443) を追加、HTTP → HTTPS リダイレクト
```

**実装手順**:
1. ACM（AWS Certificate Manager）で SSL 証明書を発行（無料）
2. ALB に HTTPS リスナー（443）を追加
3. HTTP(80) → HTTPS(443) リダイレクトを設定

---

### 3. Secrets Manager によるパスワード管理

**目的**: DB パスワードのハードコーディングを排除

```
現在: terraform.tfvars にパスワードを記載（漏洩リスク）
改善: Secrets Manager に保存 → EC2・Lambda が動的に取得
```

**メリット**:
- パスワードのコード管理不要
- 自動ローテーション機能
- アクセスログが CloudTrail に記録される

---

### 4. AWS CloudTrail

**目的**: AWS API 操作の全ログを記録（誰が・いつ・何をしたか）

**有効化で検出できること**:
- 不正な IAM ロール変更
- セキュリティグループの変更
- S3 バケットへの不審なアクセス

---

### 5. Amazon GuardDuty

**目的**: 機械学習を使った脅威検出サービス

**検出できる脅威例**:
- EC2 からの不審な外部通信（マルウェア感染の疑い）
- 認証情報の不正使用
- 暗号通貨マイニング

---

### 6. RDS の強化

| 対策 | 内容 |
|------|------|
| 自動バックアップ | 保持期間を 7→30 日に延長 |
| スナップショット | 手動スナップショットを定期取得 |
| 監査ログ | MySQL の General/Slow Query Log を CloudWatch に送信 |
| IAM 認証 | パスワードの代わりに IAM トークンで DB 接続 |

---

### 7. セキュリティ対策の優先順位（コストとのバランス）

```
高優先度（すぐ実装すべき）
├── HTTPS 化（ACM は無料）
├── CloudTrail（月 $2 程度）
└── Secrets Manager（月 $0.40/シークレット）

中優先度（重要システムに）
├── WAF（月 $5〜）
└── GuardDuty（月 $10〜）

低優先度（大規模・高セキュリティ要件）
├── AWS Config（コンプライアンス監視）
├── Security Hub（セキュリティ統合管理）
└── VPC Endpoint（AWS サービスをプライベート接続）
```
