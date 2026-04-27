package leetcode

import "net/url"

// problemRefURL returns the canonical Referer header value for endpoints
// scoped to a single problem (e.g. submit, interpret_solution, problem
// detail). Centralised so slug escaping is consistent across the package
// even if validSlug is later relaxed.
func problemRefURL(slug string) string {
	return BaseURL + "/problems/" + url.PathEscape(slug) + "/"
}

// interpretURL is the REST endpoint that runs a Solution against the
// Problem's example test cases.
func interpretURL(slug string) string {
	return BaseURL + "/problems/" + url.PathEscape(slug) + "/interpret_solution/"
}

// submitURL is the REST endpoint that submits a Solution to LeetCode's
// full grader.
func submitURL(slug string) string {
	return BaseURL + "/problems/" + url.PathEscape(slug) + "/submit/"
}

// submissionCheckURL is the REST endpoint Run/Submit poll for a Verdict.
// The id is LeetCode's interpret_id / submission_id.
func submissionCheckURL(id string) string {
	return BaseURL + "/submissions/detail/" + url.PathEscape(id) + "/check/"
}

// submissionsRefURL is the Referer for the submissionList GraphQL query;
// LeetCode wants Referer to match the page that "owns" the data being
// requested, which for per-Problem submissions is the per-slug page.
func submissionsRefURL(slug string) string {
	return BaseURL + "/problems/" + url.PathEscape(slug) + "/submissions/"
}

// problemListRefURL is the Referer for queries scoped to a single
// favorite list (favoriteQuestionList).
func problemListRefURL(slug string) string {
	return BaseURL + "/problem-list/" + url.PathEscape(slug) + "/"
}
