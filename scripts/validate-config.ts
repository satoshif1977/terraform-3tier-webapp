/**
 * Terraform 設定検証スクリプト（TypeScript 版）
 *
 * Python 版 validate_config.py と同じチェックを TypeScript で実装。
 * 個別チェック関数は helpers.ts に分離し、本ファイルは
 * オーケストレーターとレポートフォーマッターに集中。
 *
 * チェック内容:
 *   - required_version 制約の有無
 *   - required_providers に aws が含まれるか
 *   - default_tags に必須タグ（Project / Environment / ManagedBy）があるか
 *   - var.project / var.environment を使った命名規則か
 *   - 必須変数（aws_region / project / environment）の定義有無
 *   - ハードコードされたシークレット（AKIA キー / パスワード）の検出
 */

import type { CheckResult, ValidationReport } from "./helpers";
import {
  checkRequiredVersion,
  checkRequiredProviders,
  checkDefaultTags,
  checkNamingConvention,
  checkRequiredVariables,
  checkNoHardcodedSecrets,
} from "./helpers";

// ── re-export（テスト互換） ─────────────────────────────────
export type { CheckResult, ValidationReport };
export {
  REQUIRED_TAGS,
  REQUIRED_VARIABLES,
  checkRequiredVersion,
  checkRequiredProviders,
  checkDefaultTags,
  checkNamingConvention,
  checkRequiredVariables,
  checkNoHardcodedSecrets,
} from "./helpers";

// ── 検証オーケストレーター ────────────────────────────────────

/**
 * main.tf と variables.tf の内容を受け取り ValidationReport を返す
 * Python: validate()
 */
export function validateContent(
  mainTf: string,
  variablesTf?: string
): ValidationReport {
  const results: CheckResult[] = [
    checkRequiredVersion(mainTf),
    checkRequiredProviders(mainTf),
    checkDefaultTags(mainTf),
    checkNamingConvention(mainTf),
    checkNoHardcodedSecrets(mainTf),
  ];

  if (variablesTf !== undefined) {
    results.push(checkRequiredVariables(variablesTf));
  } else {
    results.push({
      name: "required_variables",
      passed: false,
      message: "variables.tf が見つかりません",
    });
  }

  return {
    results,
    hasErrors: results.some((r) => !r.passed),
  };
}

/**
 * ValidationReport を人間が読みやすい文字列にフォーマットする
 * Python: print_report()
 */
export function formatReport(report: ValidationReport): string {
  const lines: string[] = ["=== Terraform 設定検証レポート ===", ""];
  for (const r of report.results) {
    const icon = r.passed ? "OK" : "NG";
    const status = r.passed ? "PASS" : "FAIL";
    lines.push(`  [${icon}] ${r.name.padEnd(28)} ${status}  ${r.message}`);
    if (r.detail) {
      for (const line of r.detail.split("\n")) {
        lines.push(`         ${line}`);
      }
    }
  }
  lines.push("");
  const errorCount = report.results.filter((r) => !r.passed).length;
  if (report.hasErrors) {
    lines.push(`結果: ${errorCount} 件のエラーがあります`);
  } else {
    lines.push(`結果: すべてのチェックが通過しました (${report.results.length} 件)`);
  }
  return lines.join("\n");
}
