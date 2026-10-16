// healthcheck は ALB / EC2 / RDS のヘルス状態を確認する Lambda 関数。
//
// 環境変数:
//
//	ALB_TARGET_GROUP_ARN  - ALB ターゲットグループの ARN
//	EC2_INSTANCE_IDS      - カンマ区切りの EC2 インスタンス ID リスト
//	RDS_DB_IDENTIFIER     - RDS インスタンス識別子
//	LOG_LEVEL             - ログレベル（debug/info/warn/error/silent・既定 info）
//	METRICS_NAMESPACE     - EMF メトリクスの名前空間（既定 Tier3Webapp/HealthCheck）
//
// Lambda が受け取るイベントは任意（スケジュール実行を想定）。
// 正常終了時: 各リソースのヘルス情報を JSON で返す。
// 異常検知時: HealthSummary の Healthy フィールドが false になる（パニックはしない）。
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	elbv2types "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"
	"github.com/aws/aws-sdk-go-v2/service/rds"
)

// ── インターフェース ──────────────────────────────────────────────────────────

// ELBV2Client は ALB ターゲットグループのヘルス確認に必要なメソッドを定義する。
// テスト時にモックで差し替えられるよう interface に分離している。
type ELBV2Client interface {
	DescribeTargetHealth(
		ctx context.Context,
		input *elasticloadbalancingv2.DescribeTargetHealthInput,
		opts ...func(*elasticloadbalancingv2.Options),
	) (*elasticloadbalancingv2.DescribeTargetHealthOutput, error)
}

// EC2Client は EC2 インスタンス状態確認に必要なメソッドを定義する。
type EC2Client interface {
	DescribeInstanceStatus(
		ctx context.Context,
		input *ec2.DescribeInstanceStatusInput,
		opts ...func(*ec2.Options),
	) (*ec2.DescribeInstanceStatusOutput, error)
}

// RDSClient は RDS インスタンス状態確認に必要なメソッドを定義する。
type RDSClient interface {
	DescribeDBInstances(
		ctx context.Context,
		input *rds.DescribeDBInstancesInput,
		opts ...func(*rds.Options),
	) (*rds.DescribeDBInstancesOutput, error)
}

// ── レスポンス型 ──────────────────────────────────────────────────────────────

// TargetStatus は ALB ターゲット 1 台分のヘルス状態を表す。
type TargetStatus struct {
	ID     string `json:"id"`
	Port   int32  `json:"port"`
	State  string `json:"state"`
	Reason string `json:"reason,omitempty"`
}

// ALBHealth は ALB ターゲットグループ全体のヘルス情報を表す。
type ALBHealth struct {
	TargetGroupARN string         `json:"target_group_arn"`
	Targets        []TargetStatus `json:"targets"`
	AllHealthy     bool           `json:"all_healthy"`
}

// InstanceStatus は EC2 インスタンス 1 台分の状態を表す。
type InstanceStatus struct {
	InstanceID    string `json:"instance_id"`
	InstanceState string `json:"instance_state"`
	SystemStatus  string `json:"system_status"`
	InstanceCheck string `json:"instance_check"`
}

// EC2Health は EC2 ヘルス確認結果の集合を表す。
type EC2Health struct {
	Instances  []InstanceStatus `json:"instances"`
	AllHealthy bool             `json:"all_healthy"`
}

// RDSHealth は RDS インスタンスの状態を表す。
type RDSHealth struct {
	DBIdentifier string `json:"db_identifier"`
	Status       string `json:"status"`
	Healthy      bool   `json:"healthy"`
}

// HealthSummary は Lambda レスポンスのトップレベル構造体。
type HealthSummary struct {
	ALB     *ALBHealth `json:"alb,omitempty"`
	EC2     *EC2Health `json:"ec2,omitempty"`
	RDS     *RDSHealth `json:"rds,omitempty"`
	Healthy bool       `json:"healthy"`
}

// ── Checker ──────────────────────────────────────────────────────────────────

// Checker はヘルスチェックロジックをまとめた構造体。
// フィールドに interface を持つことでテスト時にモックを注入できる。
type Checker struct {
	elbv2  ELBV2Client
	ec2cli EC2Client
	rdsCli RDSClient
}

// CheckALB は指定したターゲットグループのヘルス状態を返す。
// targetGroupARN が空の場合は nil を返す。
func (c *Checker) CheckALB(ctx context.Context, targetGroupARN string) (*ALBHealth, error) {
	if targetGroupARN == "" {
		return nil, nil
	}
	out, err := c.elbv2.DescribeTargetHealth(ctx, &elasticloadbalancingv2.DescribeTargetHealthInput{
		TargetGroupArn: aws.String(targetGroupARN),
	})
	if err != nil {
		return nil, fmt.Errorf("ALB DescribeTargetHealth: %w", err)
	}

	health := &ALBHealth{
		TargetGroupARN: targetGroupARN,
		AllHealthy:     true,
	}
	for _, thd := range out.TargetHealthDescriptions {
		state := string(thd.TargetHealth.State)
		reason := string(thd.TargetHealth.Reason)
		ts := TargetStatus{
			State:  state,
			Reason: reason,
		}
		if thd.Target != nil {
			if thd.Target.Id != nil {
				ts.ID = *thd.Target.Id
			}
			if thd.Target.Port != nil {
				ts.Port = *thd.Target.Port
			}
		}
		if state != string(elbv2types.TargetHealthStateEnumHealthy) {
			health.AllHealthy = false
		}
		health.Targets = append(health.Targets, ts)
	}
	return health, nil
}

// CheckEC2 は指定したインスタンス ID リストの状態を返す。
// instanceIDs が空の場合は nil を返す。
func (c *Checker) CheckEC2(ctx context.Context, instanceIDs []string) (*EC2Health, error) {
	if len(instanceIDs) == 0 {
		return nil, nil
	}
	out, err := c.ec2cli.DescribeInstanceStatus(ctx, &ec2.DescribeInstanceStatusInput{
		InstanceIds:         instanceIDs,
		IncludeAllInstances: aws.Bool(true),
	})
	if err != nil {
		return nil, fmt.Errorf("EC2 DescribeInstanceStatus: %w", err)
	}

	health := &EC2Health{AllHealthy: true}
	for _, s := range out.InstanceStatuses {
		inst := InstanceStatus{
			InstanceState: string(s.InstanceState.Name),
			SystemStatus:  string(s.SystemStatus.Status),
			InstanceCheck: string(s.InstanceStatus.Status),
		}
		if s.InstanceId != nil {
			inst.InstanceID = *s.InstanceId
		}
		if inst.InstanceState != string(ec2types.InstanceStateNameRunning) ||
			inst.SystemStatus != string(ec2types.SummaryStatusOk) ||
			inst.InstanceCheck != string(ec2types.SummaryStatusOk) {
			health.AllHealthy = false
		}
		health.Instances = append(health.Instances, inst)
	}
	return health, nil
}

// CheckRDS は指定した RDS インスタンスの状態を返す。
// dbIdentifier が空の場合は nil を返す。
func (c *Checker) CheckRDS(ctx context.Context, dbIdentifier string) (*RDSHealth, error) {
	if dbIdentifier == "" {
		return nil, nil
	}
	out, err := c.rdsCli.DescribeDBInstances(ctx, &rds.DescribeDBInstancesInput{
		DBInstanceIdentifier: aws.String(dbIdentifier),
	})
	if err != nil {
		return nil, fmt.Errorf("RDS DescribeDBInstances: %w", err)
	}
	if len(out.DBInstances) == 0 {
		return &RDSHealth{DBIdentifier: dbIdentifier, Status: "not-found", Healthy: false}, nil
	}
	db := out.DBInstances[0]
	status := aws.ToString(db.DBInstanceStatus)
	return &RDSHealth{
		DBIdentifier: dbIdentifier,
		Status:       status,
		Healthy:      status == "available",
	}, nil
}

// ── 集計ヘルパー ──────────────────────────────────────────────────────────────

// UnhealthyCount は healthy 以外のターゲット数を返す。
func (h *ALBHealth) UnhealthyCount() int {
	n := 0
	for _, t := range h.Targets {
		if t.State != string(elbv2types.TargetHealthStateEnumHealthy) {
			n++
		}
	}
	return n
}

// UnhealthyCount は running かつステータスチェック 2/2 成功でないインスタンス数を返す。
func (h *EC2Health) UnhealthyCount() int {
	n := 0
	for _, i := range h.Instances {
		if i.InstanceState != string(ec2types.InstanceStateNameRunning) ||
			i.SystemStatus != string(ec2types.SummaryStatusOk) ||
			i.InstanceCheck != string(ec2types.SummaryStatusOk) {
			n++
		}
	}
	return n
}

// ── 設定 ──────────────────────────────────────────────────────────────────────

// defaultMetricsNamespace は METRICS_NAMESPACE 未設定時に使う名前空間。
const defaultMetricsNamespace = "Tier3Webapp/HealthCheck"

// checkConfig は環境変数から読み取ったチェック対象。
type checkConfig struct {
	TargetGroupARN string
	InstanceIDs    []string
	DBIdentifier   string
}

// loadCheckConfig は環境変数からチェック対象を組み立てる。
// EC2_INSTANCE_IDS はカンマ区切りで、空要素と前後の空白は捨てる。
func loadCheckConfig() checkConfig {
	cfg := checkConfig{
		TargetGroupARN: strings.TrimSpace(os.Getenv("ALB_TARGET_GROUP_ARN")),
		DBIdentifier:   strings.TrimSpace(os.Getenv("RDS_DB_IDENTIFIER")),
	}
	for _, id := range strings.Split(os.Getenv("EC2_INSTANCE_IDS"), ",") {
		if trimmed := strings.TrimSpace(id); trimmed != "" {
			cfg.InstanceIDs = append(cfg.InstanceIDs, trimmed)
		}
	}
	return cfg
}

// ── メトリクス ────────────────────────────────────────────────────────────────

// boolMetric は真偽値を CloudWatch に載せられる 1 / 0 に変換する。
func boolMetric(ok bool) float64 {
	if ok {
		return 1
	}
	return 0
}

// recordCount はカウントメトリクスを 1 件記録する。
//
// メトリクスの記録に失敗してもヘルスチェック自体は続ける。
// 監視のための計装が、監視対象の可用性を下げては本末転倒なため。
func recordCount(logger *slog.Logger, metrics *Metrics, name string, value float64) {
	if metrics == nil {
		return
	}
	if err := metrics.Count(name, value); err != nil {
		logger.Warn("メトリクスの記録に失敗しました", "metric", name, "error", err.Error())
	}
}

// ── ヘルスチェック本体 ────────────────────────────────────────────────────────

// runHealthCheck は ALB / EC2 / RDS のヘルスチェックを実行し、結果を集約する。
//
// AWS SDK の生成を handler 側に残してあるため、テストではモックを注入した Checker と、
// メモリ上の Writer を持つ logger / metrics を渡して出力まで検証できる。
//
// いずれかのチェックが失敗しても残りは実行する。1 つの API エラーで
// 他のリソースの状態が分からなくなるのを避けるため。
func runHealthCheck(
	ctx context.Context,
	checker *Checker,
	logger *slog.Logger,
	metrics *Metrics,
	cfg checkConfig,
) *HealthSummary {
	summary := &HealthSummary{Healthy: true}
	checkErrors := 0

	// ALB
	albHealth, err := checker.CheckALB(ctx, cfg.TargetGroupARN)
	switch {
	case err != nil:
		logger.Error("ALB のヘルスチェックに失敗しました", "error", err.Error())
		summary.Healthy = false
		checkErrors++
	case albHealth != nil:
		summary.ALB = albHealth
		unhealthy := albHealth.UnhealthyCount()
		logger.Info("ALB のヘルスチェックが完了しました",
			"target_group_arn", albHealth.TargetGroupARN,
			"targets", len(albHealth.Targets),
			"unhealthy", unhealthy,
			"all_healthy", albHealth.AllHealthy,
		)
		recordCount(logger, metrics, "ALBHealthy", boolMetric(albHealth.AllHealthy))
		recordCount(logger, metrics, "ALBUnhealthyTargets", float64(unhealthy))
		if !albHealth.AllHealthy {
			summary.Healthy = false
		}
	default:
		logger.Debug("ALB のヘルスチェックをスキップしました", "reason", "ALB_TARGET_GROUP_ARN が未設定")
	}

	// EC2
	ec2Health, err := checker.CheckEC2(ctx, cfg.InstanceIDs)
	switch {
	case err != nil:
		logger.Error("EC2 のヘルスチェックに失敗しました", "error", err.Error())
		summary.Healthy = false
		checkErrors++
	case ec2Health != nil:
		summary.EC2 = ec2Health
		unhealthy := ec2Health.UnhealthyCount()
		logger.Info("EC2 のヘルスチェックが完了しました",
			"instances", len(ec2Health.Instances),
			"unhealthy", unhealthy,
			"all_healthy", ec2Health.AllHealthy,
		)
		recordCount(logger, metrics, "EC2Healthy", boolMetric(ec2Health.AllHealthy))
		recordCount(logger, metrics, "EC2UnhealthyInstances", float64(unhealthy))
		if !ec2Health.AllHealthy {
			summary.Healthy = false
		}
	default:
		logger.Debug("EC2 のヘルスチェックをスキップしました", "reason", "EC2_INSTANCE_IDS が未設定")
	}

	// RDS
	rdsHealth, err := checker.CheckRDS(ctx, cfg.DBIdentifier)
	switch {
	case err != nil:
		logger.Error("RDS のヘルスチェックに失敗しました", "error", err.Error())
		summary.Healthy = false
		checkErrors++
	case rdsHealth != nil:
		summary.RDS = rdsHealth
		logger.Info("RDS のヘルスチェックが完了しました",
			"db_identifier", rdsHealth.DBIdentifier,
			"status", rdsHealth.Status,
			"healthy", rdsHealth.Healthy,
		)
		recordCount(logger, metrics, "RDSHealthy", boolMetric(rdsHealth.Healthy))
		if !rdsHealth.Healthy {
			summary.Healthy = false
		}
	default:
		logger.Debug("RDS のヘルスチェックをスキップしました", "reason", "RDS_DB_IDENTIFIER が未設定")
	}

	recordCount(logger, metrics, "CheckErrors", float64(checkErrors))
	recordCount(logger, metrics, "OverallHealthy", boolMetric(summary.Healthy))

	if summary.Healthy {
		logger.Info("ヘルスチェックが完了しました", "healthy", true)
	} else {
		logger.Warn("ヘルスチェックで異常を検知しました", "healthy", false, "check_errors", checkErrors)
	}
	return summary
}

// ── Lambda ハンドラー ────────────────────────────────────────────────────────

func handler(ctx context.Context, _ json.RawMessage) (*HealthSummary, error) {
	logger := NewLoggerFromEnv(LoggerOptions{})

	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		logger.Error("AWS 設定の読み込みに失敗しました", "error", err.Error())
		return nil, fmt.Errorf("AWS 設定の読み込みに失敗しました: %w", err)
	}

	namespace := strings.TrimSpace(os.Getenv("METRICS_NAMESPACE"))
	if namespace == "" {
		namespace = defaultMetricsNamespace
	}
	metrics, err := NewMetrics(MetricsOptions{Namespace: namespace})
	if err != nil {
		// メトリクスを出せないことはヘルスチェックを止める理由にならない。
		logger.Warn("メトリクスの初期化に失敗しました。計装なしで継続します", "error", err.Error())
		metrics = nil
	}

	checker := &Checker{
		elbv2:  elasticloadbalancingv2.NewFromConfig(cfg),
		ec2cli: ec2.NewFromConfig(cfg),
		rdsCli: rds.NewFromConfig(cfg),
	}

	recordCount(logger, metrics, "HealthCheckInvocation", 1)
	summary := runHealthCheck(ctx, checker, logger, metrics, loadCheckConfig())

	if metrics != nil {
		if err := metrics.Flush(); err != nil {
			logger.Warn("メトリクスの出力に失敗しました", "error", err.Error())
		}
	}
	return summary, nil
}

func main() {
	lambda.Start(handler)
}
