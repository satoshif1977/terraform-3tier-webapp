# Changelog

All notable changes to this project will be documented in this file.

## [Unreleased]

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
