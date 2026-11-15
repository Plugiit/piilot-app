package gitsource

import (
	"context"
	"time"
)

// GitLab est l'emplacement du rattrapage GitLab. Pas encore ecrit : le
// webhook GitLab reste la seule source.
type GitLab struct{}

var _ Provider = GitLab{}

// MergedPullRequests n'est pas implemente.
func (GitLab) MergedPullRequests(context.Context, string, time.Time, time.Time) ([]PullRequest, error) {
	return nil, ErrNotImplemented
}
