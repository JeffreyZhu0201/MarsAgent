// Package oj 的判题引擎：在 Docker 沙箱中运行提交代码，逐条比对测试用例。
package oj

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/marsagent/gateway/internal/sandbox"
)

// 判题结果状态常量。
const (
	StatusPending     = "pending"
	StatusJudging     = "judging"
	StatusAccepted    = "accepted"
	StatusWrongAnswer = "wrong_answer"
	StatusTLE         = "tle" // 超时
	StatusMLE         = "mle" // 内存超限（预留）
	StatusRE          = "re"  // 运行时错误
	StatusCE          = "ce"  // 编译错误（预留）
)

// JudgeEngine 在 Docker 沙箱中执行代码并比对输出。
type JudgeEngine struct {
	store   *OJStore
	sandbox *sandbox.Scheduler
}

// NewJudgeEngine 创建判题引擎。
func NewJudgeEngine(store *OJStore, sch *sandbox.Scheduler) *JudgeEngine {
	return &JudgeEngine{store: store, sandbox: sch}
}

// JudgeSubmission 对一次提交运行全部测试用例，并将结果写入数据库。
func (e *JudgeEngine) JudgeSubmission(ctx context.Context, submissionID string) error {
	sub, err := e.store.GetSubmission(ctx, submissionID)
	if err != nil {
		return fmt.Errorf("get submission: %w", err)
	}
	problem, err := e.store.GetProblem(ctx, sub.ProblemID)
	if err != nil {
		return fmt.Errorf("get problem: %w", err)
	}
	testCases, err := e.store.ListTestCases(ctx, sub.ProblemID, true)
	if err != nil {
		return fmt.Errorf("list test cases: %w", err)
	}

	// 标记为判题中
	_ = e.store.UpdateSubmission(ctx, submissionID, StatusJudging, 0, 0, 0, "")

	var totalScore, totalDuration, maxMemory int
	var finalStatus = StatusAccepted
	results := make([]SubmissionResult, 0, len(testCases))

	for _, tc := range testCases {
		result := e.judgeOne(ctx, sub, &tc, problem)
		results = append(results, result)
		totalScore += result.Score
		totalDuration += result.DurationMs
		if result.MemoryKb > maxMemory {
			maxMemory = result.MemoryKb
		}
		if result.Status != StatusAccepted && finalStatus == StatusAccepted {
			finalStatus = result.Status
		}
		if err := e.store.SaveSubmissionResult(ctx, &result); err != nil {
			return fmt.Errorf("save result: %w", err)
		}
	}

	if err := e.store.UpdateSubmission(ctx, submissionID, finalStatus, totalScore, totalDuration, maxMemory, ""); err != nil {
		return fmt.Errorf("update submission: %w", err)
	}
	return nil
}

// judgeOne 运行单个测试用例并返回比对结果。
func (e *JudgeEngine) judgeOne(_ context.Context, sub *Submission, tc *TestCase, problem *Problem) SubmissionResult {
	result := SubmissionResult{
		SubmissionID: sub.ID,
		TestCaseID:   tc.ID,
		Status:       StatusAccepted,
		Score:        tc.Score,
	}

	timeoutSec := problem.TimeLimitMs / 1000
	if timeoutSec == 0 {
		timeoutSec = 1
	}

	runResult, err := e.sandbox.Run(sandbox.RunRequest{
		Lang:    sub.Lang,
		Code:    sub.Code,
		Stdin:   tc.Input,
		Timeout: timeoutSec,
	})

	// 沙箱异常（容器崩溃等）
	if err != nil {
		result.Status = StatusRE
		result.Score = 0
		result.ActualOutput = err.Error()
		return result
	}

	result.DurationMs = int(runResult.Duration)

	// 非零退出码视为运行时错误
	if runResult.ExitCode != 0 {
		result.Status = StatusRE
		result.Score = 0
		result.ActualOutput = runResult.Stderr
		return result
	}

	// 超时：stderr 含 "timeout" 或耗时超过题目限制
	if strings.Contains(runResult.Stderr, "timeout") || runResult.Duration > int64(problem.TimeLimitMs) {
		result.Status = StatusTLE
		result.Score = 0
		return result
	}

	// 比对输出（支持浮点容差）
	actual := NormalizeOutput(runResult.Stdout)
	expected := NormalizeOutput(tc.ExpectedOutput)
	if !CompareOutput(expected, actual, 1e-6) {
		result.Status = StatusWrongAnswer
		result.Score = 0
		result.ActualOutput = actual
		return result
	}

	result.Status = StatusAccepted
	result.ActualOutput = actual
	return result
}

// JudgeSubmissionAsync 在后台 goroutine 中异步判题。
func (e *JudgeEngine) JudgeSubmissionAsync(ctx context.Context, submissionID string) {
	go func() {
		bg := context.WithoutCancel(ctx)
		if err := e.JudgeSubmission(bg, submissionID); err != nil {
			fmt.Printf("[OJ] judge error for %s: %v\n", submissionID, err)
		}
	}()
}

// CompareOutput 比较两段输出：先精确匹配，再按 token 做浮点容差比对。
// expected 与 actual 应已 NormalizeOutput；传入原始字符串亦可。
func CompareOutput(expected, actual string, tol float64) bool {
	if expected == actual {
		return true
	}
	expTokens := strings.Fields(expected)
	actTokens := strings.Fields(actual)
	if len(expTokens) != len(actTokens) {
		return false
	}
	for i := range expTokens {
		expNum, err1 := strconv.ParseFloat(expTokens[i], 64)
		actNum, err2 := strconv.ParseFloat(actTokens[i], 64)
		if err1 != nil || err2 != nil {
			return false
		}
		if math.Abs(expNum-actNum) > tol {
			return false
		}
	}
	return true
}
