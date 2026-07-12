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


VALID_MAIN_TF = textwrap.dedent("""\
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
    """)

VALID_VARIABLES_TF = textwrap.dedent("""\
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
    """)


# ── check_required_version ────────────────────────────────────────────────────


class TestCheckRequiredVersion:
    def test_pass_when_version_exists(self) -> None:
        result = check_required_version(VALID_MAIN_TF)
        assert result.passed is True
        assert ">= 1.5" in result.message

    def test_fail_when_version_missing(self) -> None:
        content = "terraform {\n  required_providers {}\n}"
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
        content = textwrap.dedent("""\
            provider "aws" {
              default_tags {
                tags = {
                  Project   = var.project
                  ManagedBy = "Terraform"
                }
              }
            }
            """)
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
        tf_file.write_text("db_password = var.db_password\n", encoding="utf-8")
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
        tf_file.write_text(
            'db_password = "Change-me-securely-2024!"\n', encoding="utf-8"
        )
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


# ── check_required_version 追加 ───────────────────────────────────────────────


class TestCheckRequiredVersionExtra:
    def test_pass_with_patch_version(self) -> None:
        content = 'terraform {\n  required_version = ">= 1.5.0"\n}'
        result = check_required_version(content)
        assert result.passed is True
        assert "1.5.0" in result.message

    def test_fail_on_empty_content(self) -> None:
        result = check_required_version("")
        assert result.passed is False

    def test_pass_with_exact_version(self) -> None:
        content = 'terraform {\n  required_version = "= 1.9.3"\n}'
        result = check_required_version(content)
        assert result.passed is True


# ── check_default_tags 追加 ───────────────────────────────────────────────────


class TestCheckDefaultTagsExtra:
    def test_fail_when_project_tag_missing(self) -> None:
        content = textwrap.dedent("""\
            provider "aws" {
              default_tags {
                tags = {
                  Environment = var.environment
                  ManagedBy   = "Terraform"
                }
              }
            }
            """)
        result = check_default_tags(content)
        assert result.passed is False
        assert "Project" in result.message

    def test_fail_when_managedby_tag_missing(self) -> None:
        content = textwrap.dedent("""\
            provider "aws" {
              default_tags {
                tags = {
                  Project     = var.project
                  Environment = var.environment
                }
              }
            }
            """)
        result = check_default_tags(content)
        assert result.passed is False
        assert "ManagedBy" in result.message


# ── check_no_hardcoded_secrets 追加 ──────────────────────────────────────────


class TestCheckNoHardcodedSecretsExtra:
    def test_fail_when_secret_present(self, tmp_path: Path) -> None:
        # "secret" キーワードに直接値がある場合は検出される
        tf_file = tmp_path / "main.tf"
        tf_file.write_text('secret = "AbCdEfGhIjKlMnOpQ123"\n', encoding="utf-8")
        result = check_no_hardcoded_secrets(tmp_path)
        assert result.passed is False

    def test_fail_when_token_hardcoded(self, tmp_path: Path) -> None:
        tf_file = tmp_path / "main.tf"
        tf_file.write_text(
            'token = "ghp_SampleTokenValueXYZ123456789"\n', encoding="utf-8"
        )
        result = check_no_hardcoded_secrets(tmp_path)
        assert result.passed is False

    def test_pass_with_empty_directory(self, tmp_path: Path) -> None:
        result = check_no_hardcoded_secrets(tmp_path)
        assert result.passed is True


# ── ValidationReport 追加 ────────────────────────────────────────────────────


class TestValidationReportExtra:
    def test_has_errors_false_on_empty_report(self) -> None:
        report = ValidationReport()
        assert report.has_errors is False

    def test_results_count_matches_added(self) -> None:
        report = ValidationReport()
        report.add(CheckResult("a", True, "OK"))
        report.add(CheckResult("b", False, "NG"))
        report.add(CheckResult("c", True, "OK"))
        assert len(report.results) == 3


# ── check_naming_convention 追加 ──────────────────────────────────────────────


class TestCheckNamingConventionExtra:
    def test_fail_when_only_environment_used(self) -> None:
        content = 'name = "${var.environment}-vpc"'
        result = check_naming_convention(content)
        assert result.passed is False
        assert "var.project" in result.message

    def test_pass_with_vars_on_separate_lines(self) -> None:
        content = textwrap.dedent("""\
            resource "aws_instance" "main" {
              name = "${var.project}-server"
              env  = var.environment
            }
            """)
        result = check_naming_convention(content)
        assert result.passed is True

    def test_fail_message_lists_both_missing(self) -> None:
        result = check_naming_convention("")
        assert "var.project" in result.message
        assert "var.environment" in result.message


# ── CheckResult 追加 ──────────────────────────────────────────────────────────


class TestCheckResultExtra:
    def test_check_result_with_detail(self) -> None:
        result = CheckResult("test_check", False, "NG message", detail="追加情報あり")
        assert result.detail == "追加情報あり"
        assert result.passed is False

    def test_check_result_without_detail_is_none(self) -> None:
        result = CheckResult("test_check", True, "OK message")
        assert result.detail is None

    def test_check_result_name_field(self) -> None:
        result = CheckResult("my_check", True, "OK")
        assert result.name == "my_check"


# ── check_no_hardcoded_secrets さらに追加 ────────────────────────────────────


class TestCheckNoHardcodedSecretsMore:
    def test_fail_when_aws_account_id_present(self, tmp_path: Path) -> None:
        tf_file = tmp_path / "main.tf"
        tf_file.write_text('account_id = "123456789012"\n', encoding="utf-8")
        result = check_no_hardcoded_secrets(tmp_path)
        assert result.passed is False

    def test_multiple_clean_tf_files_all_pass(self, tmp_path: Path) -> None:
        (tmp_path / "main.tf").write_text(VALID_MAIN_TF, encoding="utf-8")
        (tmp_path / "variables.tf").write_text(VALID_VARIABLES_TF, encoding="utf-8")
        result = check_no_hardcoded_secrets(tmp_path)
        assert result.passed is True

    def test_finding_count_in_message(self, tmp_path: Path) -> None:
        tf_file = tmp_path / "main.tf"
        tf_file.write_text(
            'password = "Secret1234!"\ntoken = "ghp_TokenValue12345678"\n',
            encoding="utf-8",
        )
        result = check_no_hardcoded_secrets(tmp_path)
        assert result.passed is False
        assert "2" in result.message


# ── validate さらに追加 ───────────────────────────────────────────────────────


class TestValidateExtra:
    def test_validate_returns_report_type(self, tmp_path: Path) -> None:
        report = validate(tmp_path)
        assert isinstance(report, ValidationReport)

    def test_validate_naming_convention_fail_without_vars(
        self, tmp_path: Path
    ) -> None:
        # default_tags を固定文字列にして var.project / var.environment を使わない main.tf
        main_tf = textwrap.dedent("""\
            terraform {
              required_version = ">= 1.5"
              required_providers {
                aws = { source = "hashicorp/aws" }
              }
            }
            provider "aws" {
              region = "ap-northeast-1"
              default_tags {
                tags = {
                  Project     = "myapp"
                  Environment = "dev"
                  ManagedBy   = "Terraform"
                }
              }
            }
            resource "aws_vpc" "main" {
              cidr_block = "10.0.0.0/16"
            }
            """)
        (tmp_path / "main.tf").write_text(main_tf, encoding="utf-8")
        (tmp_path / "variables.tf").write_text(VALID_VARIABLES_TF, encoding="utf-8")
        report = validate(tmp_path)
        naming = [r for r in report.results if r.name == "naming_convention"]
        assert naming and not naming[0].passed
