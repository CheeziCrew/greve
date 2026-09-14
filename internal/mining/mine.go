package mining

import (
	"fmt"
	"time"
)

// Options controls a mining run.
type Options struct {
	MaxPRsPerRepo int  // 0 = no cap
	Resume        bool // continue from saved per-repo cursors
	DryRun        bool // count only, write nothing
}

// Stats summarises one repo's mining.
type Stats struct {
	Repo     string `json:"repo"`
	PRsSeen  int    `json:"prs_seen"`
	Kept     int    `json:"kept"`
	Filtered int    `json:"filtered"`
	Err      string `json:"error,omitempty"`
}

const rateFloor = 200

// Mine pulls review comments for repos into store. A per-repo error (e.g. a
// missing repo) is recorded and skipped rather than aborting the whole run.
func Mine(repos []Repo, store *Store, opts Options, log func(string)) []Stats {
	progress := store.LoadProgress()
	var all []Stats

	for _, r := range repos {
		key := r.Key()
		if opts.Resume && progress[key] == "done" {
			log(fmt.Sprintf("%s: already complete, skipping", key))
			continue
		}

		cursor := ""
		if opts.Resume {
			if c := progress[key]; c != "" && c != "done" {
				cursor = c
			}
		}

		st := Stats{Repo: key}
		fullyConsumed := false
		for {
			page, err := fetchPage(r.Org, r.Name, cursor)
			if err != nil {
				st.Err = err.Error()
				log(fmt.Sprintf("%s: %v (skipping)", key, err))
				break
			}
			st.PRsSeen += page.prCount
			for _, c := range page.comments {
				if !Keep(c) {
					st.Filtered++
					continue
				}
				if opts.DryRun {
					st.Kept++
					continue
				}
				if !store.Has(c.ID) {
					if err := store.Append(c); err == nil {
						st.Kept++
					}
				}
			}

			cursor = page.endCursor
			if !opts.DryRun {
				_ = store.Flush()
				progress[key] = cursor
				store.SaveProgress(progress)
			}

			if !page.hasNext {
				fullyConsumed = true
				break
			}
			if opts.MaxPRsPerRepo > 0 && st.PRsSeen >= opts.MaxPRsPerRepo {
				break
			}

			pace(page, log)
		}

		if fullyConsumed && !opts.DryRun {
			progress[key] = "done"
			store.SaveProgress(progress)
		}
		log(fmt.Sprintf("%s: %d PRs, kept %d, filtered %d (corpus now %d)", key, st.PRsSeen, st.Kept, st.Filtered, store.Count()))
		all = append(all, st)
	}
	return all
}

// pace sleeps a beat between pages, or until reset when the budget runs low.
func pace(page *pageResult, log func(string)) {
	if page.rateRemaining < rateFloor {
		wait := untilReset(page.rateResetAt)
		log(fmt.Sprintf("rate budget low (%d left), sleeping %s", page.rateRemaining, wait.Round(time.Second)))
		time.Sleep(wait)
		return
	}
	time.Sleep(200 * time.Millisecond)
}

func untilReset(resetAt string) time.Duration {
	t, err := time.Parse(time.RFC3339, resetAt)
	if err != nil {
		return time.Minute
	}
	d := time.Until(t) + 5*time.Second
	if d < 0 {
		return 5 * time.Second
	}
	return d
}
