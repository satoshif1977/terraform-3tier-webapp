"use strict";

/**
 * Terraform 設定検証スクリプト 詳細ユニットテスト
 *
 * メッセージ内容・境界値・複合ケース・レポート構造を中心に検証する。
 */

import {
  checkRequiredVersion,
  checkRequiredProviders,
  checkDefaultTags,
  checkNamingConvention,
  checkRequiredVariables,
  checkNoHardcodedSecrets,
  validateContent,
  formatReport,
} from "./validate-config";

// ── フィクスチャ ──────────────────────────────────────────────

const VALID_MAIN_TF = `
terraform {
  required_version = ">= 1.5.0"
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
  }
}

provider "aws" {
  region = var.aws_region
  default_tags {
    tags = {
      Project     = var.project
      Environment = var.environment
      ManagedBy   = "Terraform"
    }
  }
}

resource "aws_vpc" "main" {
  name = "\${var.project}-\${var.environment}-vpc"
}
`;

const VALID_VARIABLES_TF = `
variable "aws_region" { default = "ap-northeast-1" }
variable "project"    { default = "myapp" }
variable "environment" { default = "dev" }
`;

// ── checkRequiredVersion 詳細 ─────────────────────────────────

describe("checkRequiredVersion (詳細)", () => {
  test(">= 2.0.0 形式でも PASS", () => {
    const result = checkRequiredVersion('required_version = ">= 2.0.0"');
    expect(result.passed).toBe(true);
    expect(result.message).toContain(">= 2.0.0");
  });
});

// ── checkRequiredProviders 詳細 ───────────────────────────────

describe("checkRequiredProviders (詳細)", () => {
  test("PASS message に 'aws が含まれています' が入る", () => {
    const result = checkRequiredProviders(VALID_MAIN_TF);
    expect(result.message).toContain("aws が含まれています");
  });

  test("FAIL message に 'aws が見つかりません' が入る", () => {
    const result = checkRequiredProviders("terraform {}");
    expect(result.message).toContain("aws が見つかりません");
  });

  test("空文字列は FAIL", () => {
    expect(checkRequiredProviders("").passed).toBe(false);
  });
});

// ── checkDefaultTags 詳細 ─────────────────────────────────────

describe("checkDefaultTags (詳細)", () => {
  test("PASS message に 3 つのタグ名がすべて含まれる", () => {
    const result = checkDefaultTags(VALID_MAIN_TF);
    expect(result.message).toContain("Project");
    expect(result.message).toContain("Environment");
    expect(result.message).toContain("ManagedBy");
  });

  test("Environment のみ欠けた場合 message に Project は含まれない", () => {
    const content = `
      default_tags {
        tags = {
          Project   = "myapp"
          ManagedBy = "Terraform"
        }
      }
    `;
    const result = checkDefaultTags(content);
    expect(result.passed).toBe(false);
    expect(result.message).toContain("Environment");
    expect(result.message).not.toContain("Project");
    expect(result.message).not.toContain("ManagedBy");
  });
});

// ── checkNamingConvention 詳細 ────────────────────────────────

describe("checkNamingConvention (詳細)", () => {
  test("PASS message に '命名規則を確認しました' が含まれる", () => {
    const result = checkNamingConvention(VALID_MAIN_TF);
    expect(result.message).toContain("命名規則を確認しました");
  });

  test("var.environment のみ使用は FAIL で var.project が message に含まれる", () => {
    const content = 'name = "${var.environment}-vpc"';
    const result = checkNamingConvention(content);
    expect(result.passed).toBe(false);
    expect(result.message).toContain("var.project");
    expect(result.message).not.toContain("var.environment");
  });

  test("FAIL 時の detail に '${var.project}' が含まれる", () => {
    const result = checkNamingConvention("");
    expect(result.detail).toContain("${var.project}");
  });
});

// ── checkRequiredVariables 詳細 ───────────────────────────────

describe("checkRequiredVariables (詳細)", () => {
  test("PASS message に aws_region が含まれる", () => {
    const result = checkRequiredVariables(VALID_VARIABLES_TF);
    expect(result.message).toContain("aws_region");
  });

  test("余分な変数が定義されていても PASS", () => {
    const content = `
      variable "aws_region" {}
      variable "project" {}
      variable "environment" {}
      variable "extra_var" {}
    `;
    expect(checkRequiredVariables(content).passed).toBe(true);
  });
});

// ── checkNoHardcodedSecrets 詳細 ──────────────────────────────

describe("checkNoHardcodedSecrets (詳細)", () => {
  test("token に直接値があれば FAIL", () => {
    const content = 'token = "hardcodedvalue12345"';
    expect(checkNoHardcodedSecrets(content).passed).toBe(false);
  });

  test("8 文字未満のパスワードはパターン外のため PASS", () => {
    // secretPattern は {8,} を要求するため短い値はスキップ
    const content = 'password = "short"';
    expect(checkNoHardcodedSecrets(content).passed).toBe(true);
  });

  test("PASS message に '見つかりませんでした' が含まれる", () => {
    const result = checkNoHardcodedSecrets(VALID_MAIN_TF);
    expect(result.message).toContain("見つかりませんでした");
  });
});

// ── validateContent 詳細 ──────────────────────────────────────

describe("validateContent (詳細)", () => {
  test("シークレット混入の main.tf は hasErrors = true", () => {
    const secretMain = VALID_MAIN_TF + '\naccess_key = "AKIAIOSFODNN7EXAMPLE"';
    const report = validateContent(secretMain, VALID_VARIABLES_TF);
    expect(report.hasErrors).toBe(true);
  });

  test("results[0] の name は required_version", () => {
    const report = validateContent(VALID_MAIN_TF, VALID_VARIABLES_TF);
    expect(report.results[0].name).toBe("required_version");
  });

  test("results のすべての要素に name フィールドが存在する", () => {
    const report = validateContent(VALID_MAIN_TF, VALID_VARIABLES_TF);
    for (const r of report.results) {
      expect(r.name).toBeDefined();
      expect(r.name.length).toBeGreaterThan(0);
    }
  });
});

// ── formatReport 詳細 ─────────────────────────────────────────

describe("formatReport (詳細)", () => {
  test("全 PASS レポートの結果行に件数が含まれる", () => {
    const report = validateContent(VALID_MAIN_TF, VALID_VARIABLES_TF);
    const output = formatReport(report);
    // 「すべてのチェックが通過しました (6 件)」形式
    expect(output).toContain("6 件");
  });

  test("FAIL がある場合 detail テキストが出力に含まれる", () => {
    // checkNamingConvention が FAIL → detail あり
    const badMain = VALID_MAIN_TF.replace(/var\.project/g, "myapp").replace(
      /var\.environment/g,
      "dev"
    );
    const report = validateContent(badMain, VALID_VARIABLES_TF);
    const output = formatReport(report);
    expect(output).toContain("${var.project}");
  });

  test("validateContent('', '') のエラー件数は 5 件", () => {
    const report = validateContent("", "");
    const output = formatReport(report);
    expect(output).toContain("5 件のエラー");
  });
});
