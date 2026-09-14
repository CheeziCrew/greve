package mining

import (
	"encoding/json"
	"fmt"
	"os/exec"
)

// prQuery pages a repo's PRs (newest first) and pulls each review thread's
// comments, plus the rate-limit budget so the caller can pace itself.
const prQuery = `query($owner:String!,$name:String!,$cursor:String){
  rateLimit{remaining resetAt}
  repository(owner:$owner,name:$name){
    pullRequests(first:25,after:$cursor,orderBy:{field:UPDATED_AT,direction:DESC}){
      pageInfo{hasNextPage endCursor}
      nodes{
        number
        reviewThreads(first:50){
          nodes{
            comments(first:30){
              nodes{ id body path line diffHunk author{login} authorAssociation createdAt }
            }
          }
        }
      }
    }
  }
}`

type pageResult struct {
	rateRemaining int
	rateResetAt   string
	hasNext       bool
	endCursor     string
	prCount       int
	comments      []Comment
}

// fetchPage runs one GraphQL page through the authenticated gh CLI.
func fetchPage(org, repo, cursor string) (*pageResult, error) {
	args := []string{"api", "graphql", "-f", "query=" + prQuery, "-F", "owner=" + org, "-F", "name=" + repo}
	if cursor != "" {
		args = append(args, "-F", "cursor="+cursor)
	}
	out, err := exec.Command("gh", args...).Output()
	if err != nil {
		return nil, fmt.Errorf("gh api graphql: %w", err)
	}

	var resp struct {
		Data struct {
			RateLimit struct {
				Remaining int    `json:"remaining"`
				ResetAt   string `json:"resetAt"`
			} `json:"rateLimit"`
			Repository struct {
				PullRequests struct {
					PageInfo struct {
						HasNextPage bool   `json:"hasNextPage"`
						EndCursor   string `json:"endCursor"`
					} `json:"pageInfo"`
					Nodes []struct {
						Number        int `json:"number"`
						ReviewThreads struct {
							Nodes []struct {
								Comments struct {
									Nodes []struct {
										ID       string `json:"id"`
										Body     string `json:"body"`
										Path     string `json:"path"`
										Line     int    `json:"line"`
										DiffHunk string `json:"diffHunk"`
										Author   struct {
											Login string `json:"login"`
										} `json:"author"`
										AuthorAssociation string `json:"authorAssociation"`
										CreatedAt         string `json:"createdAt"`
									} `json:"nodes"`
								} `json:"comments"`
							} `json:"nodes"`
						} `json:"reviewThreads"`
					} `json:"nodes"`
				} `json:"pullRequests"`
			} `json:"repository"`
		} `json:"data"`
	}
	if err := json.Unmarshal(out, &resp); err != nil {
		return nil, fmt.Errorf("decoding graphql response: %w", err)
	}

	prs := resp.Data.Repository.PullRequests
	page := &pageResult{
		rateRemaining: resp.Data.RateLimit.Remaining,
		rateResetAt:   resp.Data.RateLimit.ResetAt,
		hasNext:       prs.PageInfo.HasNextPage,
		endCursor:     prs.PageInfo.EndCursor,
		prCount:       len(prs.Nodes),
	}
	for _, pr := range prs.Nodes {
		for _, th := range pr.ReviewThreads.Nodes {
			for _, cm := range th.Comments.Nodes {
				page.comments = append(page.comments, Comment{
					ID:          cm.ID,
					Org:         org,
					Repo:        repo,
					PR:          pr.Number,
					Author:      cm.Author.Login,
					Association: cm.AuthorAssociation,
					Path:        cm.Path,
					Line:        cm.Line,
					Body:        cm.Body,
					DiffHunk:    cm.DiffHunk,
					CreatedAt:   cm.CreatedAt,
				})
			}
		}
	}
	return page, nil
}
