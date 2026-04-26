package leetcode

import (
	"context"
	"encoding/json"
	"fmt"
)

const myCreatedFavoriteListQuery = `
query myCreatedFavoriteList {
  myCreatedFavoriteList {
    hasMore
    total
    favorites {
      name
      slug
      description
      isPublicFavorite
      questionCount
    }
  }
}
`

// allFavoritesQuery covers the user's saved/default favorites (in addition to
// the lists they created). Field shape is `FavoriteNode` which uses
// `questionCount` (not questionNumber).
const allFavoritesQuery = `
query allFavorites {
  favoritesLists {
    allFavorites {
      idHash
      name
      isPublicFavorite
      questionCount
    }
  }
}
`

// MyFavoriteLists fetches the lists the user can pick from. Merges results
// from the two known schema entry points (`myCreatedFavoriteList` for lists
// the user authored, `favoritesLists.allFavorites` for everything they've
// saved or that's been provided by default). Errors from one path don't
// prevent the other from being used; if both fail, the second error is
// returned for diagnostics.
func (c *Client) MyFavoriteLists(ctx context.Context) ([]FavoriteList, error) {
	referer := BaseURL + "/problem-list/"
	seen := map[string]struct{}{}
	out := []FavoriteList{}

	if data, err := c.doGraphQL(ctx, "myCreatedFavoriteList", myCreatedFavoriteListQuery, map[string]any{}, referer); err == nil {
		var wrap struct {
			MyCreatedFavoriteList struct {
				Favorites []FavoriteList `json:"favorites"`
			} `json:"myCreatedFavoriteList"`
		}
		if jerr := json.Unmarshal(data, &wrap); jerr == nil {
			for _, f := range wrap.MyCreatedFavoriteList.Favorites {
				if f.Slug == "" {
					continue
				}
				if _, dup := seen[f.Slug]; dup {
					continue
				}
				seen[f.Slug] = struct{}{}
				out = append(out, f)
			}
		}
	}

	data, err := c.doGraphQL(ctx, "allFavorites", allFavoritesQuery, map[string]any{}, referer)
	if err != nil {
		if len(out) > 0 {
			return out, nil
		}
		return nil, fmt.Errorf("fetch lists: %w", err)
	}

	var legacy struct {
		FavoritesLists struct {
			AllFavorites []struct {
				IDHash           string `json:"idHash"`
				Name             string `json:"name"`
				IsPublicFavorite bool   `json:"isPublicFavorite"`
				QuestionCount    int    `json:"questionCount"`
			} `json:"allFavorites"`
		} `json:"favoritesLists"`
	}
	if err := json.Unmarshal(data, &legacy); err != nil {
		if len(out) > 0 {
			return out, nil
		}
		return nil, fmt.Errorf("decode favoritesLists: %w", err)
	}
	for _, f := range legacy.FavoritesLists.AllFavorites {
		if f.IDHash == "" {
			continue
		}
		if _, dup := seen[f.IDHash]; dup {
			continue
		}
		seen[f.IDHash] = struct{}{}
		out = append(out, FavoriteList{
			Slug:             f.IDHash,
			Name:             f.Name,
			IsPublicFavorite: f.IsPublicFavorite,
			QuestionCount:    f.QuestionCount,
		})
	}
	return out, nil
}
