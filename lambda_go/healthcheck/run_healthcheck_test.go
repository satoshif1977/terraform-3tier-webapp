package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	elbv2types "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"
)

// ── テスト用ヘルパー ──────────────────────────────────────────────────────────

// runTestHarness は runHealthCheck をメモリ上の出力先で実行するための一式。
type runTestHarness struct {
	checker *Checker
	logger  *slog.Logger
	metrics *Metrics
	logBuf  *bytes.Buffer
	metBuf  *bytes.Buffer
}

// newRunTestHarness はログとメトリクスをバッファに書き出す実行環境を作る。
func newRunTestHarness(t *testing.T, checker *Checker) *runTestHarness {
	t.Helper()
	logBuf := &bytes.Buffer{}
	metBuf := &bytes.Buffer{}
	metrics, err := NewMetrics(MetricsOptions{Namespace: "Test/HealthCheck", Sink: metBuf})
	if err != nil {
		t.Fatalf("NewMetrics が失敗しました: %v", err)
	}
	return &runTestHarness{
		checker: checker,
		logger:  NewLogger(LoggerOptions{Level: LogLevelDebug, Writer: logBuf}),
		metrics: metrics,
		logBuf:  logBuf,
		metBuf:  metBuf,
	}
}

// metricValues は Flush 後の EMF ドキュメントからメトリクス名と値の対応を取り出す。
func (h *runTestHarness) metricValues(t *testing.T) map[string]float64 {
	t.Helper()
	if err := h.metrics.Flush(); err != nil {
		t.Fatalf("Flush が失敗しました: %v", err)
	}
	values := map[string]float64{}
	for _, line := range strings.Split(strings.TrimSpace(h.metBuf.String()), "\n") {
		if line == "" {
			continue
		}
		var doc map[string]any
		if err := json.Unmarshal([]byte(line), &doc); err != nil {
			t.Fatalf("EMF ドキュメントの解析に失敗しました: %v\n%s", err, line)
		}
		for k, v := range doc {
			if num, ok := v.(float64); ok {
				values[k] = num
			}
		}
	}
	return values
}

// logMessages はロガーが書き出した各行の message を返す。
func (h *runTestHarness) logMessages(t *testing.T) []string {
	t.Helper()
	var msgs []string
	for _, line := range strings.Split(strings.TrimSpace(h.logBuf.String()), "\n") {
		if line == "" {
			continue
		}
		var rec map[string]any
		if err := json.Unmarshal([]byte(line), &rec); err != nil {
			t.Fatalf("ログ行の解析に失敗しました: %v\n%s", err, line)
		}
		if msg, ok := rec["message"].(string); ok {
			msgs = append(msgs, msg)
		}
	}
	return msgs
}

func healthyCheckerFixture() *Checker {
	return &Checker{
		elbv2: &mockELBV2Client{output: &elasticloadbalancingv2.DescribeTargetHealthOutput{
			TargetHealthDescriptions: []elbv2types.TargetHealthDescription{
				{
					Target:       &elbv2types.TargetDescription{Id: aws.String("i-aaa"), Port: aws.Int32(80)},
					TargetHealth: &elbv2types.TargetHealth{State: elbv2types.TargetHealthStateEnumHealthy},
				},
			},
		}},
		ec2cli: &mockEC2Client{output: &ec2.DescribeInstanceStatusOutput{
			InstanceStatuses: []ec2types.InstanceStatus{
				{
					InstanceId:     aws.String("i-aaa"),
					InstanceState:  &ec2types.InstanceState{Name: ec2types.InstanceStateNameRunning},
					SystemStatus:   &ec2types.InstanceStatusSummary{Status: ec2types.SummaryStatusOk},
					InstanceStatus: &ec2types.InstanceStatusSummary{Status: ec2types.SummaryStatusOk},
				},
			},
		}},
		rdsCli: &mockRDSClient{output: &rds.DescribeDBInstancesOutput{
			DBInstances: []rdstypes.DBInstance{
				{DBInstanceIdentifier: aws.String("db-1"), DBInstanceStatus: aws.String("available")},
			},
		}},
	}
}

func fullConfigFixture() checkConfig {
	return checkConfig{
		TargetGroupARN: "arn:aws:elasticloadbalancing:ap-northeast-1:123456789012:targetgroup/tg/abc",
		InstanceIDs:    []string{"i-aaa"},
		DBIdentifier:   "db-1",
	}
}

// ── UnhealthyCount ────────────────────────────────────────────────────────────

func TestALBHealthUnhealthyCount(t *testing.T) {
	tests := []struct {
		name   string
		states []elbv2types.TargetHealthStateEnum
		want   int
	}{
		{"全て healthy", []elbv2types.TargetHealthStateEnum{
			elbv2types.TargetHealthStateEnumHealthy,
			elbv2types.TargetHealthStateEnumHealthy,
		}, 0},
		{"1台 unhealthy", []elbv2types.TargetHealthStateEnum{
			elbv2types.TargetHealthStateEnumHealthy,
			elbv2types.TargetHealthStateEnumUnhealthy,
		}, 1},
		{"draining と initial も数える", []elbv2types.TargetHealthStateEnum{
			elbv2types.TargetHealthStateEnumDraining,
			elbv2types.TargetHealthStateEnumInitial,
		}, 2},
		{"ターゲットなし", nil, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := &ALBHealth{}
			for _, s := range tt.states {
				h.Targets = append(h.Targets, TargetStatus{State: string(s)})
			}
			if got := h.UnhealthyCount(); got != tt.want {
				t.Errorf("UnhealthyCount() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestEC2HealthUnhealthyCount(t *testing.T) {
	ok := string(ec2types.SummaryStatusOk)
	running := string(ec2types.InstanceStateNameRunning)

	tests := []struct {
		name      string
		instances []InstanceStatus
		want      int
	}{
		{"全て正常", []InstanceStatus{
			{InstanceState: running, SystemStatus: ok, InstanceCheck: ok},
		}, 0},
		{"停止中は異常", []InstanceStatus{
			{InstanceState: string(ec2types.InstanceStateNameStopped), SystemStatus: ok, InstanceCheck: ok},
		}, 1},
		{"システムチェック失敗は異常", []InstanceStatus{
			{InstanceState: running, SystemStatus: string(ec2types.SummaryStatusImpaired), InstanceCheck: ok},
		}, 1},
		{"インスタンスチェック失敗は異常", []InstanceStatus{
			{InstanceState: running, SystemStatus: ok, InstanceCheck: string(ec2types.SummaryStatusImpaired)},
		}, 1},
		{"正常1台・異常2台", []InstanceStatus{
			{InstanceState: running, SystemStatus: ok, InstanceCheck: ok},
			{InstanceState: string(ec2types.InstanceStateNameStopped), SystemStatus: ok, InstanceCheck: ok},
			{InstanceState: running, SystemStatus: string(ec2types.SummaryStatusImpaired), InstanceCheck: ok},
		}, 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := &EC2Health{Instances: tt.instances}
			if got := h.UnhealthyCount(); got != tt.want {
				t.Errorf("UnhealthyCount() = %d, want %d", got, tt.want)
			}
		})
	}
}

// ── loadCheckConfig ───────────────────────────────────────────────────────────

func TestLoadCheckConfig(t *testing.T) {
	tests := []struct {
		name    string
		env     map[string]string
		wantIDs []string
		wantARN string
		wantDB  string
	}{
		{"未設定なら空", map[string]string{}, nil, "", ""},
		{"3つとも設定", map[string]string{
			"ALB_TARGET_GROUP_ARN": "arn:tg",
			"EC2_INSTANCE_IDS":     "i-a,i-b",
			"RDS_DB_IDENTIFIER":    "db-1",
		}, []string{"i-a", "i-b"}, "arn:tg", "db-1"},
		{"空要素と空白は捨てる", map[string]string{
			"EC2_INSTANCE_IDS": " i-a , , i-b ,",
		}, []string{"i-a", "i-b"}, "", ""},
		{"ARN と識別子も trim する", map[string]string{
			"ALB_TARGET_GROUP_ARN": "  arn:tg  ",
			"RDS_DB_IDENTIFIER":    "  db-1  ",
		}, nil, "arn:tg", "db-1"},
		{"カンマのみなら空", map[string]string{
			"EC2_INSTANCE_IDS": ",,,",
		}, nil, "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, k := range []string{"ALB_TARGET_GROUP_ARN", "EC2_INSTANCE_IDS", "RDS_DB_IDENTIFIER"} {
				t.Setenv(k, tt.env[k])
			}
			got := loadCheckConfig()
			if got.TargetGroupARN != tt.wantARN {
				t.Errorf("TargetGroupARN = %q, want %q", got.TargetGroupARN, tt.wantARN)
			}
			if got.DBIdentifier != tt.wantDB {
				t.Errorf("DBIdentifier = %q, want %q", got.DBIdentifier, tt.wantDB)
			}
			if len(got.InstanceIDs) != len(tt.wantIDs) {
				t.Fatalf("InstanceIDs = %v, want %v", got.InstanceIDs, tt.wantIDs)
			}
			for i, id := range tt.wantIDs {
				if got.InstanceIDs[i] != id {
					t.Errorf("InstanceIDs[%d] = %q, want %q", i, got.InstanceIDs[i], id)
				}
			}
		})
	}
}

// ── boolMetric / recordCount ──────────────────────────────────────────────────

func TestBoolMetric(t *testing.T) {
	if got := boolMetric(true); got != 1 {
		t.Errorf("boolMetric(true) = %v, want 1", got)
	}
	if got := boolMetric(false); got != 0 {
		t.Errorf("boolMetric(false) = %v, want 0", got)
	}
}

func TestRecordCountWithNilMetricsDoesNotPanic(t *testing.T) {
	logBuf := &bytes.Buffer{}
	logger := NewLogger(LoggerOptions{Writer: logBuf})
	recordCount(logger, nil, "Anything", 1)
	if logBuf.Len() != 0 {
		t.Errorf("メトリクス未設定時はログを出さない想定だが出力された: %s", logBuf.String())
	}
}

func TestRecordCountLogsWarningOnInvalidName(t *testing.T) {
	logBuf := &bytes.Buffer{}
	metBuf := &bytes.Buffer{}
	logger := NewLogger(LoggerOptions{Writer: logBuf})
	metrics, err := NewMetrics(MetricsOptions{Namespace: "Test/HealthCheck", Sink: metBuf})
	if err != nil {
		t.Fatalf("NewMetrics が失敗しました: %v", err)
	}
	recordCount(logger, metrics, "", 1)
	if !strings.Contains(logBuf.String(), "メトリクスの記録に失敗しました") {
		t.Errorf("警告ログが出ていません: %s", logBuf.String())
	}
}

// ── runHealthCheck ────────────────────────────────────────────────────────────

func TestRunHealthCheckAllHealthy(t *testing.T) {
	h := newRunTestHarness(t, healthyCheckerFixture())
	summary := runHealthCheck(context.Background(), h.checker, h.logger, h.metrics, fullConfigFixture())

	if !summary.Healthy {
		t.Error("Healthy = false, want true")
	}
	if summary.ALB == nil || summary.EC2 == nil || summary.RDS == nil {
		t.Fatal("ALB / EC2 / RDS のいずれかが nil です")
	}

	values := h.metricValues(t)
	for name, want := range map[string]float64{
		"ALBHealthy":            1,
		"EC2Healthy":            1,
		"RDSHealthy":            1,
		"OverallHealthy":        1,
		"CheckErrors":           0,
		"ALBUnhealthyTargets":   0,
		"EC2UnhealthyInstances": 0,
	} {
		if got, ok := values[name]; !ok {
			t.Errorf("メトリクス %q が出力されていません", name)
		} else if got != want {
			t.Errorf("%s = %v, want %v", name, got, want)
		}
	}
}

func TestRunHealthCheckUnhealthyTargetsAreCounted(t *testing.T) {
	checker := healthyCheckerFixture()
	checker.elbv2 = &mockELBV2Client{output: &elasticloadbalancingv2.DescribeTargetHealthOutput{
		TargetHealthDescriptions: []elbv2types.TargetHealthDescription{
			{
				Target:       &elbv2types.TargetDescription{Id: aws.String("i-aaa"), Port: aws.Int32(80)},
				TargetHealth: &elbv2types.TargetHealth{State: elbv2types.TargetHealthStateEnumHealthy},
			},
			{
				Target:       &elbv2types.TargetDescription{Id: aws.String("i-bbb"), Port: aws.Int32(80)},
				TargetHealth: &elbv2types.TargetHealth{State: elbv2types.TargetHealthStateEnumUnhealthy},
			},
		},
	}}

	h := newRunTestHarness(t, checker)
	summary := runHealthCheck(context.Background(), h.checker, h.logger, h.metrics, fullConfigFixture())

	if summary.Healthy {
		t.Error("Healthy = true, want false")
	}
	values := h.metricValues(t)
	if values["ALBHealthy"] != 0 {
		t.Errorf("ALBHealthy = %v, want 0", values["ALBHealthy"])
	}
	if values["ALBUnhealthyTargets"] != 1 {
		t.Errorf("ALBUnhealthyTargets = %v, want 1", values["ALBUnhealthyTargets"])
	}
	if values["OverallHealthy"] != 0 {
		t.Errorf("OverallHealthy = %v, want 0", values["OverallHealthy"])
	}
}

func TestRunHealthCheckCountsEachCheckError(t *testing.T) {
	checker := &Checker{
		elbv2:  &mockELBV2Client{err: errors.New("ALB API エラー")},
		ec2cli: &mockEC2Client{err: errors.New("EC2 API エラー")},
		rdsCli: &mockRDSClient{err: errors.New("RDS API エラー")},
	}
	h := newRunTestHarness(t, checker)
	summary := runHealthCheck(context.Background(), h.checker, h.logger, h.metrics, fullConfigFixture())

	if summary.Healthy {
		t.Error("Healthy = true, want false")
	}
	if summary.ALB != nil || summary.EC2 != nil || summary.RDS != nil {
		t.Error("エラー時は各フィールドを埋めない想定です")
	}
	if got := h.metricValues(t)["CheckErrors"]; got != 3 {
		t.Errorf("CheckErrors = %v, want 3", got)
	}
}

func TestRunHealthCheckContinuesAfterFirstError(t *testing.T) {
	checker := healthyCheckerFixture()
	checker.elbv2 = &mockELBV2Client{err: errors.New("ALB API エラー")}

	h := newRunTestHarness(t, checker)
	summary := runHealthCheck(context.Background(), h.checker, h.logger, h.metrics, fullConfigFixture())

	if summary.ALB != nil {
		t.Error("ALB はエラーなので nil のままであるべきです")
	}
	if summary.EC2 == nil || summary.RDS == nil {
		t.Fatal("ALB が失敗しても EC2 / RDS は実行されるべきです")
	}
	values := h.metricValues(t)
	if values["CheckErrors"] != 1 {
		t.Errorf("CheckErrors = %v, want 1", values["CheckErrors"])
	}
	if values["EC2Healthy"] != 1 || values["RDSHealthy"] != 1 {
		t.Errorf("後続のメトリクスが記録されていません: %v", values)
	}
}

func TestRunHealthCheckSkipsUnconfiguredResources(t *testing.T) {
	h := newRunTestHarness(t, healthyCheckerFixture())
	summary := runHealthCheck(context.Background(), h.checker, h.logger, h.metrics, checkConfig{})

	if !summary.Healthy {
		t.Error("対象が無いときは Healthy = true のままであるべきです")
	}
	if summary.ALB != nil || summary.EC2 != nil || summary.RDS != nil {
		t.Error("未設定のリソースは nil のままであるべきです")
	}

	values := h.metricValues(t)
	for _, name := range []string{"ALBHealthy", "EC2Healthy", "RDSHealthy"} {
		if _, ok := values[name]; ok {
			t.Errorf("未設定なのに %q が記録されています", name)
		}
	}
	if values["OverallHealthy"] != 1 {
		t.Errorf("OverallHealthy = %v, want 1", values["OverallHealthy"])
	}
}

func TestRunHealthCheckRDSUnavailable(t *testing.T) {
	checker := healthyCheckerFixture()
	checker.rdsCli = &mockRDSClient{output: &rds.DescribeDBInstancesOutput{
		DBInstances: []rdstypes.DBInstance{
			{DBInstanceIdentifier: aws.String("db-1"), DBInstanceStatus: aws.String("stopped")},
		},
	}}

	h := newRunTestHarness(t, checker)
	summary := runHealthCheck(context.Background(), h.checker, h.logger, h.metrics, fullConfigFixture())

	if summary.Healthy {
		t.Error("Healthy = true, want false")
	}
	if got := h.metricValues(t)["RDSHealthy"]; got != 0 {
		t.Errorf("RDSHealthy = %v, want 0", got)
	}
}

func TestRunHealthCheckLogsWarningWhenUnhealthy(t *testing.T) {
	checker := healthyCheckerFixture()
	checker.rdsCli = &mockRDSClient{err: errors.New("RDS API エラー")}

	h := newRunTestHarness(t, checker)
	runHealthCheck(context.Background(), h.checker, h.logger, h.metrics, fullConfigFixture())

	msgs := strings.Join(h.logMessages(t), "\n")
	if !strings.Contains(msgs, "RDS のヘルスチェックに失敗しました") {
		t.Errorf("エラーログが出ていません:\n%s", msgs)
	}
	if !strings.Contains(msgs, "ヘルスチェックで異常を検知しました") {
		t.Errorf("異常検知の警告ログが出ていません:\n%s", msgs)
	}
}

func TestRunHealthCheckWorksWithoutMetrics(t *testing.T) {
	logBuf := &bytes.Buffer{}
	logger := NewLogger(LoggerOptions{Writer: logBuf})

	summary := runHealthCheck(
		context.Background(), healthyCheckerFixture(), logger, nil, fullConfigFixture())

	if summary == nil || !summary.Healthy {
		t.Fatal("メトリクスが nil でもヘルスチェックは完走すべきです")
	}
}
