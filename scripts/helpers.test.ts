"use strict";

/**
 * terraform-3tier-webapp helpers.ts 直接インポートテスト
 *
 * helpers.ts に分離した型定義・定数・個別チェック関数を直接テストする。
 */

import {
  REQUIRED_TAGS,
  REQUIRED_VARIABLES,
  checkRequiredVersion,
  checkRequiredProviders,
  checkDefaultTags,
  checkNamingConvention,
  checkRequiredVariables,
  checkNoHardcodedSecrets,
} from "./helpers";
import type { CheckResult } from "./helpers";

// ── 定数テスト ────────────────────────────────────────────────

describe("REQUIRED_TAGS (from helpers)", () => {
  test("3 件のタグが定義されている", () => {
    expect(REQUIRED_TAGS).toHaveLength(3);
  });

  test("Project / Environment / ManagedBy を含む", () => {
    expect(REQUIRED_TAGS).toContain("Project");
    expect(REQUIRED_TAGS).toContain("Environment");
    expect(REQUIRED_TAGS).toContain("ManagedBy");
  });
});

describe("REQUIRED_VARIABLES (from helpers)", () => {
  test("3 件の変数が定義されている", () => {
    expect(REQUIRED_VARIABLES).toHaveLength(3);
  });

  test("aws_region / project / environment を含む", () => {
    expect(REQUIRED_VARIABLES).toContain("aws_region");
    expect(REQUIRED_VARIABLES).toContain("project");
    expect(REQUIRED_VARIABLES).toContain("environment");
  });
});

// ── CheckResult 型の構造確認 ──────────────────────────────────

describe("CheckResult structure (from helpers)", () => {
  test("PASS 結果は name / passed / message を持つ", () => {
    const result: CheckResult = checkRequiredVersion('required_version = ">= 1.5.0"');
    expect(result).toHaveProperty("name");
    expect(result).toHaveProperty("passed");
    expect(result).toHaveProperty("message");
    expect(result.passed).toBe(true);
  });

  test("FAIL 結果は detail を持つことがある", () => {
    const result: CheckResult = checkRequiredVersion("");
    expect(result.passed).toBe(false);
    expect(result.detail).toBeDefined();
  });
});

// ── checkRequiredVersion（helpers 直接） ─────────────────────

describe("checkRequiredVersion (from helpers)", () => {
  test("required_version があれば PASS", () => {
    expect(checkRequiredVersion('required_version = ">= 1.5.0"').passed).toBe(true);
  });

  test("なければ FAIL", () => {
    expect(checkRequiredVersion("terraform {}").passed).toBe(false);
  });

  test("バージョン文字列が message に含まれる", () => {
    expect(checkRequiredVersion('required_version = "~> 1.9"').message).toContain("~> 1.9");
  });

  test("name は required_version", () => {
    expect(checkRequiredVersion("").name).toBe("required_version");
  });
});

// ── checkRequiredProviders（helpers 直接） ────────────────────

describe("checkRequiredProviders (from helpers)", () => {
  test("aws があれば PASS", () => {
    const content = 'required_providers { aws = { source = "hashicorp/aws" } }';
    expect(checkRequiredProviders(content).passed).toBe(true);
  });

  test("aws がなければ FAIL", () => {
    expect(checkRequiredProviders("terraform {}").passed).toBe(false);
  });

  test("name は required_providers", () => {
    expect(checkRequiredProviders("").name).toBe("required_providers");
  });
});

// ── checkDefaultTags（helpers 直接） ─────────────────────────

describe("checkDefaultTags (from helpers)", () => {
  test("必須タグ全てあれば PASS", () => {
    const content = `default_tags { tags = { Project = "x" Environment = "y" ManagedBy = "z" } }`;
    expect(checkDefaultTags(content).passed).toBe(true);
  });

  test("タグ不足なら FAIL", () => {
    const content = `default_tags { tags = { Project = "x" } }`;
    expect(checkDefaultTags(content).passed).toBe(false);
  });

  test("default_tags がなければ FAIL", () => {
    expect(checkDefaultTags("provider aws {}").passed).toBe(false);
  });

  test("name は default_tags", () => {
    expect(checkDefaultTags("").name).toBe("default_tags");
  });
});

// ── checkNamingConvention（helpers 直接） ─────────────────────

describe("checkNamingConvention (from helpers)", () => {
  test("両方の変数を使えば PASS", () => {
    const content = 'name = "${var.project}-${var.environment}-vpc"';
    expect(checkNamingConvention(content).passed).toBe(true);
  });

  test("var.project がなければ FAIL", () => {
    const result = checkNamingConvention("var.environment");
    expect(result.passed).toBe(false);
    expect(result.message).toContain("var.project");
  });

  test("両方なければ FAIL で両方が message に含まれる", () => {
    const result = checkNamingConvention("");
    expect(result.message).toContain("var.project");
    expect(result.message).toContain("var.environment");
  });

  test("name は naming_convention", () => {
    expect(checkNamingConvention("").name).toBe("naming_convention");
  });
});

// ── checkRequiredVariables（helpers 直接） ────────────────────

describe("checkRequiredVariables (from helpers)", () => {
  test("必須変数全てあれば PASS", () => {
    const content = 'variable "aws_region" {} variable "project" {} variable "environment" {}';
    expect(checkRequiredVariables(content).passed).toBe(true);
  });

  test("欠けていれば FAIL", () => {
    expect(checkRequiredVariables('variable "aws_region" {}').passed).toBe(false);
  });

  test("空文字列は FAIL", () => {
    expect(checkRequiredVariables("").passed).toBe(false);
  });

  test("name は required_variables", () => {
    expect(checkRequiredVariables("").name).toBe("required_variables");
  });
});

// ── checkNoHardcodedSecrets（helpers 直接） ───────────────────

describe("checkNoHardcodedSecrets (from helpers)", () => {
  test("シークレットがなければ PASS", () => {
    expect(checkNoHardcodedSecrets("resource {}").passed).toBe(true);
  });

  test("AKIA キーがあれば FAIL", () => {
    expect(checkNoHardcodedSecrets('key = "AKIAIOSFODNN7EXAMPLE"').passed).toBe(false);
  });

  test("12桁アカウント ID があれば FAIL", () => {
    expect(checkNoHardcodedSecrets("account = 123456789012").passed).toBe(false);
  });

  test("var. 参照はシークレットと判定しない", () => {
    expect(checkNoHardcodedSecrets('password = "${var.db_password}"').passed).toBe(true);
  });

  test("Change-me プレースホルダーは PASS", () => {
    expect(checkNoHardcodedSecrets('password = "Change-me"').passed).toBe(true);
  });

  test("name は no_hardcoded_secrets", () => {
    expect(checkNoHardcodedSecrets("").name).toBe("no_hardcoded_secrets");
  });
});
