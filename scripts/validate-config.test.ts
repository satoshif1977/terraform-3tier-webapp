"use strict";

import {
  checkRequiredVersion,
  checkRequiredProviders,
  checkDefaultTags,
  checkNamingConvention,
  checkRequiredVariables,
  checkNoHardcodedSecrets,
  validateContent,
  formatReport,
  REQUIRED_TAGS,
  REQUIRED_VARIABLES,
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

// ── checkRequiredVersion ──────────────────────────────────────

describe("checkRequiredVersion", () => {
  test("required_version があれば PASS", () => {
    const result = checkRequiredVersion(VALID_MAIN_TF);
    expect(result.passed).toBe(true);
    expect(result.message).toContain(">= 1.5.0");
  });

  test("required_version がなければ FAIL", () => {
    const result = checkRequiredVersion('terraform { backend "s3" {} }');
    expect(result.passed).toBe(false);
    expect(result.message).toContain("required_version");
  });

  test("バージョン文字列が message に含まれる", () => {
    const result = checkRequiredVersion('required_version = "~> 1.9"');
    expect(result.message).toContain("~> 1.9");
  });

  test("FAIL 時は detail が設定される", () => {
    const result = checkRequiredVersion("");
    expect(result.detail).toBeDefined();
  });
});

// ── checkRequiredProviders ────────────────────────────────────

describe("checkRequiredProviders", () => {
  test("required_providers に aws があれば PASS", () => {
    expect(checkRequiredProviders(VALID_MAIN_TF).passed).toBe(true);
  });

  test("required_providers がなければ FAIL", () => {
    expect(checkRequiredProviders("terraform {}").passed).toBe(false);
  });

  test("aws 以外のプロバイダーのみなら FAIL", () => {
    const content = `required_providers { google = { source = "hashicorp/google" } }`;
    expect(checkRequiredProviders(content).passed).toBe(false);
  });
});

// ── checkDefaultTags ──────────────────────────────────────────

describe("checkDefaultTags", () => {
  test("必須タグがすべてあれば PASS", () => {
    expect(checkDefaultTags(VALID_MAIN_TF).passed).toBe(true);
  });

  test("default_tags がなければ FAIL", () => {
    const result = checkDefaultTags('provider "aws" { region = "ap-northeast-1" }');
    expect(result.passed).toBe(false);
    expect(result.detail).toContain("Project");
  });

  test("必須タグが欠けていれば FAIL", () => {
    const content = `
      default_tags {
        tags = {
          Project = "myapp"
        }
      }
    `;
    const result = checkDefaultTags(content);
    expect(result.passed).toBe(false);
    expect(result.message).toContain("Environment");
  });

  test("REQUIRED_TAGS は 3 件", () => {
    expect(REQUIRED_TAGS).toHaveLength(3);
  });

  test("ManagedBy が欠けていれば FAIL", () => {
    const content = `
      default_tags {
        tags = {
          Project     = "myapp"
          Environment = "dev"
        }
      }
    `;
    expect(checkDefaultTags(content).passed).toBe(false);
  });
});

// ── checkNamingConvention ─────────────────────────────────────

describe("checkNamingConvention", () => {
  test("両方の変数を使っていれば PASS", () => {
    expect(checkNamingConvention(VALID_MAIN_TF).passed).toBe(true);
  });

  test("var.project がなければ FAIL", () => {
    const content = 'name = "${var.environment}-vpc"';
    const result = checkNamingConvention(content);
    expect(result.passed).toBe(false);
    expect(result.message).toContain("var.project");
  });

  test("var.environment がなければ FAIL", () => {
    const content = 'name = "${var.project}-vpc"';
    const result = checkNamingConvention(content);
    expect(result.passed).toBe(false);
    expect(result.message).toContain("var.environment");
  });

  test("両方ない場合は FAIL で両方 message に含まれる", () => {
    const result = checkNamingConvention('resource "aws_vpc" "main" {}');
    expect(result.passed).toBe(false);
    expect(result.message).toContain("var.project");
    expect(result.message).toContain("var.environment");
  });

  test("FAIL 時は detail が設定される", () => {
    const result = checkNamingConvention("");
    expect(result.detail).toContain("var.project");
  });
});

// ── checkRequiredVariables ────────────────────────────────────

describe("checkRequiredVariables", () => {
  test("必須変数がすべてあれば PASS", () => {
    expect(checkRequiredVariables(VALID_VARIABLES_TF).passed).toBe(true);
  });

  test("変数が欠けていれば FAIL", () => {
    const content = 'variable "aws_region" {} variable "project" {}';
    const result = checkRequiredVariables(content);
    expect(result.passed).toBe(false);
    expect(result.message).toContain("environment");
  });

  test("空の variables.tf は FAIL", () => {
    expect(checkRequiredVariables("").passed).toBe(false);
  });

  test("REQUIRED_VARIABLES は 3 件", () => {
    expect(REQUIRED_VARIABLES).toHaveLength(3);
  });
});

// ── checkNoHardcodedSecrets ───────────────────────────────────

describe("checkNoHardcodedSecrets", () => {
  test("シークレットがなければ PASS", () => {
    expect(checkNoHardcodedSecrets(VALID_MAIN_TF).passed).toBe(true);
  });

  test("AKIA キーがあれば FAIL", () => {
    const content = 'access_key = "AKIAIOSFODNN7EXAMPLE"';
    const result = checkNoHardcodedSecrets(content);
    expect(result.passed).toBe(false);
    expect(result.message).toContain("1 件");
  });

  test("12桁のアカウント ID があれば FAIL", () => {
    const content = "account_id = 123456789012";
    const result = checkNoHardcodedSecrets(content);
    expect(result.passed).toBe(false);
  });

  test("var. 参照はシークレットと判定しない", () => {
    const content = 'password = "${var.db_password}"';
    expect(checkNoHardcodedSecrets(content).passed).toBe(true);
  });

  test("FAIL 時は detail に詳細が含まれる", () => {
    const content = 'access_key = "AKIAIOSFODNN7EXAMPLE"';
    const result = checkNoHardcodedSecrets(content);
    expect(result.detail).toBeDefined();
  });
});

// ── validateContent ───────────────────────────────────────────

describe("validateContent", () => {
  test("有効な main.tf + variables.tf はエラーなし", () => {
    const report = validateContent(VALID_MAIN_TF, VALID_VARIABLES_TF);
    expect(report.hasErrors).toBe(false);
  });

  test("required_version がなければ hasErrors = true", () => {
    const badMain = VALID_MAIN_TF.replace('required_version = ">= 1.5.0"', "");
    const report = validateContent(badMain, VALID_VARIABLES_TF);
    expect(report.hasErrors).toBe(true);
  });

  test("variables.tf が undefined なら required_variables FAIL", () => {
    const report = validateContent(VALID_MAIN_TF, undefined);
    const varResult = report.results.find((r) => r.name === "required_variables");
    expect(varResult?.passed).toBe(false);
  });

  test("results に 6 件含まれる", () => {
    const report = validateContent(VALID_MAIN_TF, VALID_VARIABLES_TF);
    expect(report.results).toHaveLength(6);
  });
});

// ── formatReport ─────────────────────────────────────────────

describe("formatReport", () => {
  test("全 PASS のレポートに「すべてのチェックが通過」を含む", () => {
    const report = validateContent(VALID_MAIN_TF, VALID_VARIABLES_TF);
    const output = formatReport(report);
    expect(output).toContain("すべてのチェックが通過");
  });

  test("エラーがあるレポートに「エラーがあります」を含む", () => {
    const report = validateContent("", "");
    const output = formatReport(report);
    expect(output).toContain("エラーがあります");
  });

  test("PASS は [OK] を含む", () => {
    const report = validateContent(VALID_MAIN_TF, VALID_VARIABLES_TF);
    const output = formatReport(report);
    expect(output).toContain("[OK]");
  });

  test("FAIL は [NG] を含む", () => {
    const report = validateContent("", "");
    const output = formatReport(report);
    expect(output).toContain("[NG]");
  });

  test("出力が空文字列でない", () => {
    const report = validateContent(VALID_MAIN_TF, VALID_VARIABLES_TF);
    expect(formatReport(report)).not.toBe("");
  });
});

// ── 追加テスト ─────────────────────────────────────────────────

describe("checkRequiredVersion (追加)", () => {
  test("PASS 時は detail が undefined", () => {
    const result = checkRequiredVersion(VALID_MAIN_TF);
    expect(result.detail).toBeUndefined();
  });

  test("バージョン指定が = (exact) 形式でも PASS", () => {
    const result = checkRequiredVersion('required_version = "= 1.9.3"');
    expect(result.passed).toBe(true);
    expect(result.message).toContain("= 1.9.3");
  });
});

describe("checkDefaultTags (追加)", () => {
  test("Project が欠けていれば FAIL", () => {
    const content = `
      default_tags {
        tags = {
          Environment = var.environment
          ManagedBy   = "Terraform"
        }
      }
    `;
    const result = checkDefaultTags(content);
    expect(result.passed).toBe(false);
    expect(result.message).toContain("Project");
  });
});

describe("checkNamingConvention (追加)", () => {
  test("format() 関数で var.project と var.environment を使っていれば PASS", () => {
    const content = "name = format('%s-%s-vpc', var.project, var.environment)";
    expect(checkNamingConvention(content).passed).toBe(true);
  });
});

describe("checkRequiredVariables (追加)", () => {
  test("project が欠けていれば FAIL で message に project が含まれる", () => {
    const content =
      'variable "aws_region" {} variable "environment" {}';
    const result = checkRequiredVariables(content);
    expect(result.passed).toBe(false);
    expect(result.message).toContain("project");
  });
});

describe("checkNoHardcodedSecrets (追加)", () => {
  test("password に直接値があれば FAIL", () => {
    const content = 'db_password = "SuperSecret123!"';
    expect(checkNoHardcodedSecrets(content).passed).toBe(false);
  });
});

describe("validateContent (追加)", () => {
  test("required_providers がなければ hasErrors = true", () => {
    const badMain = VALID_MAIN_TF.replace(
      /required_providers \{[\s\S]*?\}/,
      "required_providers {}"
    );
    const report = validateContent(badMain, VALID_VARIABLES_TF);
    expect(report.hasErrors).toBe(true);
  });

  test("全 PASS 時は results のすべて passed = true", () => {
    const report = validateContent(VALID_MAIN_TF, VALID_VARIABLES_TF);
    expect(report.results.every((r) => r.passed)).toBe(true);
  });
});

// ── 追加テスト（件数拡充） ─────────────────────────────────────────

describe("checkRequiredVersion (詳細)", () => {
  test("name フィールドが required_version である", () => {
    const result = checkRequiredVersion(VALID_MAIN_TF);
    expect(result.name).toBe("required_version");
  });

  test("チルダ形式 ~> 1.9 でも PASS", () => {
    const result = checkRequiredVersion('required_version = "~> 1.9"');
    expect(result.passed).toBe(true);
    expect(result.message).toContain("~> 1.9");
  });

  test("空文字列は FAIL", () => {
    expect(checkRequiredVersion("").passed).toBe(false);
  });
});

describe("checkRequiredProviders (詳細)", () => {
  test("aws と google の共存でも PASS", () => {
    const content = `
      required_providers {
        aws    = { source = "hashicorp/aws" }
        google = { source = "hashicorp/google" }
      }
    `;
    expect(checkRequiredProviders(content).passed).toBe(true);
  });

  test("name フィールドが required_providers である", () => {
    const result = checkRequiredProviders(VALID_MAIN_TF);
    expect(result.name).toBe("required_providers");
  });
});

describe("checkDefaultTags (詳細)", () => {
  test("3 タグすべて欠けている場合の FAIL メッセージに全タグが含まれる", () => {
    const content = `
      default_tags {
        tags = {
          Custom = "value"
        }
      }
    `;
    const result = checkDefaultTags(content);
    expect(result.passed).toBe(false);
    expect(result.message).toContain("Project");
    expect(result.message).toContain("Environment");
    expect(result.message).toContain("ManagedBy");
  });

  test("name フィールドが default_tags である", () => {
    const result = checkDefaultTags(VALID_MAIN_TF);
    expect(result.name).toBe("default_tags");
  });
});

describe("checkNamingConvention (詳細)", () => {
  test("補間構文 \${var.project} と \${var.environment} で PASS", () => {
    const content = 'name = "\${var.project}-\${var.environment}-sg"';
    expect(checkNamingConvention(content).passed).toBe(true);
  });

  test("name フィールドが naming_convention である", () => {
    const result = checkNamingConvention(VALID_MAIN_TF);
    expect(result.name).toBe("naming_convention");
  });
});

describe("checkRequiredVariables (詳細)", () => {
  test("aws_region が欠けていれば FAIL で message に aws_region が含まれる", () => {
    const content = 'variable "project" {} variable "environment" {}';
    const result = checkRequiredVariables(content);
    expect(result.passed).toBe(false);
    expect(result.message).toContain("aws_region");
  });

  test("3 変数すべて欠けている場合はメッセージに全変数が含まれる", () => {
    const result = checkRequiredVariables("");
    expect(result.message).toContain("aws_region");
    expect(result.message).toContain("project");
    expect(result.message).toContain("environment");
  });

  test("name フィールドが required_variables である", () => {
    const result = checkRequiredVariables(VALID_VARIABLES_TF);
    expect(result.name).toBe("required_variables");
  });
});

describe("checkNoHardcodedSecrets (詳細)", () => {
  test("複数シークレット検出時は件数が 2+ になる", () => {
    const content =
      'db_password = "SuperSecret123!" access_key = "AKIAIOSFODNN7EXAMPLE"';
    const result = checkNoHardcodedSecrets(content);
    expect(result.passed).toBe(false);
    expect(result.message).toMatch(/[2-9] 件|[1-9][0-9]+ 件/);
  });

  test("Change-me プレースホルダーは PASS", () => {
    const content = 'password = "Change-me"';
    expect(checkNoHardcodedSecrets(content).passed).toBe(true);
  });

  test("< プレースホルダーは PASS", () => {
    const content = 'secret = "<your-secret-here>"';
    expect(checkNoHardcodedSecrets(content).passed).toBe(true);
  });

  test("name フィールドが no_hardcoded_secrets である", () => {
    const result = checkNoHardcodedSecrets(VALID_MAIN_TF);
    expect(result.name).toBe("no_hardcoded_secrets");
  });
});

describe("validateContent (詳細)", () => {
  test("空文字列の main.tf は hasErrors = true", () => {
    const report = validateContent("", VALID_VARIABLES_TF);
    expect(report.hasErrors).toBe(true);
  });

  test("命名規則違反があれば results に naming_convention FAIL が含まれる", () => {
    const badMain = VALID_MAIN_TF.replace(/var\.project/g, "myapp").replace(
      /var\.environment/g,
      "dev"
    );
    const report = validateContent(badMain, VALID_VARIABLES_TF);
    const r = report.results.find((x) => x.name === "naming_convention");
    expect(r?.passed).toBe(false);
  });
});

describe("formatReport (詳細)", () => {
  test("ヘッダー行を含む", () => {
    const report = validateContent(VALID_MAIN_TF, VALID_VARIABLES_TF);
    expect(formatReport(report)).toContain("Terraform 設定検証レポート");
  });

  test("エラー件数が出力に含まれる", () => {
    const report = validateContent("", "");
    const output = formatReport(report);
    // 「X 件のエラー」の数字が含まれること
    expect(output).toMatch(/\d+ 件のエラー/);
  });

  test("check name が出力に含まれる", () => {
    const report = validateContent(VALID_MAIN_TF, VALID_VARIABLES_TF);
    const output = formatReport(report);
    expect(output).toContain("required_version");
    expect(output).toContain("required_providers");
  });
});
