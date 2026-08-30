/**
 * terraform-3tier-webapp: 共通ヘルパー関数・型定義・定数
 *
 * validate-config.ts から抽出した型定義・定数・個別チェック関数群。
 * 各チェック関数は純粋関数で、ファイルシステムに依存しない。
 */

// ── 型定義 ────────────────────────────────────────────────────

export interface CheckResult {
  name: string;
  passed: boolean;
  message: string;
  detail?: string;
}

export interface ValidationReport {
  results: CheckResult[];
  hasErrors: boolean;
}

// ── 定数 ─────────────────────────────────────────────────────

export const REQUIRED_TAGS = ["Project", "Environment", "ManagedBy"] as const;
export const REQUIRED_VARIABLES = ["aws_region", "project", "environment"] as const;

// ── チェック関数（純粋関数） ──────────────────────────────────

/**
 * terraform ブロックに required_version 制約があるか確認する
 * Python: check_required_version()
 */
export function checkRequiredVersion(content: string): CheckResult {
  const match = content.match(/required_version\s*=\s*"([^"]+)"/);
  if (!match) {
    return {
      name: "required_version",
      passed: false,
      message: "required_version が見つかりません",
      detail: 'terraform ブロックに required_version = ">= 1.x" を追加してください',
    };
  }
  return {
    name: "required_version",
    passed: true,
    message: `required_version = "${match[1]}" を確認しました`,
  };
}

/**
 * required_providers に aws が含まれるか確認する
 * Python: check_required_providers()
 */
export function checkRequiredProviders(content: string): CheckResult {
  const pattern = /required_providers\s*\{[^}]*aws\s*=/s;
  if (pattern.test(content)) {
    return {
      name: "required_providers",
      passed: true,
      message: "required_providers に aws が含まれています",
    };
  }
  return {
    name: "required_providers",
    passed: false,
    message: "required_providers に aws が見つかりません",
  };
}

/**
 * provider aws の default_tags に必須タグが含まれるか確認する
 * Python: check_default_tags()
 */
export function checkDefaultTags(content: string): CheckResult {
  const match = content.match(/default_tags\s*\{[^}]*tags\s*=\s*\{([^}]+)\}/s);
  if (!match) {
    return {
      name: "default_tags",
      passed: false,
      message: "default_tags が見つかりません",
      detail: `provider aws ブロックに default_tags を追加してください (必須: ${REQUIRED_TAGS.join(", ")})`,
    };
  }
  const tagsBlock = match[1];
  const missing = REQUIRED_TAGS.filter((tag) => !tagsBlock.includes(tag));
  if (missing.length > 0) {
    return {
      name: "default_tags",
      passed: false,
      message: `必須タグが不足しています: ${missing.join(", ")}`,
    };
  }
  return {
    name: "default_tags",
    passed: true,
    message: `必須タグを確認しました: ${REQUIRED_TAGS.join(", ")}`,
  };
}

/**
 * var.project / var.environment を使った命名規則か確認する
 * Python: check_naming_convention()
 */
export function checkNamingConvention(content: string): CheckResult {
  const usesProject = /\$\{var\.project\}|\bvar\.project\b/.test(content);
  const usesEnv = /\$\{var\.environment\}|\bvar\.environment\b/.test(content);
  if (usesProject && usesEnv) {
    return {
      name: "naming_convention",
      passed: true,
      message: "var.project / var.environment を使った命名規則を確認しました",
    };
  }
  const missing: string[] = [];
  if (!usesProject) missing.push("var.project");
  if (!usesEnv) missing.push("var.environment");
  return {
    name: "naming_convention",
    passed: false,
    message: `命名規則で使われていない変数があります: ${missing.join(", ")}`,
    detail: "リソース名は ${var.project}-${var.environment}-<resource> 形式にしてください",
  };
}

/**
 * variables.tf に必須変数が定義されているか確認する
 * Python: check_required_variables()
 */
export function checkRequiredVariables(content: string): CheckResult {
  const defined = new Set<string>();
  const pattern = /variable\s+"(\w+)"\s*\{/g;
  let m: RegExpExecArray | null;
  while ((m = pattern.exec(content)) !== null) {
    defined.add(m[1]);
  }
  const missing = REQUIRED_VARIABLES.filter((v) => !defined.has(v));
  if (missing.length > 0) {
    return {
      name: "required_variables",
      passed: false,
      message: `必須変数が未定義です: ${missing.join(", ")}`,
    };
  }
  return {
    name: "required_variables",
    passed: true,
    message: `必須変数をすべて確認しました: ${REQUIRED_VARIABLES.join(", ")}`,
  };
}

/**
 * ハードコードされたシークレットがないか確認する
 * Python: check_no_hardcoded_secrets()
 */
export function checkNoHardcodedSecrets(content: string): CheckResult {
  const findings: string[] = [];

  const secretPattern =
    /(password|secret|token)\s*=\s*"(?!var\.|Change-me|<|\$\{)[^"]{8,}"/gi;
  const akiaPattern = /AKIA[0-9A-Z]{16}/g;
  const accountIdPattern = /(?<![0-9])[0-9]{12}(?![0-9])/g;

  for (const m of content.matchAll(secretPattern)) {
    findings.push(`ハードコードされた機密値: ${m[0].slice(0, 40)}`);
  }
  for (const m of content.matchAll(akiaPattern)) {
    findings.push(`AWS アクセスキー: ${m[0]}`);
  }
  for (const m of content.matchAll(accountIdPattern)) {
    findings.push(`AWS アカウント ID の疑い: ${m[0]}`);
  }

  if (findings.length > 0) {
    return {
      name: "no_hardcoded_secrets",
      passed: false,
      message: `機密値の疑いがある箇所を ${findings.length} 件検出しました`,
      detail: findings.join("\n"),
    };
  }
  return {
    name: "no_hardcoded_secrets",
    passed: true,
    message: "ハードコードされた機密値は見つかりませんでした",
  };
}
