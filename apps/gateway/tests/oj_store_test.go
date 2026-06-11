package tests

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/marsagent/gateway/internal/oj"
	"github.com/stretchr/testify/require"
)

func TestOJProblemLifecycle(t *testing.T) {
	db := openTestDB(t)
	store := oj.NewOJStore(db)
	ctx := context.Background()

	title := "A+B " + uuid.NewString()[:8]
	p := &oj.Problem{
		Title:         title,
		DescriptionMD: "## 求两数之和\n\n读入 a, b，输出 a+b。",
		Tags:          []string{"math", "easy"},
		Difficulty:    "easy",
		TimeLimitMs:   1000,
		MemoryLimitMb: 128,
	}
	require.NoError(t, store.CreateProblem(ctx, p))
	require.NotEmpty(t, p.ID)

	got, err := store.GetProblem(ctx, p.ID)
	require.NoError(t, err)
	require.Equal(t, title, got.Title)
	require.Equal(t, []string{"math", "easy"}, got.Tags)

	tc := &oj.TestCase{
		ProblemID:      p.ID,
		Input:          "1 2\n",
		ExpectedOutput: "3",
		IsSample:       true,
		IsHidden:       false,
		Score:          100,
		Ordering:       0,
	}
	require.NoError(t, store.AddTestCase(ctx, tc))
	require.NotEmpty(t, tc.ID)

	samples, err := store.ListTestCases(ctx, p.ID, false)
	require.NoError(t, err)
	require.Len(t, samples, 1)
	require.Equal(t, "3", samples[0].ExpectedOutput)

	allCases, err := store.ListTestCases(ctx, p.ID, true)
	require.NoError(t, err)
	require.Len(t, allCases, 1)

	problems, total, err := store.ListProblems(ctx, 20, 0, "easy", "math")
	require.NoError(t, err)
	require.GreaterOrEqual(t, total, 1)
	found := false
	for _, prob := range problems {
		if prob.ID == p.ID {
			found = true
			break
		}
	}
	require.True(t, found, "created problem should appear in list")
}

func TestOJSubmissionLifecycle(t *testing.T) {
	db := openTestDB(t)
	store := oj.NewOJStore(db)
	ctx := context.Background()

	p := &oj.Problem{Title: "Echo " + uuid.NewString()[:8], Difficulty: "easy"}
	require.NoError(t, store.CreateProblem(ctx, p))

	tc := &oj.TestCase{
		ProblemID: p.ID, Input: "", ExpectedOutput: "hi",
		IsSample: true, IsHidden: false, Score: 100, Ordering: 0,
	}
	require.NoError(t, store.AddTestCase(ctx, tc))

	sub := &oj.Submission{ProblemID: p.ID, Code: `print("hi")`, Lang: "python"}
	require.NoError(t, store.CreateSubmission(ctx, sub))
	require.Equal(t, "pending", sub.Status)

	got, err := store.GetSubmission(ctx, sub.ID)
	require.NoError(t, err)
	require.Equal(t, p.ID, got.ProblemID)

	require.NoError(t, store.UpdateSubmission(ctx, sub.ID, oj.StatusAccepted, 100, 50, 0, ""))

	subs, total, err := store.ListSubmissions(ctx, p.ID, "", 10, 0)
	require.NoError(t, err)
	require.GreaterOrEqual(t, total, 1)

	result := &oj.SubmissionResult{
		SubmissionID: sub.ID,
		TestCaseID:   tc.ID,
		Status:       oj.StatusAccepted,
		ActualOutput: "hi",
		DurationMs:   50,
		Score:        100,
	}
	require.NoError(t, store.SaveSubmissionResult(ctx, result))

	results, err := store.GetSubmissionResults(ctx, sub.ID)
	require.NoError(t, err)
	require.Len(t, results, 1)
	require.Equal(t, oj.StatusAccepted, results[0].Status)

	_ = subs
}
