# =============================================================
# WAF モジュール - AWS WAF v2
# ALB 前段に配置し、OWASP Top 10 等の代表的な攻撃パターンをブロック
# Shield Standard は AWS デフォルトで有効（追加設定不要）
# =============================================================

# WAF Web ACL
resource "aws_wafv2_web_acl" "this" {
  # checkov:skip=CKV2_AWS_31: dev/PoC のため WAF ログ記録は省略（本番では S3/CW Logs へ出力推奨）
  name  = "${var.project}-${var.environment}-waf"
  scope = "REGIONAL" # ALB 用（CloudFront の場合は CLOUDFRONT）

  # デフォルト: ルールに一致しないリクエストは許可
  default_action {
    allow {}
  }

  # ── ルール 1: AWS 共通マネージドルール（OWASP Top 10 対応）─────
  # SQL インジェクション・XSS・パストラバーサル等をブロック
  rule {
    name     = "AWSManagedRulesCommonRuleSet"
    priority = 1

    override_action {
      none {} # AWS の推奨アクション（Block）をそのまま使用
    }

    statement {
      managed_rule_group_statement {
        name        = "AWSManagedRulesCommonRuleSet"
        vendor_name = "AWS"
      }
    }

    visibility_config {
      cloudwatch_metrics_enabled = true
      metric_name                = "${var.project}-${var.environment}-common-rules"
      sampled_requests_enabled   = true
    }
  }

  # ── ルール 2: AWS IP レピュテーションリスト ───────────────────
  # AWS が管理する悪意のある IP アドレス（ボット・スキャナー等）をブロック
  rule {
    name     = "AWSManagedRulesAmazonIpReputationList"
    priority = 2

    override_action {
      none {}
    }

    statement {
      managed_rule_group_statement {
        name        = "AWSManagedRulesAmazonIpReputationList"
        vendor_name = "AWS"
      }
    }

    visibility_config {
      cloudwatch_metrics_enabled = true
      metric_name                = "${var.project}-${var.environment}-ip-reputation"
      sampled_requests_enabled   = true
    }
  }

  # ── ルール 3: 既知の悪意あるリクエストパターン ────────────────
  # Log4Shell・SSRF 等の既知エクスプロイトをブロック
  rule {
    name     = "AWSManagedRulesKnownBadInputsRuleSet"
    priority = 3

    override_action {
      none {}
    }

    statement {
      managed_rule_group_statement {
        name        = "AWSManagedRulesKnownBadInputsRuleSet"
        vendor_name = "AWS"
      }
    }

    visibility_config {
      cloudwatch_metrics_enabled = true
      metric_name                = "${var.project}-${var.environment}-bad-inputs"
      sampled_requests_enabled   = true
    }
  }

  # Web ACL 全体のメトリクス設定
  visibility_config {
    cloudwatch_metrics_enabled = true
    metric_name                = "${var.project}-${var.environment}-waf"
    sampled_requests_enabled   = true
  }

  tags = {
    Name = "${var.project}-${var.environment}-waf"
  }
}

# WAF Web ACL を ALB に関連付け
resource "aws_wafv2_web_acl_association" "this" {
  resource_arn = var.alb_arn
  web_acl_arn  = aws_wafv2_web_acl.this.arn
}
