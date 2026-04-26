package leetcode

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"leetcode-anki/internal/auth"
)

// routedDoer dispatches based on the GraphQL operationName in the request body.
// Anything not in the table returns a 500.
type routedDoer struct {
	byOp map[string]string
}

func (r *routedDoer) Do(req *http.Request) (*http.Response, error) {
	body, _ := io.ReadAll(req.Body)
	var probe struct {
		OperationName string `json:"operationName"`
	}
	_ = json.Unmarshal(body, &probe)

	respBody, ok := r.byOp[probe.OperationName]
	if !ok {
		return &http.Response{
			StatusCode: 500,
			Body:       io.NopCloser(strings.NewReader(`{"errors":[{"message":"unknown op"}]}`)),
			Header:     make(http.Header),
		}, nil
	}
	return &http.Response{
		StatusCode: 200,
		Body:       io.NopCloser(strings.NewReader(respBody)),
		Header:     make(http.Header),
	}, nil
}

func TestMyFavoriteLists_MergesAndDedupes(t *testing.T) {
	d := &routedDoer{byOp: map[string]string{
		"myCreatedFavoriteList": `{"data":{"myCreatedFavoriteList":{"favorites":[
			{"slug":"created-1","name":"My Sheet","questionNumber":42,"isPublicFavorite":false},
			{"slug":"shared","name":"Authored","questionNumber":10}
		]}}}`,
		"allFavorites": `{"data":{"favoritesLists":{"allFavorites":[
			{"idHash":"shared","name":"Saved (dup)","questionCount":99},
			{"idHash":"saved-1","name":"Neetcode 75","questionCount":75,"isPublicFavorite":true}
		]}}}`,
	}}
	c := newClientWithDoer(&auth.Credentials{Session: "s", CSRF: "c"}, d)

	got, err := c.MyFavoriteLists(context.Background())
	if err != nil {
		t.Fatalf("MyFavoriteLists: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("expected 3 unique lists, got %d: %+v", len(got), got)
	}
	// Authored entries appear first; the dup ("shared") is suppressed when
	// the second query returns it again.
	if got[0].Slug != "created-1" || got[1].Slug != "shared" || got[2].Slug != "saved-1" {
		t.Errorf("unexpected order/contents: %+v", got)
	}
	// The dup should keep the *first* occurrence's name, not the saved one.
	if got[1].Name != "Authored" {
		t.Errorf("dedup kept the wrong name: got %q", got[1].Name)
	}
}

// If `myCreatedFavoriteList` 500s but `allFavorites` succeeds, the user still
// sees their saved lists. Without this resilience a single schema regression
// on either side would empty the lists screen.
func TestMyFavoriteLists_FallsBackWhenFirstFails(t *testing.T) {
	d := &routedDoer{byOp: map[string]string{
		"allFavorites": `{"data":{"favoritesLists":{"allFavorites":[
			{"idHash":"only","name":"Only One","questionCount":1}
		]}}}`,
	}}
	c := newClientWithDoer(&auth.Credentials{Session: "s", CSRF: "c"}, d)

	got, err := c.MyFavoriteLists(context.Background())
	if err != nil {
		t.Fatalf("MyFavoriteLists: %v", err)
	}
	if len(got) != 1 || got[0].Slug != "only" {
		t.Errorf("expected fallback to surface the saved list; got %+v", got)
	}
}

func TestMyFavoriteLists_ErrorsWhenBothFail(t *testing.T) {
	d := &routedDoer{byOp: map[string]string{}}
	c := newClientWithDoer(&auth.Credentials{Session: "s", CSRF: "c"}, d)

	if _, err := c.MyFavoriteLists(context.Background()); err == nil {
		t.Error("expected error when both queries fail")
	}
}

func TestMyFavoriteLists_SkipsBlankSlugs(t *testing.T) {
	d := &routedDoer{byOp: map[string]string{
		"myCreatedFavoriteList": `{"data":{"myCreatedFavoriteList":{"favorites":[
			{"slug":"","name":"Bad"},
			{"slug":"good","name":"Good"}
		]}}}`,
		"allFavorites": `{"data":{"favoritesLists":{"allFavorites":[
			{"idHash":"","name":"Also Bad"}
		]}}}`,
	}}
	c := newClientWithDoer(&auth.Credentials{Session: "s", CSRF: "c"}, d)

	got, err := c.MyFavoriteLists(context.Background())
	if err != nil {
		t.Fatalf("MyFavoriteLists: %v", err)
	}
	if len(got) != 1 || got[0].Slug != "good" {
		t.Errorf("expected blank slugs filtered; got %+v", got)
	}
}
