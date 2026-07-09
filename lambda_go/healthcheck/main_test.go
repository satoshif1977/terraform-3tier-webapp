package main

import (
	"context"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	elbv2types "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"
)

// ── モック ────────────────────────────────────────────────────────────────────

type mockELBV2Client struct {
	output *elasticloadbalancingv2.DescribeTargetHealthOutput
	err    error
}

func (m *mockELBV2Client) DescribeTargetHealth(
	_ context.Context,
	_ *elasticloadbalancingv2.DescribeTargetHealthInput,
	_ ...func(*elasticloadbalancingv2.Options),
) (*elasticloadbalancingv2.DescribeTargetHealthOutput, error) {
	return m.output, m.err
}

type mockEC2Client struct {
	output *ec2.DescribeInstanceStatusOutput
	err    error
}

func (m *mockEC2Client) DescribeInstanceStatus(
	_ context.Context,
	_ *ec2.DescribeInstanceStatusInput,
	_ ...func(*ec2.Options),
) (*ec2.DescribeInstanceStatusOutput, error) {
	return m.output, m.err
}

type mockRDSClient struct {
	output *rds.DescribeDBInstancesOutput
	err    error
}

func (m *mockRDSClient) DescribeDBInstances(
	_ context.Context,
	_ *rds.DescribeDBInstancesInput,
	_ ...func(*rds.Options),
) (*rds.DescribeDBInstancesOutput, error) {
	return m.output, m.err
}

// ── CheckALB ──────────────────────────────────────────────────────────────────

func TestCheckALB_AllHealthy(t *testing.T) {
	checker := &Checker{
		elbv2: &mockELBV2Client{
			output: &elasticloadbalancingv2.DescribeTargetHealthOutput{
				TargetHealthDescriptions: []elbv2types.TargetHealthDescription{
					{
						Target: &elbv2types.TargetDescription{
							Id:   aws.String("i-0123456789abcdef0"),
							Port: aws.Int32(80),
						},
						TargetHealth: &elbv2types.TargetHealth{
							State: elbv2types.TargetHealthStateEnumHealthy,
						},
					},
				},
			},
		},
	}

	result, err := checker.CheckALB(context.Background(), "arn:aws:elasticloadbalancing:ap-northeast-1:123456789012:targetgroup/test/abc")
	if err != nil {
		t.Fatalf("CheckALB returned unexpected error: %v", err)
	}
	if !result.AllHealthy {
		t.Error("expected AllHealthy=true, got false")
	}
	if len(result.Targets) != 1 {
		t.Errorf("expected 1 target, got %d", len(result.Targets))
	}
	if result.Targets[0].State != string(elbv2types.TargetHealthStateEnumHealthy) {
		t.Errorf("unexpected target state: %s", result.Targets[0].State)
	}
}

func TestCheckALB_Unhealthy(t *testing.T) {
	checker := &Checker{
		elbv2: &mockELBV2Client{
			output: &elasticloadbalancingv2.DescribeTargetHealthOutput{
				TargetHealthDescriptions: []elbv2types.TargetHealthDescription{
					{
						Target: &elbv2types.TargetDescription{
							Id:   aws.String("i-unhealthy"),
							Port: aws.Int32(80),
						},
						TargetHealth: &elbv2types.TargetHealth{
							State:  elbv2types.TargetHealthStateEnumUnhealthy,
							Reason: elbv2types.TargetHealthReasonEnumFailedHealthChecks,
						},
					},
				},
			},
		},
	}

	result, err := checker.CheckALB(context.Background(), "arn:aws:elasticloadbalancing:ap-northeast-1:123456789012:targetgroup/test/abc")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.AllHealthy {
		t.Error("expected AllHealthy=false, got true")
	}
}

func TestCheckALB_EmptyARN(t *testing.T) {
	checker := &Checker{elbv2: &mockELBV2Client{}}
	result, err := checker.CheckALB(context.Background(), "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != nil {
		t.Error("expected nil result for empty ARN")
	}
}

func TestCheckALB_APIError(t *testing.T) {
	checker := &Checker{
		elbv2: &mockELBV2Client{err: fmt.Errorf("API error")},
	}
	_, err := checker.CheckALB(context.Background(), "arn:aws:elasticloadbalancing:ap-northeast-1:123456789012:targetgroup/test/abc")
	if err == nil {
		t.Error("expected error, got nil")
	}
}

// ── CheckEC2 ──────────────────────────────────────────────────────────────────

func TestCheckEC2_AllHealthy(t *testing.T) {
	checker := &Checker{
		ec2cli: &mockEC2Client{
			output: &ec2.DescribeInstanceStatusOutput{
				InstanceStatuses: []ec2types.InstanceStatus{
					{
						InstanceId: aws.String("i-0abc123456789def0"),
						InstanceState: &ec2types.InstanceState{
							Name: ec2types.InstanceStateNameRunning,
						},
						SystemStatus: &ec2types.InstanceStatusSummary{
							Status: ec2types.SummaryStatusOk,
						},
						InstanceStatus: &ec2types.InstanceStatusSummary{
							Status: ec2types.SummaryStatusOk,
						},
					},
				},
			},
		},
	}

	result, err := checker.CheckEC2(context.Background(), []string{"i-0abc123456789def0"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.AllHealthy {
		t.Error("expected AllHealthy=true, got false")
	}
	if len(result.Instances) != 1 {
		t.Errorf("expected 1 instance, got %d", len(result.Instances))
	}
}

func TestCheckEC2_StoppedInstance(t *testing.T) {
	checker := &Checker{
		ec2cli: &mockEC2Client{
			output: &ec2.DescribeInstanceStatusOutput{
				InstanceStatuses: []ec2types.InstanceStatus{
					{
						InstanceId: aws.String("i-stopped"),
						InstanceState: &ec2types.InstanceState{
							Name: ec2types.InstanceStateNameStopped,
						},
						SystemStatus: &ec2types.InstanceStatusSummary{
							Status: ec2types.SummaryStatusNotApplicable,
						},
						InstanceStatus: &ec2types.InstanceStatusSummary{
							Status: ec2types.SummaryStatusNotApplicable,
						},
					},
				},
			},
		},
	}

	result, err := checker.CheckEC2(context.Background(), []string{"i-stopped"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.AllHealthy {
		t.Error("expected AllHealthy=false for stopped instance")
	}
}

func TestCheckEC2_EmptyInstanceIDs(t *testing.T) {
	checker := &Checker{ec2cli: &mockEC2Client{}}
	result, err := checker.CheckEC2(context.Background(), []string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != nil {
		t.Error("expected nil result for empty instance IDs")
	}
}

func TestCheckEC2_APIError(t *testing.T) {
	checker := &Checker{
		ec2cli: &mockEC2Client{err: fmt.Errorf("API error")},
	}
	_, err := checker.CheckEC2(context.Background(), []string{"i-0000"})
	if err == nil {
		t.Error("expected error, got nil")
	}
}

// ── CheckRDS ──────────────────────────────────────────────────────────────────

func TestCheckRDS_Available(t *testing.T) {
	checker := &Checker{
		rdsCli: &mockRDSClient{
			output: &rds.DescribeDBInstancesOutput{
				DBInstances: []rdstypes.DBInstance{
					{
						DBInstanceIdentifier: aws.String("webapp-dev-rds"),
						DBInstanceStatus:     aws.String("available"),
					},
				},
			},
		},
	}

	result, err := checker.CheckRDS(context.Background(), "webapp-dev-rds")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Healthy {
		t.Error("expected Healthy=true for available RDS")
	}
	if result.Status != "available" {
		t.Errorf("expected status=available, got %s", result.Status)
	}
}

func TestCheckRDS_Stopped(t *testing.T) {
	checker := &Checker{
		rdsCli: &mockRDSClient{
			output: &rds.DescribeDBInstancesOutput{
				DBInstances: []rdstypes.DBInstance{
					{
						DBInstanceIdentifier: aws.String("webapp-dev-rds"),
						DBInstanceStatus:     aws.String("stopped"),
					},
				},
			},
		},
	}

	result, err := checker.CheckRDS(context.Background(), "webapp-dev-rds")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Healthy {
		t.Error("expected Healthy=false for stopped RDS")
	}
}

func TestCheckRDS_NotFound(t *testing.T) {
	checker := &Checker{
		rdsCli: &mockRDSClient{
			output: &rds.DescribeDBInstancesOutput{DBInstances: []rdstypes.DBInstance{}},
		},
	}

	result, err := checker.CheckRDS(context.Background(), "non-existent-db")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Healthy {
		t.Error("expected Healthy=false for not-found RDS")
	}
	if result.Status != "not-found" {
		t.Errorf("expected status=not-found, got %s", result.Status)
	}
}

func TestCheckRDS_EmptyIdentifier(t *testing.T) {
	checker := &Checker{rdsCli: &mockRDSClient{}}
	result, err := checker.CheckRDS(context.Background(), "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != nil {
		t.Error("expected nil result for empty identifier")
	}
}

func TestCheckRDS_APIError(t *testing.T) {
	checker := &Checker{
		rdsCli: &mockRDSClient{err: fmt.Errorf("API error")},
	}
	_, err := checker.CheckRDS(context.Background(), "webapp-dev-rds")
	if err == nil {
		t.Error("expected error, got nil")
	}
}

// ── CheckALB 追加テスト ───────────────────────────────────────────────────────

func TestCheckALB_NoTargets(t *testing.T) {
	// ターゲットが 0 件の場合は AllHealthy=true（問題となるターゲットがない）
	checker := &Checker{
		elbv2: &mockELBV2Client{
			output: &elasticloadbalancingv2.DescribeTargetHealthOutput{
				TargetHealthDescriptions: []elbv2types.TargetHealthDescription{},
			},
		},
	}
	result, err := checker.CheckALB(context.Background(), "arn:aws:elasticloadbalancing:ap-northeast-1:123:targetgroup/test/abc")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.AllHealthy {
		t.Error("expected AllHealthy=true when no targets")
	}
}

func TestCheckALB_MixedTargets(t *testing.T) {
	// healthy と unhealthy が混在する場合は AllHealthy=false
	checker := &Checker{
		elbv2: &mockELBV2Client{
			output: &elasticloadbalancingv2.DescribeTargetHealthOutput{
				TargetHealthDescriptions: []elbv2types.TargetHealthDescription{
					{
						Target: &elbv2types.TargetDescription{Id: aws.String("i-ok"), Port: aws.Int32(80)},
						TargetHealth: &elbv2types.TargetHealth{State: elbv2types.TargetHealthStateEnumHealthy},
					},
					{
						Target: &elbv2types.TargetDescription{Id: aws.String("i-ng"), Port: aws.Int32(80)},
						TargetHealth: &elbv2types.TargetHealth{
							State:  elbv2types.TargetHealthStateEnumUnhealthy,
							Reason: elbv2types.TargetHealthReasonEnumFailedHealthChecks,
						},
					},
				},
			},
		},
	}
	result, err := checker.CheckALB(context.Background(), "arn:aws:elasticloadbalancing:ap-northeast-1:123:targetgroup/test/abc")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.AllHealthy {
		t.Error("expected AllHealthy=false when mixed targets")
	}
	if len(result.Targets) != 2 {
		t.Errorf("expected 2 targets, got %d", len(result.Targets))
	}
}

// ── CheckEC2 追加テスト ───────────────────────────────────────────────────────

func TestCheckEC2_MultipleInstances_OneUnhealthy(t *testing.T) {
	checker := &Checker{
		ec2cli: &mockEC2Client{
			output: &ec2.DescribeInstanceStatusOutput{
				InstanceStatuses: []ec2types.InstanceStatus{
					{
						InstanceId:    aws.String("i-ok"),
						InstanceState: &ec2types.InstanceState{Name: ec2types.InstanceStateNameRunning},
						SystemStatus:  &ec2types.InstanceStatusSummary{Status: ec2types.SummaryStatusOk},
						InstanceStatus: &ec2types.InstanceStatusSummary{Status: ec2types.SummaryStatusOk},
					},
					{
						InstanceId:    aws.String("i-ng"),
						InstanceState: &ec2types.InstanceState{Name: ec2types.InstanceStateNameStopped},
						SystemStatus:  &ec2types.InstanceStatusSummary{Status: ec2types.SummaryStatusNotApplicable},
						InstanceStatus: &ec2types.InstanceStatusSummary{Status: ec2types.SummaryStatusNotApplicable},
					},
				},
			},
		},
	}
	result, err := checker.CheckEC2(context.Background(), []string{"i-ok", "i-ng"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.AllHealthy {
		t.Error("expected AllHealthy=false when one instance is stopped")
	}
	if len(result.Instances) != 2 {
		t.Errorf("expected 2 instances, got %d", len(result.Instances))
	}
}

func TestCheckEC2_PendingInstance(t *testing.T) {
	// pending 状態はまだ起動中 → AllHealthy=false
	checker := &Checker{
		ec2cli: &mockEC2Client{
			output: &ec2.DescribeInstanceStatusOutput{
				InstanceStatuses: []ec2types.InstanceStatus{
					{
						InstanceId:    aws.String("i-pending"),
						InstanceState: &ec2types.InstanceState{Name: ec2types.InstanceStateNamePending},
						SystemStatus:  &ec2types.InstanceStatusSummary{Status: ec2types.SummaryStatusInitializing},
						InstanceStatus: &ec2types.InstanceStatusSummary{Status: ec2types.SummaryStatusInitializing},
					},
				},
			},
		},
	}
	result, err := checker.CheckEC2(context.Background(), []string{"i-pending"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.AllHealthy {
		t.Error("expected AllHealthy=false for pending instance")
	}
}

// ── CheckRDS 追加テスト ───────────────────────────────────────────────────────

func TestCheckRDS_ModifyingStatus(t *testing.T) {
	// modifying 状態は available でない → Healthy=false
	checker := &Checker{
		rdsCli: &mockRDSClient{
			output: &rds.DescribeDBInstancesOutput{
				DBInstances: []rdstypes.DBInstance{
					{
						DBInstanceIdentifier: aws.String("webapp-dev-rds"),
						DBInstanceStatus:     aws.String("modifying"),
					},
				},
			},
		},
	}
	result, err := checker.CheckRDS(context.Background(), "webapp-dev-rds")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Healthy {
		t.Error("expected Healthy=false for modifying RDS")
	}
	if result.Status != "modifying" {
		t.Errorf("expected status=modifying, got %s", result.Status)
	}
}

func TestCheckRDS_IdentifierPreservedInResult(t *testing.T) {
	// レスポンスに渡した dbIdentifier が含まれること
	checker := &Checker{
		rdsCli: &mockRDSClient{
			output: &rds.DescribeDBInstancesOutput{
				DBInstances: []rdstypes.DBInstance{
					{
						DBInstanceIdentifier: aws.String("my-db"),
						DBInstanceStatus:     aws.String("available"),
					},
				},
			},
		},
	}
	result, err := checker.CheckRDS(context.Background(), "my-db")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.DBIdentifier != "my-db" {
		t.Errorf("expected DBIdentifier=my-db, got %s", result.DBIdentifier)
	}
}

// ── CheckALB draining / initial / TargetGroupARN / 3ターゲット ────────────

func TestCheckALB_DrainingTarget(t *testing.T) {
	// draining 状態のターゲットは AllHealthy=false
	checker := &Checker{
		elbv2: &mockELBV2Client{
			output: &elasticloadbalancingv2.DescribeTargetHealthOutput{
				TargetHealthDescriptions: []elbv2types.TargetHealthDescription{
					{
						Target:       &elbv2types.TargetDescription{Id: aws.String("i-drain"), Port: aws.Int32(80)},
						TargetHealth: &elbv2types.TargetHealth{State: elbv2types.TargetHealthStateEnumDraining},
					},
				},
			},
		},
	}
	result, err := checker.CheckALB(context.Background(), "arn:aws:elasticloadbalancing:ap-northeast-1:123:targetgroup/test/abc")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.AllHealthy {
		t.Error("expected AllHealthy=false for draining target")
	}
}

func TestCheckALB_InitialTarget(t *testing.T) {
	// initial 状態（登録直後）のターゲットは AllHealthy=false
	checker := &Checker{
		elbv2: &mockELBV2Client{
			output: &elasticloadbalancingv2.DescribeTargetHealthOutput{
				TargetHealthDescriptions: []elbv2types.TargetHealthDescription{
					{
						Target:       &elbv2types.TargetDescription{Id: aws.String("i-init"), Port: aws.Int32(80)},
						TargetHealth: &elbv2types.TargetHealth{State: elbv2types.TargetHealthStateEnumInitial},
					},
				},
			},
		},
	}
	result, err := checker.CheckALB(context.Background(), "arn:aws:elasticloadbalancing:ap-northeast-1:123:targetgroup/test/abc")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.AllHealthy {
		t.Error("expected AllHealthy=false for initial target")
	}
}

func TestCheckALB_ThreeTargets_AllHealthy(t *testing.T) {
	// 3ターゲット全員 healthy → AllHealthy=true、件数も一致
	checker := &Checker{
		elbv2: &mockELBV2Client{
			output: &elasticloadbalancingv2.DescribeTargetHealthOutput{
				TargetHealthDescriptions: []elbv2types.TargetHealthDescription{
					{Target: &elbv2types.TargetDescription{Id: aws.String("i-1"), Port: aws.Int32(80)}, TargetHealth: &elbv2types.TargetHealth{State: elbv2types.TargetHealthStateEnumHealthy}},
					{Target: &elbv2types.TargetDescription{Id: aws.String("i-2"), Port: aws.Int32(80)}, TargetHealth: &elbv2types.TargetHealth{State: elbv2types.TargetHealthStateEnumHealthy}},
					{Target: &elbv2types.TargetDescription{Id: aws.String("i-3"), Port: aws.Int32(80)}, TargetHealth: &elbv2types.TargetHealth{State: elbv2types.TargetHealthStateEnumHealthy}},
				},
			},
		},
	}
	result, err := checker.CheckALB(context.Background(), "arn:aws:elasticloadbalancing:ap-northeast-1:123:targetgroup/test/abc")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.AllHealthy {
		t.Error("expected AllHealthy=true when all 3 targets are healthy")
	}
	if len(result.Targets) != 3 {
		t.Errorf("expected 3 targets, got %d", len(result.Targets))
	}
}

func TestCheckALB_TargetGroupARNPreserved(t *testing.T) {
	// 入力した TargetGroupARN が result に保存されること
	const arn = "arn:aws:elasticloadbalancing:ap-northeast-1:999:targetgroup/my-tg/xyz"
	checker := &Checker{
		elbv2: &mockELBV2Client{
			output: &elasticloadbalancingv2.DescribeTargetHealthOutput{
				TargetHealthDescriptions: []elbv2types.TargetHealthDescription{},
			},
		},
	}
	result, err := checker.CheckALB(context.Background(), arn)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.TargetGroupARN != arn {
		t.Errorf("expected TargetGroupARN=%s, got %s", arn, result.TargetGroupARN)
	}
}

// ── CheckEC2 impaired / 2台全正常 / InstanceID保存 ────────────────────────

func TestCheckEC2_ImpairedSystemStatus(t *testing.T) {
	// SystemStatus が impaired のとき AllHealthy=false
	checker := &Checker{
		ec2cli: &mockEC2Client{
			output: &ec2.DescribeInstanceStatusOutput{
				InstanceStatuses: []ec2types.InstanceStatus{
					{
						InstanceId:     aws.String("i-impaired"),
						InstanceState:  &ec2types.InstanceState{Name: ec2types.InstanceStateNameRunning},
						SystemStatus:   &ec2types.InstanceStatusSummary{Status: ec2types.SummaryStatusImpaired},
						InstanceStatus: &ec2types.InstanceStatusSummary{Status: ec2types.SummaryStatusOk},
					},
				},
			},
		},
	}
	result, err := checker.CheckEC2(context.Background(), []string{"i-impaired"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.AllHealthy {
		t.Error("expected AllHealthy=false when system status is impaired")
	}
}

func TestCheckEC2_AllRunning_TwoInstances(t *testing.T) {
	// 2台ともRunning+ok → AllHealthy=true, Instances len=2
	checker := &Checker{
		ec2cli: &mockEC2Client{
			output: &ec2.DescribeInstanceStatusOutput{
				InstanceStatuses: []ec2types.InstanceStatus{
					{
						InstanceId:     aws.String("i-001"),
						InstanceState:  &ec2types.InstanceState{Name: ec2types.InstanceStateNameRunning},
						SystemStatus:   &ec2types.InstanceStatusSummary{Status: ec2types.SummaryStatusOk},
						InstanceStatus: &ec2types.InstanceStatusSummary{Status: ec2types.SummaryStatusOk},
					},
					{
						InstanceId:     aws.String("i-002"),
						InstanceState:  &ec2types.InstanceState{Name: ec2types.InstanceStateNameRunning},
						SystemStatus:   &ec2types.InstanceStatusSummary{Status: ec2types.SummaryStatusOk},
						InstanceStatus: &ec2types.InstanceStatusSummary{Status: ec2types.SummaryStatusOk},
					},
				},
			},
		},
	}
	result, err := checker.CheckEC2(context.Background(), []string{"i-001", "i-002"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.AllHealthy {
		t.Error("expected AllHealthy=true when all instances are running")
	}
	if len(result.Instances) != 2 {
		t.Errorf("expected 2 instances, got %d", len(result.Instances))
	}
}

func TestCheckEC2_InstanceIDPreserved(t *testing.T) {
	// result.Instances[0].InstanceID が API 応答の InstanceId と一致すること
	checker := &Checker{
		ec2cli: &mockEC2Client{
			output: &ec2.DescribeInstanceStatusOutput{
				InstanceStatuses: []ec2types.InstanceStatus{
					{
						InstanceId:     aws.String("i-abc123"),
						InstanceState:  &ec2types.InstanceState{Name: ec2types.InstanceStateNameRunning},
						SystemStatus:   &ec2types.InstanceStatusSummary{Status: ec2types.SummaryStatusOk},
						InstanceStatus: &ec2types.InstanceStatusSummary{Status: ec2types.SummaryStatusOk},
					},
				},
			},
		},
	}
	result, err := checker.CheckEC2(context.Background(), []string{"i-abc123"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Instances) == 0 {
		t.Fatal("expected at least one instance in result")
	}
	if result.Instances[0].InstanceID != "i-abc123" {
		t.Errorf("expected InstanceID=i-abc123, got %s", result.Instances[0].InstanceID)
	}
}

// ── CheckRDS backing-up / table driven statuses ───────────────────────────

func TestCheckRDS_BackingUp(t *testing.T) {
	// backing-up 状態は Healthy=false
	checker := &Checker{
		rdsCli: &mockRDSClient{
			output: &rds.DescribeDBInstancesOutput{
				DBInstances: []rdstypes.DBInstance{
					{
						DBInstanceIdentifier: aws.String("webapp-dev-rds"),
						DBInstanceStatus:     aws.String("backing-up"),
					},
				},
			},
		},
	}
	result, err := checker.CheckRDS(context.Background(), "webapp-dev-rds")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Healthy {
		t.Error("expected Healthy=false for backing-up RDS")
	}
	if result.Status != "backing-up" {
		t.Errorf("expected status=backing-up, got %s", result.Status)
	}
}

func TestCheckRDS_TableDrivenStatuses(t *testing.T) {
	// available のみ Healthy=true、それ以外は false
	cases := []struct {
		status  string
		healthy bool
	}{
		{"available", true},
		{"stopped", false},
		{"modifying", false},
		{"backing-up", false},
		{"creating", false},
		{"deleting", false},
	}
	for _, tc := range cases {
		t.Run(tc.status, func(t *testing.T) {
			checker := &Checker{
				rdsCli: &mockRDSClient{
					output: &rds.DescribeDBInstancesOutput{
						DBInstances: []rdstypes.DBInstance{
							{
								DBInstanceIdentifier: aws.String("db"),
								DBInstanceStatus:     aws.String(tc.status),
							},
						},
					},
				},
			}
			result, err := checker.CheckRDS(context.Background(), "db")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result.Healthy != tc.healthy {
				t.Errorf("status=%q: expected Healthy=%v, got %v", tc.status, tc.healthy, result.Healthy)
			}
		})
	}
}
