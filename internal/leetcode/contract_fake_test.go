package leetcode_test

import (
	"testing"

	"leetcode-anki/internal/leetcode"
	"leetcode-anki/internal/leetcode/contracttest"
	"leetcode-anki/internal/leetcode/leetcodefake"
)

// TestContract_Fake runs the contract suite against a seeded
// leetcodefake.Fake. The seed values are chosen so every invariant holds;
// if a future contract assertion is tightened, this seed must be tightened
// to match. The fake-side test exists to keep the contract definition
// honest — it's the proof that the assertions are satisfiable in
// principle, separate from whether they hold against the live API.
func TestContract_Fake(t *testing.T) {
	fx := twoSumFixture()
	api := seedFake(fx)
	contracttest.ContractTest(t, api, fx)
}

func twoSumFixture() contracttest.Fixture {
	return contracttest.Fixture{
		KnownProblemSlug: "two-sum",
		KnownQuestionID:  "1",
		PassingSolution: contracttest.PassingSolution{
			Lang: "python3",
			Code: `class Solution:
    def twoSum(self, nums, target):
        seen = {}
        for i, n in enumerate(nums):
            if target-n in seen:
                return [seen[target-n], i]
            seen[n] = i
`,
			Input:    "[2,7,11,15]\n9",
			MetaData: `{"name":"twoSum","params":[{"name":"nums","type":"integer[]"},{"name":"target","type":"integer"}],"return":{"type":"integer[]"}}`,
		},
		NoteText: "leetcode-anki contract test marker",
	}
}

func seedFake(fx contracttest.Fixture) *leetcodefake.Fake {
	const fakeListSlug = "fake-favorites"
	return &leetcodefake.Fake{
		Lists: []leetcode.FavoriteList{
			{Slug: fakeListSlug, Name: "Favorite Questions", QuestionCount: 1},
		},
		Questions: map[string][]leetcode.Question{
			fakeListSlug: {
				{
					ID:                 1,
					QuestionFrontendID: "1",
					TitleSlug:          fx.KnownProblemSlug,
					Title:              "Two Sum",
					Difficulty:         "Easy",
				},
			},
		},
		Details: map[string]*leetcode.ProblemDetail{
			fx.KnownProblemSlug: {
				QuestionID:         fx.KnownQuestionID,
				QuestionFrontendID: "1",
				TitleSlug:          fx.KnownProblemSlug,
				Title:              "Two Sum",
				Difficulty:         "Easy",
				Content:            "<p>Find two numbers that sum to target.</p>",
				CodeSnippets: []leetcode.CodeSnippet{
					{Lang: "Python3", LangSlug: "python3", Code: "class Solution:\n    def twoSum(self, nums, target):\n        pass\n"},
				},
			},
		},
		RunResult: &leetcode.RunResult{
			State:      "SUCCESS",
			StatusCode: 10,
			Cases: []leetcode.RunCase{
				{Index: 0, Input: "[2,7,11,15]\n9", Output: "[0,1]", Expected: "[0,1]", Pass: true},
			},
		},
		SubmitResult: &leetcode.SubmitResult{State: "SUCCESS", StatusCode: 10, SubmissionID: "fake-1"},
	}
}
