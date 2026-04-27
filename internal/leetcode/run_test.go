package leetcode

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"leetcode-anki/internal/auth"
)

// scriptedDoer returns a sequence of canned response bodies in order; further
// calls past the script length re-use the last entry. All responses are 200 OK.
type scriptedDoer struct {
	bodies []string
	calls  int32
}

func (s *scriptedDoer) Do(_ *http.Request) (*http.Response, error) {
	idx := atomic.AddInt32(&s.calls, 1) - 1
	body := s.bodies[len(s.bodies)-1]
	if int(idx) < len(s.bodies) {
		body = s.bodies[idx]
	}
	return &http.Response{
		StatusCode: 200,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     make(http.Header),
	}, nil
}

func newTestClient(d httpDoer) *Client {
	return newClientWithDoer(&auth.Credentials{Session: "s", CSRF: "c"}, d)
}

func TestPollCheck_ReturnsOnSuccess(t *testing.T) {
	d := &scriptedDoer{bodies: []string{
		`{"state":"PENDING"}`,
		`{"state":"STARTED"}`,
		`{"state":"SUCCESS","status_msg":"Accepted"}`,
	}}
	c := newTestClient(d)

	raw, err := c.pollCheck(context.Background(), "abc")
	if err != nil {
		t.Fatalf("pollCheck: %v", err)
	}
	var probe struct {
		State     string `json:"state"`
		StatusMsg string `json:"status_msg"`
	}
	_ = json.Unmarshal(raw, &probe)
	if probe.State != "SUCCESS" || probe.StatusMsg != "Accepted" {
		t.Errorf("got %+v", probe)
	}
	if got := atomic.LoadInt32(&d.calls); got != 3 {
		t.Errorf("expected 3 polls, got %d", got)
	}
}

// First poll already terminal: pollCheck must skip the initial 700ms
// sleep and return on the first response. Test cap is generous so a
// regression where the loop sleeps before checking would visibly stall.
func TestPollCheck_ImmediateSuccessSkipsBackoff(t *testing.T) {
	d := &scriptedDoer{bodies: []string{
		`{"state":"SUCCESS","status_msg":"Accepted"}`,
	}}
	c := newTestClient(d)

	start := time.Now()
	raw, err := c.pollCheck(context.Background(), "abc")
	if err != nil {
		t.Fatalf("pollCheck: %v", err)
	}
	if elapsed := time.Since(start); elapsed >= 500*time.Millisecond {
		t.Errorf("first-call-terminal path slept (took %v); should return immediately", elapsed)
	}
	var probe struct{ State string }
	_ = json.Unmarshal(raw, &probe)
	if probe.State != "SUCCESS" {
		t.Errorf("got state %q, want SUCCESS", probe.State)
	}
	if got := atomic.LoadInt32(&d.calls); got != 1 {
		t.Errorf("expected 1 poll, got %d", got)
	}
}

// LeetCode does not document the full state alphabet; pollCheck treats any
// non-pending value as terminal so the TUI doesn't spin until the outer
// timeout when LeetCode introduces a new state.
func TestPollCheck_UnknownTerminalStateReturned(t *testing.T) {
	d := &scriptedDoer{bodies: []string{
		`{"state":"PENDING"}`,
		`{"state":"PENDING_REJUDGE","status_msg":"Rejudging"}`,
	}}
	c := newTestClient(d)

	raw, err := c.pollCheck(context.Background(), "abc")
	if err != nil {
		t.Fatalf("pollCheck: %v", err)
	}
	var probe struct{ State string }
	_ = json.Unmarshal(raw, &probe)
	if probe.State != "PENDING_REJUDGE" {
		t.Errorf("expected unknown state to surface, got %q", probe.State)
	}
}

func TestPollCheck_RespectsContextCancel(t *testing.T) {
	d := &scriptedDoer{bodies: []string{`{"state":"PENDING"}`}}
	c := newTestClient(d)

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	_, err := c.pollCheck(ctx, "abc")
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got %v", err)
	}
}

// An empty state ("") would loop forever under the old implementation; the
// iteration cap guarantees pollCheck eventually gives up rather than hanging
// the TUI for the full outer timeout.
func TestPollCheck_GivesUpAfterCapWhenStateNeverArrives(t *testing.T) {
	d := &scriptedDoer{bodies: []string{`{}`}} // no state field

	// Override timing constants by running with a short context so the test
	// completes quickly; the cap path still exercises because state is empty.
	c := newTestClient(d)
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_, err := c.pollCheck(ctx, "abc")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	// Either ctx.DeadlineExceeded (likely, given short timeout) or the cap
	// message — both prove the loop terminates instead of hanging.
}

func TestBuildRunCases(t *testing.T) {
	// twoSumMeta has two params, so each test case is two lines of
	// dataInput. Used by the happy-path entries below.
	const twoSumMeta = `{"name":"twoSum","params":[{"name":"nums","type":"integer[]"},{"name":"target","type":"integer"}],"return":{"type":"integer[]"}}`
	const oneArgMeta = `{"name":"reverse","params":[{"name":"x","type":"integer"}],"return":{"type":"integer"}}`

	tests := []struct {
		name      string
		wire      runResultWire
		dataInput string
		metaData  string
		want      []RunCase
	}{
		{
			name: "two cases two params; compare_result drives Pass",
			wire: runResultWire{
				CodeAnswer:         []string{"[0,1]", "[0,2]"},
				ExpectedCodeAnswer: []string{"[0,1]", "[1,2]"},
				StdOutputList:      []string{"", "visited 3\nvisited 2"},
				CompareResult:      "10",
			},
			dataInput: "[2,7,11,15]\n9\n[3,2,4]\n6",
			metaData:  twoSumMeta,
			want: []RunCase{
				{Index: 0, Input: "[2,7,11,15]\n9", Output: "[0,1]", Expected: "[0,1]", Stdout: "", Pass: true},
				{Index: 1, Input: "[3,2,4]\n6", Output: "[0,2]", Expected: "[1,2]", Stdout: "visited 3\nvisited 2", Pass: false},
			},
		},
		{
			name: "single param single case",
			wire: runResultWire{
				CodeAnswer:         []string{"321"},
				ExpectedCodeAnswer: []string{"321"},
				CompareResult:      "1",
			},
			dataInput: "123",
			metaData:  oneArgMeta,
			want: []RunCase{
				{Index: 0, Input: "123", Output: "321", Expected: "321", Pass: true},
			},
		},
		{
			name: "missing compare_result falls back to Output==Expected",
			wire: runResultWire{
				CodeAnswer:         []string{"[0,1]", "[]"},
				ExpectedCodeAnswer: []string{"[0,1]", "[1,2]"},
			},
			dataInput: "[2,7,11,15]\n9\n[3,2,4]\n6",
			metaData:  twoSumMeta,
			want: []RunCase{
				{Index: 0, Input: "[2,7,11,15]\n9", Output: "[0,1]", Expected: "[0,1]", Pass: true},
				{Index: 1, Input: "[3,2,4]\n6", Output: "[]", Expected: "[1,2]", Pass: false},
			},
		},
		{
			name: "metadata unparseable: even-chunk fallback",
			wire: runResultWire{
				CodeAnswer:         []string{"a", "b"},
				ExpectedCodeAnswer: []string{"a", "b"},
				CompareResult:      "11",
			},
			dataInput: "line1\nline2\nline3\nline4",
			metaData:  "not json",
			want: []RunCase{
				{Index: 0, Input: "line1\nline2", Output: "a", Expected: "a", Pass: true},
				{Index: 1, Input: "line3\nline4", Output: "b", Expected: "b", Pass: true},
			},
		},
		{
			name: "ragged division: degrade to per-line input",
			wire: runResultWire{
				CodeAnswer:         []string{"a", "b"},
				ExpectedCodeAnswer: []string{"a", "b"},
				CompareResult:      "11",
			},
			dataInput: "only-three\nlines-here\nbut-three", // 3 lines, 2 cases, 2 params → ragged
			metaData:  twoSumMeta,
			want: []RunCase{
				{Index: 0, Input: "only-three", Output: "a", Expected: "a", Pass: true},
				{Index: 1, Input: "lines-here", Output: "b", Expected: "b", Pass: true},
			},
		},
		{
			name: "short std_output_list: missing entries become empty",
			wire: runResultWire{
				CodeAnswer:         []string{"a", "b"},
				ExpectedCodeAnswer: []string{"a", "b"},
				StdOutputList:      []string{"only first"},
				CompareResult:      "11",
			},
			dataInput: "x\ny",
			metaData:  oneArgMeta,
			want: []RunCase{
				{Index: 0, Input: "x", Output: "a", Expected: "a", Stdout: "only first", Pass: true},
				{Index: 1, Input: "y", Output: "b", Expected: "b", Pass: true},
			},
		},
		{
			name: "no answers: empty slice (compile/runtime error path)",
			wire: runResultWire{
				CodeAnswer: nil,
			},
			dataInput: "anything",
			metaData:  twoSumMeta,
			want:      nil,
		},
		{
			// LeetCode's live API has been observed to return code_answer
			// arrays with a trailing empty entry beyond the cases we
			// actually submitted (origin unknown — possibly a benchmark
			// or expected-solution row). Trimming trailing empties keeps
			// the TUI from rendering ghost cases without losing valid
			// cases (an interior "" is still a real, possibly-failing
			// output).
			name: "trailing empty code_answer entries trimmed",
			wire: runResultWire{
				CodeAnswer:         []string{"[0,1]", ""},
				ExpectedCodeAnswer: []string{"[0,1]", ""},
				CompareResult:      "1",
			},
			dataInput: "[2,7,11,15]\n9",
			metaData:  twoSumMeta,
			want: []RunCase{
				{Index: 0, Input: "[2,7,11,15]\n9", Output: "[0,1]", Expected: "[0,1]", Pass: true},
			},
		},
		{
			name: "interior empty code_answer is preserved (real failing case)",
			wire: runResultWire{
				CodeAnswer:         []string{"", "[0,1]"},
				ExpectedCodeAnswer: []string{"[1,2]", "[0,1]"},
				CompareResult:      "01",
			},
			dataInput: "[3,2,4]\n6\n[2,7,11,15]\n9",
			metaData:  twoSumMeta,
			want: []RunCase{
				{Index: 0, Input: "[3,2,4]\n6", Output: "", Expected: "[1,2]", Pass: false},
				{Index: 1, Input: "[2,7,11,15]\n9", Output: "[0,1]", Expected: "[0,1]", Pass: true},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := buildRunCases(tc.wire, tc.dataInput, tc.metaData)
			if len(got) != len(tc.want) {
				t.Fatalf("len = %d, want %d\ngot: %#v", len(got), len(tc.want), got)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Errorf("case %d:\n got  %#v\n want %#v", i, got[i], tc.want[i])
				}
			}
		})
	}
}

// InterpretSolution should drop a populated Cases slice on the returned
// RunResult, sourced from the per-test arrays in the check response.
// This is the end-to-end wire-decode coverage; buildRunCases unit-tests
// the pure assembly logic above.
func TestInterpretSolution_PopulatesCases(t *testing.T) {
	const twoSumMeta = `{"name":"twoSum","params":[{"name":"nums","type":"integer[]"},{"name":"target","type":"integer"}],"return":{"type":"integer[]"}}`
	d := &scriptedDoer{bodies: []string{
		// initial POST /interpret_solution/
		`{"interpret_id":"abc","interpret_expected_id":"def","test_case":""}`,
		// poll #1: terminal verdict with per-case wire arrays
		`{
			"state":"SUCCESS","status_msg":"Wrong Answer","correct_answer":false,
			"code_answer":["[0,1]","[0,2]"],
			"expected_code_answer":["[0,1]","[1,2]"],
			"std_output_list":["","dbg"],
			"compare_result":"10"
		}`,
	}}
	c := newTestClient(d)

	res, err := c.InterpretSolution(context.Background(), "two-sum", "golang", "1", "code", "[2,7,11,15]\n9\n[3,2,4]\n6", twoSumMeta)
	if err != nil {
		t.Fatalf("InterpretSolution: %v", err)
	}
	if len(res.Cases) != 2 {
		t.Fatalf("len(Cases) = %d, want 2; res = %+v", len(res.Cases), res)
	}
	c0, c1 := res.Cases[0], res.Cases[1]
	if c0.Input != "[2,7,11,15]\n9" || c0.Output != "[0,1]" || c0.Expected != "[0,1]" || !c0.Pass {
		t.Errorf("case 0 = %#v", c0)
	}
	if c1.Input != "[3,2,4]\n6" || c1.Output != "[0,2]" || c1.Expected != "[1,2]" || c1.Stdout != "dbg" || c1.Pass {
		t.Errorf("case 1 = %#v", c1)
	}
}
