#!/usr/bin/env python3
"""
Terraform 設定検証スクリプト

terraform/ ディレクトリの .tf ファイルを静的解析し、
命名規則・必須タグ・バージョン制約・シークレット混入を検出する。

使い方:
    python scripts/validate_config.py
    python scripts/validate_config.py --dir terraform/

終了コード:
    0: 検証 OK
    1: 検証エラーあり
"""

from __future__ import annotations

import argparse
import re
import sys
from dataclasses import dataclass, field
from pathlib import Path
from typing import Optional


# ── 定数 ──────────────────────────────────────────────────────────────────────

REQUIRED_VERSION_PATTERN = re.compile(r'required_version\s*=\s*"([^"]+)"')
REQUIRED_PROVIDERS_PATTERN = re.compile(
    r"required_providers\s*\{[^}]*aws\s*=", re.DOTALL
)
DEFAULT_TAGS_PATTERN = re.compile(
    r"default_tags\s*\{[^}]*tags\s*=\s*\{([^}]+)\}", re.DOTALL
)
VARIABLE_PATTERN = re.compile(r'variable\s+"(\w+)"\s*\{')

REQUIRED_VARIABLES: frozenset[str] = frozenset({"aws_region", "project", "environment"})
REQUIRED_TAGS: frozenset[str] = frozenset({"Project", "Environment", "ManagedBy"})

# ハードコードされたシークレットを検出するパターン（var.xxx 参照・プレースホルダは除外）
_SECRET_PATTERNS: list[tuple[re.Pattern[str], str]] = [
    (
        re.compile(
            r'(?i)(password|secret|token)\s*=\s*"(?!var\.|Change-me|<|\$\{)[^"]{8,}"'
        ),
        "ハードコードされた機密値",
    ),
    (re.compile(r"AKIA[0-9A-Z]{16}"), "AWS アクセスキー"),
    (re.compile(r"(?<![0-9])[0-9]{12}(?![0-9])"), "AWS アカウント ID"),
]


# ── データクラス ──────────────────────────────────────────────────────────────


@dataclass
class CheckResult:
    name: str
    passed: bool
    message: str
    detail: Optional[str] = None


@dataclass
class ValidationReport:
    results: list[CheckResult] = field(default_factory=list)

    @property
    def has_errors(self) -> bool:
        return any(not r.passed for r in self.results)

    def add(self, result: CheckResult) -> None:
        self.results.append(result)


# ── チェック関数 ──────────────────────────────────────────────────────────────


def check_required_version(content: str) -> CheckResult:
    """terraform ブロックに required_version 制約があるか確認する。"""
    match = REQUIRED_VERSION_PATTERN.search(content)
    if not match:
        return CheckResult(
            name="required_version",
            passed=False,
            message="required_version が見つかりません",
            detail='terraform ブロックに required_version = ">= 1.x" を追加してください',
        )
    return CheckResult(
        name="required_version",
        passed=True,
        message=f'required_version = "{match.group(1)}" を確認しました',
    )


def check_required_providers(content: str) -> CheckResult:
    """required_providers に aws が含まれるか確認する。"""
    if REQUIRED_PROVIDERS_PATTERN.search(content):
        return CheckResult(
            name="required_providers",
            passed=True,
            message="required_providers に aws が含まれています",
        )
    return CheckResult(
        name="required_providers",
        passed=False,
        message="required_providers に aws が見つかりません",
    )


def check_default_tags(content: str) -> CheckResult:
    """provider aws の default_tags に必須タグが含まれるか確認する。"""
    match = DEFAULT_TAGS_PATTERN.search(content)
    if not match:
        return CheckResult(
            name="default_tags",
            passed=False,
            message="default_tags が見つかりません",
            detail=f"provider aws ブロックに default_tags を追加してください (必須: {REQUIRED_TAGS})",
        )
    tags_block = match.group(1)
    missing = {tag for tag in REQUIRED_TAGS if tag not in tags_block}
    if missing:
        return CheckResult(
            name="default_tags",
            passed=False,
            message=f"必須タグが不足しています: {missing}",
        )
    return CheckResult(
        name="default_tags",
        passed=True,
        message=f"必須タグを確認しました: {REQUIRED_TAGS}",
    )


def check_required_variables(content: str) -> CheckResult:
    """variables.tf に必須変数が定義されているか確認する。"""
    defined = set(VARIABLE_PATTERN.findall(content))
    missing = REQUIRED_VARIABLES - defined
    if missing:
        return CheckResult(
            name="required_variables",
            passed=False,
            message=f"必須変数が未定義です: {missing}",
        )
    return CheckResult(
        name="required_variables",
        passed=True,
        message=f"必須変数をすべて確認しました: {REQUIRED_VARIABLES}",
    )


def check_naming_convention(content: str) -> CheckResult:
    """リソース名に var.project / var.environment を使った命名規則があるか確認する。"""
    uses_project = bool(re.search(r"\$\{var\.project\}|\bvar\.project\b", content))
    uses_env = bool(re.search(r"\$\{var\.environment\}|\bvar\.environment\b", content))
    if uses_project and uses_env:
        return CheckResult(
            name="naming_convention",
            passed=True,
            message="var.project / var.environment を使った命名規則を確認しました",
        )
    missing_parts = []
    if not uses_project:
        missing_parts.append("var.project")
    if not uses_env:
        missing_parts.append("var.environment")
    return CheckResult(
        name="naming_convention",
        passed=False,
        message=f"命名規則で使われていない変数があります: {missing_parts}",
        detail="リソース名は ${var.project}-${var.environment}-<resource> 形式にしてください",
    )


def check_no_hardcoded_secrets(tf_dir: Path) -> CheckResult:
    """全 .tf ファイルにハードコードされたシークレットがないか確認する。"""
    findings: list[str] = []
    for tf_file in sorted(tf_dir.rglob("*.tf")):
        if ".terraform" in tf_file.parts:
            continue
        text = tf_file.read_text(encoding="utf-8")
        for pattern, label in _SECRET_PATTERNS:
            for m in pattern.finditer(text):
                line_start = text.rfind("\n", 0, m.start()) + 1
                line_text = text[line_start : text.find("\n", m.start())]
                if line_text.lstrip().startswith("#"):
                    continue
                snippet = m.group()[:40].replace("\n", " ")
                findings.append(f"  {tf_file.name}: {label} ({snippet})")

    if findings:
        detail = "\n".join(findings)
        return CheckResult(
            name="no_hardcoded_secrets",
            passed=False,
            message=f"機密値の疑いがある箇所を {len(findings)} 件検出しました",
            detail=detail,
        )
    return CheckResult(
        name="no_hardcoded_secrets",
        passed=True,
        message="ハードコードされた機密値は見つかりませんでした",
    )


# ── 検証エントリポイント ──────────────────────────────────────────────────────


def validate(tf_dir: Path) -> ValidationReport:
    """tf_dir 以下の Terraform 設定を検証して ValidationReport を返す。"""
    report = ValidationReport()

    main_tf = tf_dir / "main.tf"
    variables_tf = tf_dir / "variables.tf"

    if not main_tf.exists():
        report.add(CheckResult("main.tf", False, f"{main_tf} が見つかりません"))
        return report

    main_content = main_tf.read_text(encoding="utf-8")
    report.add(check_required_version(main_content))
    report.add(check_required_providers(main_content))
    report.add(check_default_tags(main_content))
    report.add(check_naming_convention(main_content))

    if variables_tf.exists():
        variables_content = variables_tf.read_text(encoding="utf-8")
        report.add(check_required_variables(variables_content))
    else:
        report.add(
            CheckResult("required_variables", False, f"{variables_tf} が見つかりません")
        )

    report.add(check_no_hardcoded_secrets(tf_dir))

    return report


def print_report(report: ValidationReport) -> None:
    """検証結果をコンソールに出力する。"""
    print("\n=== Terraform 設定検証レポート ===\n")
    for r in report.results:
        icon = "OK" if r.passed else "NG"
        status = "PASS" if r.passed else "FAIL"
        print(f"  [{icon}] {r.name:<28} {status}  {r.message}")
        if r.detail:
            for line in r.detail.splitlines():
                print(f"         {line}")
    print()
    error_count = sum(1 for r in report.results if not r.passed)
    if report.has_errors:
        print(f"結果: {error_count} 件のエラーがあります")
    else:
        print(f"結果: すべてのチェックが通過しました ({len(report.results)} 件)")
    print()


def main() -> int:
    parser = argparse.ArgumentParser(
        description="Terraform 設定検証スクリプト",
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog="例:\n  python scripts/validate_config.py\n  python scripts/validate_config.py --dir terraform/",
    )
    parser.add_argument(
        "--dir",
        default="terraform",
        help="Terraform ファイルのディレクトリ (デフォルト: terraform)",
    )
    args = parser.parse_args()

    tf_dir = Path(args.dir)
    if not tf_dir.exists():
        print(f"エラー: ディレクトリが見つかりません: {tf_dir}", file=sys.stderr)
        return 1

    report = validate(tf_dir)
    print_report(report)
    return 1 if report.has_errors else 0


if __name__ == "__main__":
    sys.exit(main())
