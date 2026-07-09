# Changelog

All notable changes to this project will be documented in this file.

## [Unreleased]

## [1.8.0] - 2026-07-10

### Added
- `scripts/validate_config.py`: Python ユニットテスト 28 件 → 38 件に拡充（境界値・エラーメッセージ検証・複合パターン追加）
- `scripts/validate-config.ts`: TypeScript ユニットテスト 34 件 → 43 件に拡充（詳細ケース追加）
- `lambda_go/healthcheck/main_test.go`: Go ユニットテスト 19 件 → 29 件に拡充（エラー系・境界値追加）

### Fixed
- `scripts/validate-config.test.ts`: TypeScript 正規表現 `(?i:...)` → `/gi` フラグに修正（Node.js 非対応の Python 方言を解消）

### Changed
- CI: `actions/setup-node` v4 → v6（deprecated 対応）
- CI: `actions/checkout` v6 → v7

## [1.7.0] - 2026-06-16

### Changed
- actions/setup-go v5 -> v6
- actions/setup-python v5 -> v6

## [1.6.0] - 2026-06-09

### Added
- `scripts/validate_config.py`: Terraform 設定ファイルの静的検証スクリプト（Python）
  - 検証項目: required_version 制約 / required_providers / default_tags 必須タグ / 命名規則（var.project・var.environment） / ハードコードされたシークレット検出
- `scripts/test_validate_config.py`: pytest ユニットテスト 28 件
- `lambda_go/healthcheck/`: ALB / EC2 / RDS のヘルス状態を取得する Go Lambda 関数
  - AWS SDK Go v2 使用・interface によるモック設計でテスト容易性を確保
  - `main_test.go`: モックを使ったユニットテスト 13 件
- `.github/workflows/python-test.yml`: Python CI（ruff lint + pytest + スモークテスト）
- `.github/workflows/go-test.yml`: Go CI（build + test -race + vet）

### Fixed
- `terraform/backend.tf`: S3 バケット名に含まれていた実 AWS アカウント ID をプレースホルダー（`YOUR_ACCOUNT_ID`）に変更

## [1.5.0] - 2026-06-03

### Added
- `modules/waf/`: AWS WAF v2 モジュール追加（ALB 前段のセキュリティフィルター）
  - AWS Managed Rules 3セット適用: CommonRuleSet（OWASP Top 10）/ AmazonIpReputationList / KnownBadInputsRuleSet（Log4Shell 等）
  - WAF Web ACL を ALB に関連付け（`aws_wafv2_web_acl_association`）
  - Shield Standard はデフォルト有効のため追加設定不要
- `modules/alb/outputs.tf`: `alb_arn` output を追加（WAF 関連付け用）
- Checkov CI `soft_fail: false` 対応: WAF ログ（CKV2_AWS_31）を dev/PoC としてインラインスキップ

## [1.4.0] - 2026-05-27

### Added
- CI アクション更新: actions/checkout v4→v6 / aws-actions/configure-aws-credentials v4→v6 / actions/github-script v7→v9 / hashicorp/setup-terraform v3→v4

## [1.3.0] - 2026-05-25

### Added
- docs: guide.md にスクリーンショット参照を追加（フェーズ4・5）

## [1.2.0] - 2026-05-19

### Added
- CONTRIBUTING.md 追加（PR プロセス・スタイルガイド）
- NAT Gateway 演習スクショ・S3 Remote Backend 確認画面を追加

## [1.1.0] - 2026-05-18

### Added
- S3 + DynamoDB による Terraform Remote Backend を追加
- RDS `deletion_protection` を dev 向けに修正

## [1.0.0] - 2026-05-12

### Added
- CloudFormation テンプレート追加（Terraform / CDK / CloudFormation の IaC 3 種比較学習用）
- SECURITY.md 追加
- Dependabot 設定追加
- README にトラブルシューティング・ローカル開発テスト方法セクション追加

## [0.1.0] - 2026-03-12

### Added
- 初回実装：Terraform による AWS 3 層 Web アーキテクチャ（学習演習リポジトリとして公開化）
  - VPC / ALB / EC2 / RDS / CloudWatch 監視の全モジュール
  - Terraform CI ワークフロー（fmt / validate / plan）
  - AWS4 スタイルのアーキテクチャ構成図
  - GitHub Actions CI（Terraform lint + Checkov セキュリティスキャン）
- 「面接課題」表現を全ファイルから除去・公開学習演習リポジトリとして整備
