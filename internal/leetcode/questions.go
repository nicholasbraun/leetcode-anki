package leetcode

import (
	"context"
	"encoding/json"
	"fmt"
)

const favoriteQuestionListQuery = `
query favoriteQuestionList($favoriteSlug: String!, $filter: FavoriteQuestionFilterInput, $filtersV2: QuestionFilterInput, $searchKeyword: String, $sortBy: QuestionSortByInput, $limit: Int, $skip: Int, $version: String = "v2") {
  favoriteQuestionList(
    favoriteSlug: $favoriteSlug
    filter: $filter
    filtersV2: $filtersV2
    searchKeyword: $searchKeyword
    sortBy: $sortBy
    limit: $limit
    skip: $skip
    version: $version
  ) {
    questions {
      difficulty
      id
      paidOnly
      questionFrontendId
      status
      title
      titleSlug
      translatedTitle
      isInMyFavorites
      frequency
      acRate
      contestPoint
      topicTags {
        name
        slug
      }
    }
    totalLength
    hasMore
  }
}
`

type FavoriteQuestionListResult struct {
	Questions   []Question `json:"questions"`
	TotalLength int        `json:"totalLength"`
	HasMore     bool       `json:"hasMore"`
}

// FavoriteQuestionList fetches problems within a user's list (paginated).
func (c *Client) FavoriteQuestionList(ctx context.Context, slug string, skip, limit int) (*FavoriteQuestionListResult, error) {
	vars := map[string]any{
		"skip":         skip,
		"limit":        limit,
		"favoriteSlug": slug,
		"filtersV2": map[string]any{
			"filterCombineType":   "ALL",
			"statusFilter":        map[string]any{"questionStatuses": []string{}, "operator": "IS"},
			"difficultyFilter":    map[string]any{"difficulties": []string{}, "operator": "IS"},
			"languageFilter":      map[string]any{"languageSlugs": []string{}, "operator": "IS"},
			"topicFilter":         map[string]any{"topicSlugs": []string{}, "operator": "IS"},
			"acceptanceFilter":    map[string]any{},
			"frequencyFilter":     map[string]any{},
			"frontendIdFilter":    map[string]any{},
			"lastSubmittedFilter": map[string]any{},
			"publishedFilter":     map[string]any{},
			"companyFilter":       map[string]any{"companySlugs": []string{}, "operator": "IS"},
			"positionFilter":      map[string]any{"positionSlugs": []string{}, "operator": "IS"},
			"positionLevelFilter": map[string]any{"positionLevelSlugs": []string{}, "operator": "IS"},
			"contestPointFilter":  map[string]any{"contestPoints": []string{}, "operator": "IS"},
			"premiumFilter":       map[string]any{"premiumStatus": []string{}, "operator": "IS"},
		},
		"searchKeyword": "",
		"sortBy": map[string]any{
			"sortField": "CUSTOM",
			"sortOrder": "ASCENDING",
		},
	}

	referer := problemListRefURL(slug)
	data, err := c.doGraphQL(ctx, "favoriteQuestionList", favoriteQuestionListQuery, vars, referer)
	if err != nil {
		return nil, err
	}

	var wrap struct {
		FavoriteQuestionList FavoriteQuestionListResult `json:"favoriteQuestionList"`
	}
	if err := json.Unmarshal(data, &wrap); err != nil {
		return nil, fmt.Errorf("decode favoriteQuestionList: %w", err)
	}
	return &wrap.FavoriteQuestionList, nil
}

const questionDetailQuery = `
query questionData($titleSlug: String!) {
  question(titleSlug: $titleSlug) {
    questionId
    questionFrontendId
    title
    titleSlug
    difficulty
    isPaidOnly
    content
    codeSnippets {
      lang
      langSlug
      code
    }
    exampleTestcases
    sampleTestCase
    metaData
    hints
    topicTags {
      name
      slug
    }
  }
}
`

// ProblemDetail fetches the full content (description, code snippets, etc.) of a single Problem.
func (c *Client) ProblemDetail(ctx context.Context, titleSlug string) (*ProblemDetail, error) {
	vars := map[string]any{"titleSlug": titleSlug}
	referer := problemRefURL(titleSlug)

	data, err := c.doGraphQL(ctx, "questionData", questionDetailQuery, vars, referer)
	if err != nil {
		return nil, err
	}

	var wrap struct {
		Question *ProblemDetail `json:"question"`
	}
	if err := json.Unmarshal(data, &wrap); err != nil {
		return nil, fmt.Errorf("decode question: %w", err)
	}
	if wrap.Question == nil {
		return nil, fmt.Errorf("question %q not found", titleSlug)
	}
	return wrap.Question, nil
}
