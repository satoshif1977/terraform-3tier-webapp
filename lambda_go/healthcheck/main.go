// healthcheck は ALB / EC2 / RDS のヘルス状態を確認する Lambda 関数。
//
// 環境変数:
//
//	ALB_TARGET_GROUP_ARN  - ALB ターゲットグループの ARN
//	EC2_INSTANCE_IDS      - カンマ区切りの EC2 インスタンス ID リスト
//	RDS_DB_IDENTIFIER     - RDS インスタンス識別子
//
// Lambda が受け取るイベントは任意（スケジュール実行を想定）。
// 正常終了時: 各リソースのヘルス情報を JSON で返す。
// 異常検知時: HealthSummary の Healthy フィールドが false になる（パニックはしない）。
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
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

// ── Lambda ハンドラー ────────────────────────────────────────────────────────

func handler(ctx context.Context, _ json.RawMessage) (*HealthSummary, error) {
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("AWS 設定の読み込みに失敗しました: %w", err)
	}

	checker := &Checker{
		elbv2:  elasticloadbalancingv2.NewFromConfig(cfg),
		ec2cli: ec2.NewFromConfig(cfg),
		rdsCli: rds.NewFromConfig(cfg),
	}

	targetGroupARN := os.Getenv("ALB_TARGET_GROUP_ARN")
	instanceIDsRaw := os.Getenv("EC2_INSTANCE_IDS")
	dbIdentifier := os.Getenv("RDS_DB_IDENTIFIER")

	var instanceIDs []string
	if instanceIDsRaw != "" {
		for _, id := range strings.Split(instanceIDsRaw, ",") {
			if trimmed := strings.TrimSpace(id); trimmed != "" {
				instanceIDs = append(instanceIDs, trimmed)
			}
		}
	}

	summary := &HealthSummary{Healthy: true}

	albHealth, err := checker.CheckALB(ctx, targetGroupARN)
	if err != nil {
		log.Printf("ALB チェックエラー: %v", err)
		summary.Healthy = false
	} else {
		summary.ALB = albHealth
		if albHealth != nil && !albHealth.AllHealthy {
			summary.Healthy = false
		}
	}

	ec2Health, err := checker.CheckEC2(ctx, instanceIDs)
	if err != nil {
		log.Printf("EC2 チェックエラー: %v", err)
		summary.Healthy = false
	} else {
		summary.EC2 = ec2Health
		if ec2Health != nil && !ec2Health.AllHealthy {
			summary.Healthy = false
		}
	}

	rdsHealth, err := checker.CheckRDS(ctx, dbIdentifier)
	if err != nil {
		log.Printf("RDS チェックエラー: %v", err)
		summary.Healthy = false
	} else {
		summary.RDS = rdsHealth
		if rdsHealth != nil && !rdsHealth.Healthy {
			summary.Healthy = false
		}
	}

	return summary, nil
}

func main() {
	lambda.Start(handler)
}
