package stashbox

import (
	"context"
	"fmt"
	"math"

	"github.com/hasura/go-graphql-client"
)

// Page is one page of a paginated query: the rows plus the total the service
// reports, so a caller knows whether to ask for another.
type Page[T any] struct {
	// Items are the rows on this page.
	Items []T
	// Count is the total matching rows, not the page size.
	Count int
}

// HasMore reports whether a further page exists after the one that produced p,
// given the page number just fetched and the page size used.
func (p Page[T]) HasMore(page, perPage int) bool {
	return page*perPage < p.Count
}

// FavoritePerformers returns one page of the account's favourited performers.
//
// stash-box filters favourites server-side, so this is a page fetch rather
// than a crawl of everything followed by a local filter.
func (c *Client) FavoritePerformers(ctx context.Context, page, perPage int) (Page[Performer], error) {
	var q struct {
		QueryPerformers struct {
			Count      int         `graphql:"count"`
			Performers []Performer `graphql:"performers"`
		} `graphql:"queryPerformers(input: {is_favorite: true, page: $page, per_page: $perPage})"`
	}
	if err := c.query(ctx, &q, vars(page, perPage)); err != nil {
		return Page[Performer]{}, fmt.Errorf("stashbox: favorite performers page %d: %w", page, err)
	}
	return Page[Performer]{Items: q.QueryPerformers.Performers, Count: q.QueryPerformers.Count}, nil
}

// AllStudios returns one page of every studio the service knows.
//
// Unfiltered, unlike the performer crawl: a release names the site that
// published it whether or not the account follows that site, so studio
// resolution needs the whole list rather than the favourites.
func (c *Client) AllStudios(ctx context.Context, page, perPage int) (Page[Studio], error) {
	var q struct {
		QueryStudios struct {
			Count   int      `graphql:"count"`
			Studios []Studio `graphql:"studios"`
		} `graphql:"queryStudios(input: {page: $page, per_page: $perPage})"`
	}
	if err := c.query(ctx, &q, vars(page, perPage)); err != nil {
		return Page[Studio]{}, fmt.Errorf("stashbox: studios page %d: %w", page, err)
	}
	return Page[Studio]{Items: q.QueryStudios.Studios, Count: q.QueryStudios.Count}, nil
}

// ScenesForPerformer returns one page of a performer's scenes.
//
// Scenes are fetched per followed entity rather than through the scene query's
// own favourites filter: the crawl has to know which favourite pulled a scene
// in, so that a performer unfollowed later can have their scenes reconciled.
func (c *Client) ScenesForPerformer(ctx context.Context, performerID string, page, perPage int) (Page[Scene], error) {
	var q struct {
		QueryScenes struct {
			Count  int     `graphql:"count"`
			Scenes []Scene `graphql:"scenes"`
		} `graphql:"queryScenes(input: {performers: {value: [$id], modifier: INCLUDES}, page: $page, per_page: $perPage, sort: DATE, direction: DESC})"`
	}
	v := vars(page, perPage)
	v["id"] = graphql.ID(performerID)
	if err := c.query(ctx, &q, v); err != nil {
		return Page[Scene]{}, fmt.Errorf("stashbox: scenes for performer %s page %d: %w", performerID, page, err)
	}
	return Page[Scene]{Items: q.QueryScenes.Scenes, Count: q.QueryScenes.Count}, nil
}

// ScenesForStudio returns one page of a studio's scenes.
func (c *Client) ScenesForStudio(ctx context.Context, studioID string, page, perPage int) (Page[Scene], error) {
	var q struct {
		QueryScenes struct {
			Count  int     `graphql:"count"`
			Scenes []Scene `graphql:"scenes"`
		} `graphql:"queryScenes(input: {studios: {value: [$id], modifier: INCLUDES}, page: $page, per_page: $perPage, sort: DATE, direction: DESC})"`
	}
	v := vars(page, perPage)
	v["id"] = graphql.ID(studioID)
	if err := c.query(ctx, &q, v); err != nil {
		return Page[Scene]{}, fmt.Errorf("stashbox: scenes for studio %s page %d: %w", studioID, page, err)
	}
	return Page[Scene]{Items: q.QueryScenes.Scenes, Count: q.QueryScenes.Count}, nil
}

// query runs one request through the rate limiter.
func (c *Client) query(ctx context.Context, q any, v map[string]any) error {
	if err := c.wait(ctx); err != nil {
		return err
	}
	return c.gql.Query(ctx, q, v)
}

// vars builds the pagination variables every query shares.
//
// graphql.Int is int32; page numbers and page sizes are bounded by the
// service's own limits (a page caps at 100), so the conversion is clamped
// rather than annotated away.
func vars(page, perPage int) map[string]any {
	return map[string]any{
		"page":    graphql.Int(clampInt32(page)),
		"perPage": graphql.Int(clampInt32(perPage)),
	}
}

func clampInt32(n int) int32 {
	switch {
	case n < 1:
		return 1
	case n > math.MaxInt32:
		return math.MaxInt32
	default:
		return int32(n)
	}
}
