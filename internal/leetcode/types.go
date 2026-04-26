package leetcode

type TopicTag struct {
	Name           string `json:"name"`
	NameTranslated string `json:"nameTranslated"`
	Slug           string `json:"slug"`
}

type Question struct {
	Difficulty         string     `json:"difficulty"`
	ID                 int        `json:"id"`
	PaidOnly           bool       `json:"paidOnly"`
	QuestionFrontendID string     `json:"questionFrontendId"`
	Status             *string    `json:"status"`
	Title              string     `json:"title"`
	TitleSlug          string     `json:"titleSlug"`
	TranslatedTitle    *string    `json:"translatedTitle"`
	IsInMyFavorites    bool       `json:"isInMyFavorites"`
	Frequency          float64    `json:"frequency"`
	AcRate             float64    `json:"acRate"`
	ContestPoint       *float64   `json:"contestPoint"`
	TopicTags          []TopicTag `json:"topicTags"`
}

// FavoriteList describes one of the user's lists (favorites / custom lists).
type FavoriteList struct {
	Slug             string `json:"slug"`
	Name             string `json:"name"`
	Description      string `json:"description"`
	QuestionCount    int    `json:"questionCount"`
	IsPublicFavorite bool   `json:"isPublicFavorite"`
}

// CodeSnippet is one starter snippet for a given language on a problem.
type CodeSnippet struct {
	Lang     string `json:"lang"`
	LangSlug string `json:"langSlug"`
	Code     string `json:"code"`
}

// ProblemDetail is the full content of a single problem (description + snippets).
type ProblemDetail struct {
	QuestionID         string        `json:"questionId"`
	QuestionFrontendID string        `json:"questionFrontendId"`
	Title              string        `json:"title"`
	TitleSlug          string        `json:"titleSlug"`
	Difficulty         string        `json:"difficulty"`
	IsPaidOnly         bool          `json:"isPaidOnly"`
	Content            string        `json:"content"`
	CodeSnippets       []CodeSnippet `json:"codeSnippets"`
	ExampleTestcases   string        `json:"exampleTestcases"`
	SampleTestCase     string        `json:"sampleTestCase"`
	MetaData           string        `json:"metaData"`
	Hints              []string      `json:"hints"`
	TopicTags          []TopicTag    `json:"topicTags"`
}

// RunResult is the verdict from `interpret_solution` after polling.
type RunResult struct {
	State              string   `json:"state"`
	StatusCode         int      `json:"status_code"`
	StatusMsg          string   `json:"status_msg"`
	StatusRuntime      string   `json:"status_runtime"`
	StatusMemory       string   `json:"status_memory"`
	Lang               string   `json:"lang"`
	RunSuccess         bool     `json:"run_success"`
	CorrectAnswer      bool     `json:"correct_answer"`
	CodeAnswer         []string `json:"code_answer"`
	ExpectedCodeAnswer []string `json:"expected_code_answer"`
	StdOutput          string   `json:"std_output"`
	CompileError       string   `json:"compile_error"`
	FullCompileError   string   `json:"full_compile_error"`
	RuntimeError       string   `json:"runtime_error"`
	FullRuntimeError   string   `json:"full_runtime_error"`
	LastTestcase       string   `json:"last_testcase"`
}

// SubmitResult is the verdict from `submit` after polling.
type SubmitResult struct {
	State              string  `json:"state"`
	StatusCode         int     `json:"status_code"`
	StatusMsg          string  `json:"status_msg"`
	StatusRuntime      string  `json:"status_runtime"`
	StatusMemory       string  `json:"status_memory"`
	Lang               string  `json:"lang"`
	RunSuccess         bool    `json:"run_success"`
	TotalCorrect       int     `json:"total_correct"`
	TotalTestcases     int     `json:"total_testcases"`
	RuntimePercentile  float64 `json:"runtime_percentile"`
	MemoryPercentile   float64 `json:"memory_percentile"`
	CompileError       string  `json:"compile_error"`
	FullCompileError   string  `json:"full_compile_error"`
	RuntimeError       string  `json:"runtime_error"`
	FullRuntimeError   string  `json:"full_runtime_error"`
	LastTestcase       string  `json:"last_testcase"`
	ExpectedOutput     string  `json:"expected_output"`
	CodeOutput         string  `json:"code_output"`
}
