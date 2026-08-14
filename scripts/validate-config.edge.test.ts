"use strict";

/**
 * Terraform 設定検証スクリプト エッジケーステスト
 *
 * 境界値・パターン詳細・定数値・型・複数検出ケースを追加検証する。
 */

import {
  REQUIRED_TAGS,
  REQUIRED_VARIABLES,
  checkDefaultTags,
  checkNamingConvention,
  checkNoHardcodedSecrets,
  checkRequiredProviders,
  checkRequiredVariables,
  checkRequiredVersion,
  formatReport,
  validateContent,
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

// ── 定数値の検証 ──────────────────────────────────────────────

describe("REQUIRED_TAGS 定数", () => {
  test("Project が含まれる", () => {
    expect(REQUIRED_TAGS).toContain("Project");
  });

  test("Environment が含まれる", () => {
    expect(REQUIRED_TAGS).toContain("Environment");
  });

  test("ManagedBy が含まれる", () => {
    expect(REQUIRED_TAGS).toContain("ManagedBy");
  });

  test("要素数が 3 である", () => {
    expect(REQUIRED_TAGS.length).toBe(3);
  });
});

describe("REQUIRED_VARIABLES 定数", () => {
  test("aws_region が含まれる", () => {
    expect(REQUIRED_VARIABLES).toContain("aws_region");
  });

  test("project が含まれる", () => {
    expect(REQUIRED_VARIABLES).toContain("project");
  });

  test("environment が含まれる", () => {
    expect(REQUIRED_VARIABLES).toContain("environment");
  });

  test("要素数が 3 である", () => {
    expect(REQUIRED_VARIABLES.length).toBe(3);
  });
});

// ── checkRequiredVersion エッジケース ─────────────────────────

describe("checkRequiredVersion (エッジ)", () => {
  test("チルダ制約 ~> 1.5 形式でも PASS", () => {
    const result = checkRequiredVersion('required_version = "~> 1.5"');
    expect(result.passed).toBe(true);
  });

  test("固定バージョン 1.5.0 形式でも PASS", () => {
    const result = checkRequiredVersion('required_version = "1.5.0"');
    expect(result.passed).toBe(true);
  });

  test("メッセージにバージョン文字列が含まれる", () => {
    const result = checkRequiredVersion('required_version = "~> 1.5"');
    expect(result.message).toContain("~> 1.5");
  });

  test("FAIL 時の name は required_version", () => {
    const result = checkRequiredVersion("terraform {}");
    expect(result.name).toBe("required_version");
  });

  test("FAIL 時に detail が存在する", () => {
    const result = checkRequiredVersion("terraform {}");
    expect(result.detail).toBeDefined();
  });

  test("FAIL 時の detail に >= 1.x が含まれる", () => {
    const result = checkRequiredVersion("terraform {}");
    expect(result.detail).toContain(">= 1.x");
  });

  test("複数行テキストに埋め込まれていても PASS", () => {
    const content = `
terraform {
  required_version = ">= 1.5.0"
  backend "s3" {}
}
`;
    expect(checkRequiredVersion(content).passed).toBe(true);
  });
});

// ── checkRequiredProviders エッジケース ───────────────────────

describe("checkRequiredProviders (エッジ)", () => {
  test("name は required_providers", () => {
    expect(checkRequiredProviders(VALID_MAIN_TF).name).toBe("required_providers");
  });

  test("azure のみの場合 FAIL", () => {
    const content = `
required_providers {
  azurerm = { source = "hashicorp/azurerm" }
}
`;
    expect(checkRequiredProviders(content).passed).toBe(false);
  });

  test("aws と azurerm 両方ある場合 PASS", () => {
    const content = `
required_providers {
  aws     = { source = "hashicorp/aws" }
  azurerm = { source = "hashicorp/azurerm" }
}
`;
    expect(checkRequiredProviders(content).passed).toBe(true);
  });
});

// ── checkDefaultTags エッジケース ─────────────────────────────

describe("checkDefaultTags (エッジ)", () => {
  test("name は default_tags", () => {
    expect(checkDefaultTags(VALID_MAIN_TF).name).toBe("default_tags");
  });

  test("Project と Environment が両方欠けた場合 message に両方が含まれる", () => {
    const content = `
default_tags {
  tags = {
    ManagedBy = "Terraform"
  }
}
`;
    const result = checkDefaultTags(content);
    expect(result.passed).toBe(false);
    expect(result.message).toContain("Project");
    expect(result.message).toContain("Environment");
  });

  test("ManagedBy のみ欠けた場合 message に ManagedBy が含まれる", () => {
    const content = `
default_tags {
  tags = {
    Project     = "myapp"
    Environment = "dev"
  }
}
`;
    const result = checkDefaultTags(content);
    expect(result.passed).toBe(false);
    expect(result.message).toContain("ManagedBy");
  });

  test("FAIL 時（タグ不足）detail は undefined", () => {
    const content = `
default_tags {
  tags = {
    Project = "myapp"
  }
}
`;
    const result = checkDefaultTags(content);
    // タグ不足の FAIL には detail がない
    expect(result.detail).toBeUndefined();
  });

  test("default_tags ブロック自体が存在しない場合の FAIL detail に必須タグ名が含まれる", () => {
    const result = checkDefaultTags("provider aws {}");
    expect(result.detail).toContain("Project");
    expect(result.detail).toContain("Environment");
    expect(result.detail).toContain("ManagedBy");
  });
});

// ── checkNamingConvention エッジケース ────────────────────────

describe("checkNamingConvention (エッジ)", () => {
  test("name は naming_convention", () => {
    expect(checkNamingConvention(VALID_MAIN_TF).name).toBe("naming_convention");
  });

  test("非テンプレート形式 var.project でも PASS", () => {
    const content = "name = var.project\nenvironment = var.environment";
    expect(checkNamingConvention(content).passed).toBe(true);
  });

  test("var.project のみ使用で FAIL message に var.environment が含まれる", () => {
    const result = checkNamingConvention('name = "${var.project}-vpc"');
    expect(result.passed).toBe(false);
    expect(result.message).toContain("var.environment");
    expect(result.message).not.toContain("var.project");
  });

  test("FAIL 時の detail に ${var.environment} が含まれる", () => {
    const result = checkNamingConvention("");
    expect(result.detail).toContain("${var.environment}");
  });

  test("テンプレート形式 ${var.project} と ${var.environment} 両方で PASS", () => {
    const content = 'name = "${var.project}-${var.environment}-vpc"';
    expect(checkNamingConvention(content).passed).toBe(true);
  });
});

// ── checkRequiredVariables エッジケース ───────────────────────

describe("checkRequiredVariables (エッジ)", () => {
  test("name は required_variables", () => {
    expect(checkRequiredVariables(VALID_VARIABLES_TF).name).toBe("required_variables");
  });

  test("空文字列のとき 3 変数すべてが message に含まれる", () => {
    const result = checkRequiredVariables("");
    expect(result.passed).toBe(false);
    expect(result.message).toContain("aws_region");
    expect(result.message).toContain("project");
    expect(result.message).toContain("environment");
  });

  test("aws_region のみ欠けた場合 message に aws_region が含まれる", () => {
    const content = `
variable "project" {}
variable "environment" {}
`;
    const result = checkRequiredVariables(content);
    expect(result.passed).toBe(false);
    expect(result.message).toContain("aws_region");
  });

  test("PASS message に environment が含まれる", () => {
    const result = checkRequiredVariables(VALID_VARIABLES_TF);
    expect(result.message).toContain("environment");
  });

  test("シングルライン形式の変数定義でも認識する", () => {
    const content =
      'variable "aws_region" {} variable "project" {} variable "environment" {}';
    expect(checkRequiredVariables(content).passed).toBe(true);
  });
});

// ── checkNoHardcodedSecrets エッジケース ─────────────────────

describe("checkNoHardcodedSecrets (エッジ)", () => {
  test("AKIA + 正確に 16 文字英数字で FAIL", () => {
    const result = checkNoHardcodedSecrets("AKIAIOSFODNN7EXAMPLE");
    expect(result.passed).toBe(false);
  });

  test("AKIA + 15 文字は検出対象外で PASS", () => {
    // AKIA[0-9A-Z]{16} → 15文字は不一致
    const result = checkNoHardcodedSecrets("AKIAIOSFODNN7EXAM");
    expect(result.passed).toBe(true);
  });

  test("複数 AKIA キーは findings が 2 件以上", () => {
    // AKIA + 16 文字英数字 の正規パターン 2 件
    const content = "AKIAIOSFODNN7EXAMPLE\nAKIAJ3HO7ZY7MRDKE4KN";
    const result = checkNoHardcodedSecrets(content);
    expect(result.passed).toBe(false);
    // message に「2 件」が含まれること
    expect(result.message).toContain("2");
  });

  test("12 桁アカウント ID が検出される", () => {
    const result = checkNoHardcodedSecrets("account_id = 123456789012");
    expect(result.passed).toBe(false);
    expect(result.detail).toContain("123456789012");
  });

  test("13 桁の数字はアカウント ID 対象外で PASS", () => {
    // (?<![0-9])[0-9]{12}(?![0-9]) なので 13桁は不一致
    const result = checkNoHardcodedSecrets("1234567890123");
    expect(result.passed).toBe(true);
  });

  test("password = \"var.secret\" は PASS", () => {
    const result = checkNoHardcodedSecrets('password = "var.secret_value"');
    expect(result.passed).toBe(true);
  });

  test("password = \"Change-me-abc\" は PASS", () => {
    const result = checkNoHardcodedSecrets('password = "Change-me-abc"');
    expect(result.passed).toBe(true);
  });

  test("password = \"<secret_here>\" は PASS", () => {
    const result = checkNoHardcodedSecrets('password = "<secret_here>"');
    expect(result.passed).toBe(true);
  });

  test("secret = \"${var.secret}\" は PASS", () => {
    const result = checkNoHardcodedSecrets('secret = "${var.secret}"');
    expect(result.passed).toBe(true);
  });

  test("name は no_hardcoded_secrets", () => {
    expect(checkNoHardcodedSecrets("clean content").name).toBe("no_hardcoded_secrets");
  });

  test("FAIL 時の detail に検出テキストが含まれる", () => {
    const result = checkNoHardcodedSecrets("AKIAIOSFODNN7EXAMPLE");
    expect(result.detail).toContain("AKIA");
  });
});

// ── validateContent エッジケース ──────────────────────────────

describe("validateContent (エッジ)", () => {
  test("variablesTf が undefined のとき required_variables は FAIL", () => {
    const report = validateContent(VALID_MAIN_TF, undefined);
    const varResult = report.results.find((r) => r.name === "required_variables");
    expect(varResult?.passed).toBe(false);
  });

  test("results の長さは常に 6", () => {
    expect(validateContent(VALID_MAIN_TF, VALID_VARIABLES_TF).results.length).toBe(6);
    expect(validateContent("", undefined).results.length).toBe(6);
  });

  test("hasErrors は boolean 型", () => {
    const report = validateContent(VALID_MAIN_TF, VALID_VARIABLES_TF);
    expect(typeof report.hasErrors).toBe("boolean");
  });

  test("全 PASS のとき hasErrors は false", () => {
    expect(validateContent(VALID_MAIN_TF, VALID_VARIABLES_TF).hasErrors).toBe(false);
  });

  test("results[5].name は required_variables", () => {
    const report = validateContent(VALID_MAIN_TF, VALID_VARIABLES_TF);
    expect(report.results[5].name).toBe("required_variables");
  });

  test("variablesTf が undefined のとき message に variables.tf が含まれる", () => {
    const report = validateContent(VALID_MAIN_TF, undefined);
    const varResult = report.results.find((r) => r.name === "required_variables");
    expect(varResult?.message).toContain("variables.tf");
  });

  test("results の各要素の passed は boolean 型", () => {
    const report = validateContent(VALID_MAIN_TF, VALID_VARIABLES_TF);
    for (const r of report.results) {
      expect(typeof r.passed).toBe("boolean");
    }
  });
});

// ── formatReport エッジケース ─────────────────────────────────

describe("formatReport (エッジ)", () => {
  test("ヘッダー行に '=== Terraform 設定検証レポート ===' が含まれる", () => {
    const report = validateContent(VALID_MAIN_TF, VALID_VARIABLES_TF);
    expect(formatReport(report)).toContain("=== Terraform 設定検証レポート ===");
  });

  test("全 PASS のとき 'OK' アイコンが含まれる", () => {
    const report = validateContent(VALID_MAIN_TF, VALID_VARIABLES_TF);
    expect(formatReport(report)).toContain("[OK]");
  });

  test("FAIL がある場合 'NG' アイコンが含まれる", () => {
    const report = validateContent("", "");
    expect(formatReport(report)).toContain("[NG]");
  });

  test("全 PASS のとき 'PASS' 文字列が含まれる", () => {
    const report = validateContent(VALID_MAIN_TF, VALID_VARIABLES_TF);
    expect(formatReport(report)).toContain("PASS");
  });

  test("FAIL がある場合 'FAIL' 文字列が含まれる", () => {
    const report = validateContent("", "");
    expect(formatReport(report)).toContain("FAIL");
  });

  test("全 PASS のとき 'すべてのチェックが通過しました' が含まれる", () => {
    const report = validateContent(VALID_MAIN_TF, VALID_VARIABLES_TF);
    expect(formatReport(report)).toContain("すべてのチェックが通過しました");
  });

  test("FAIL がある場合 'エラー' が含まれる", () => {
    const report = validateContent("", "");
    expect(formatReport(report)).toContain("エラー");
  });

  test("戻り値は string 型", () => {
    const report = validateContent(VALID_MAIN_TF, VALID_VARIABLES_TF);
    expect(typeof formatReport(report)).toBe("string");
  });

  test("detail が複数行のとき改行数分のインデント行が出力される", () => {
    const secretMain = VALID_MAIN_TF + "\nAKIAIOSFODNN7EXAMPLE\nAKIAJ3HO7ZY7MRDKE4KN";
    const report = validateContent(secretMain, VALID_VARIABLES_TF);
    const output = formatReport(report);
    // detail が 2 件 → インデント行が 2 行以上存在する
    const indentLines = output.split("\n").filter((l) => l.startsWith("         "));
    expect(indentLines.length).toBeGreaterThanOrEqual(2);
  });
});
