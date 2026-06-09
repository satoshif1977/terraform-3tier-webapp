"""
validate_config.py のユニットテスト
実行: pytest scripts/test_validate_config.py -v
"""

from __future__ import annotations

import sys
import textwrap
from pathlib import Path

# scripts/ を import パスに追加
sys.path.insert(0, str(Path(__file__).parent))
from validate_config import (
    CheckResult,
    ValidationReport,
    check_default_tags,
    check_naming_convention,
    check_no_hardcoded_secrets,
    check_required_providers,
    check_required_variables,
    check_required_version,
    validate,
)


# ── フィクスチャ ──────────────────────────────────────────────────────────────


VALID_MAIN_TF = textwrap.dedent(
    """\
    terraform {
      required_version = ">= 1.5"
      required_providers {
        aws = {
          source  = "hashicorp/aws"
          version = "~> 6.0"
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
      cidr_block = "10.0.0.0/16"
      tags = {
        Name = "${var.project}-${var.environment}-vpc"
      }
    }
    """
)

VALID_VARIABLES_TF = textwrap.dedent(
    """\
    variable "aws_region" {
      description = "AWS リージョン"
      type        = string
      default     = "ap-northeast-1"
    }

    variable "project" {
      description = "プロジェクト名"
      type        = string
    }

    variable "environment" {
      description = "環境名"
      type        = string
    }
    """
)


# ── check_required_version ────────────────────────────────────────────────────


class TestCheckRequiredVersion:
    def test_pass_when_version_exists(self) -> None:
        result = check_required_version(VALID_MAIN_TF)
        assert result.passed is True
        assert ">= 1.5" in result.message

    def test_fail_when_version_missing(self) -> None:
        content = 'terraform {\n  required_providers {}\n}'
        result = check_required_version(content)
        assert result.passed is False
        assert "required_version" in result.message

    def test_captures_version_string(self) -> None:
        content = 'terraform {\n  required_version = "~> 1.9"\n}'
        result = check_required_version(content)
        assert result.passed is True
        assert "~> 1.9" in result.message


# ── check_required_providers ─────────────────────────────────────────────────


class TestCheckRequiredProviders:
    def test_pass_when_aws_provider_exists(self) -> None:
        result = check_required_providers(VALID_MAIN_TF)
        assert result.passed is True

    def test_fail_when_aws_provider_missing(self) -> None:
        content = "terraform {\n  required_providers {\n    random = {}\n  }\n}"
        result = check_required_providers(content)
        assert result.passed is False

    def test_fail_when_no_required_providers_block(self) -> None:
        result = check_required_providers("terraform {}")
        assert result.passed is False


# ── check_default_tags ────────────────────────────────────────────────────────


class TestCheckDefaultTags:
    def test_pass_when_all_required_tags_exist(self) -> None:
        result = check_default_tags(VALID_MAIN_TF)
        assert result.passed is True

    def test_fail_when_default_tags_missing(self) -> None:
        content = 'provider "aws" {\n  region = "ap-northeast-1"\n}'
        result = check_default_tags(content)
        assert result.passed is False
        assert "default_tags" in result.message

    def test_fail_when_required_tag_missing(self) -> None:
        content = textwrap.dedent(
            """\
            provider "aws" {
              default_tags {
                tags = {
                  Project   = var.project
                  ManagedBy = "Terraform"
                }
              }
            }
            """
        )
        result = check_default_tags(content)
        assert result.passed is False
        assert "Environment" in result.message


# ── check_required_variables ──────────────────────────────────────────────────


class TestCheckRequiredVariables:
    def test_pass_when_all_required_variables_defined(self) -> None:
        result = check_required_variables(VALID_VARIABLES_TF)
        assert result.passed is True

    def test_fail_when_variable_missing(self) -> None:
        content = 'variable "aws_region" {}\nvariable "project" {}'
        result = check_required_variables(content)
        assert result.passed is False
        assert "environment" in result.message

    def test_fail_when_no_variables(self) -> None:
        result = check_required_variables("")
        assert result.passed is False


# ── check_naming_convention ───────────────────────────────────────────────────


class TestCheckNamingConvention:
    def test_pass_when_both_vars_used(self) -> None:
        result = check_naming_convention(VALID_MAIN_TF)
        assert result.passed is True

    def test_fail_when_only_project_used(self) -> None:
        content = 'name = "${var.project}-vpc"'
        result = check_naming_convention(content)
        assert result.passed is False
        assert "var.environment" in result.message

    def test_fail_when_neither_used(self) -> None:
        content = 'name = "my-hardcoded-vpc"'
        result = check_naming_convention(content)
        assert result.passed is False

    def test_pass_with_non_interpolation_syntax(self) -> None:
        content = "name = format('%s-%s-vpc', var.project, var.environment)"
        result = check_naming_convention(content)
        assert result.passed is True


# ── check_no_hardcoded_secrets ────────────────────────────────────────────────


class TestCheckNoHardcodedSecrets(object):
    def test_pass_when_no_secrets(self, tmp_path: Path) -> None:
        tf_file = tmp_path / "main.tf"
        tf_file.write_text(VALID_MAIN_TF, encoding="utf-8")
        result = check_no_hardcoded_secrets(tmp_path)
        assert result.passed is True

    def test_fail_when_password_hardcoded(self, tmp_path: Path) -> None:
        tf_file = tmp_path / "main.tf"
        tf_file.write_text('db_password = "SuperSecret123!"\n', encoding="utf-8")
        result = check_no_hardcoded_secrets(tmp_path)
        assert result.passed is False
        assert "機密値" in result.message

    def test_pass_when_password_uses_var_reference(self, tmp_path: Path) -> None:
        tf_file = tmp_path / "main.tf"
        tf_file.write_text('db_password = var.db_password\n', encoding="utf-8")
        result = check_no_hardcoded_secrets(tmp_path)
        assert result.passed is True

    def test_fail_when_aws_access_key_present(self, tmp_path: Path) -> None:
        tf_file = tmp_path / "main.tf"
        tf_file.write_text('access_key = "AKIAIOSFODNN7EXAMPLE"\n', encoding="utf-8")
        result = check_no_hardcoded_secrets(tmp_path)
        assert result.passed is False

    def test_skip_dotterraform_directory(self, tmp_path: Path) -> None:
        dot_tf = tmp_path / ".terraform" / "providers"
        dot_tf.mkdir(parents=True)
        tf_file = dot_tf / "provider.tf"
        tf_file.write_text('password = "HardcodedSecret99!"\n', encoding="utf-8")
        result = check_no_hardcoded_secrets(tmp_path)
        assert result.passed is True

    def test_skip_commented_lines(self, tmp_path: Path) -> None:
        tf_file = tmp_path / "main.tf"
        tf_file.write_text('# password = "ShouldBeIgnored123!"\n', encoding="utf-8")
        result = check_no_hardcoded_secrets(tmp_path)
        assert result.passed is True

    def test_pass_with_placeholder_password(self, tmp_path: Path) -> None:
        tf_file = tmp_path / "terraform.tfvars.example"
        tf_file.write_text('db_password = "Change-me-securely-2024!"\n', encoding="utf-8")
        result = check_no_hardcoded_secrets(tmp_path)
        assert result.passed is True


# ── ValidationReport ──────────────────────────────────────────────────────────


class TestValidationReport:
    def test_has_errors_false_when_all_pass(self) -> None:
        report = ValidationReport()
        report.add(CheckResult("check1", True, "OK"))
        report.add(CheckResult("check2", True, "OK"))
        assert report.has_errors is False

    def test_has_errors_true_when_any_fail(self) -> None:
        report = ValidationReport()
        report.add(CheckResult("check1", True, "OK"))
        report.add(CheckResult("check2", False, "NG"))
        assert report.has_errors is True


# ── validate (統合テスト) ─────────────────────────────────────────────────────


class TestValidate:
    def test_valid_terraform_directory(self, tmp_path: Path) -> None:
        (tmp_path / "main.tf").write_text(VALID_MAIN_TF, encoding="utf-8")
        (tmp_path / "variables.tf").write_text(VALID_VARIABLES_TF, encoding="utf-8")
        report = validate(tmp_path)
        assert not report.has_errors, [r for r in report.results if not r.passed]

    def test_missing_main_tf(self, tmp_path: Path) -> None:
        report = validate(tmp_path)
        assert report.has_errors
        assert any("main.tf" in r.name for r in report.results)

    def test_missing_variables_tf(self, tmp_path: Path) -> None:
        (tmp_path / "main.tf").write_text(VALID_MAIN_TF, encoding="utf-8")
        report = validate(tmp_path)
        assert report.has_errors
        assert any("required_variables" in r.name for r in report.results)
