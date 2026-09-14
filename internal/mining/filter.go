package mining

import (
	"regexp"
	"strings"
)

var noiseRe = regexp.MustCompile(`(?i)^(lgtm|👍|👌|🚀|\+1|done|fixed|nit|thx|thanks?|ok|okay|nice|agreed?|sure|yes|no)[.!?]*$`)

// keepAssociations are the author associations counted as internal reviewers.
var keepAssociations = map[string]bool{"MEMBER": true, "OWNER": true, "COLLABORATOR": true}

// Keep reports whether a comment is substantive internal review feedback worth
// distilling: a human (non-bot) member/owner/collaborator, body of real length
// and not pure acknowledgement noise.
func Keep(c Comment) bool {
	if c.Author == "" || strings.HasSuffix(c.Author, "[bot]") {
		return false
	}
	if !keepAssociations[c.Association] {
		return false
	}
	body := strings.TrimSpace(c.Body)
	if len([]rune(body)) < 15 {
		return false
	}
	if noiseRe.MatchString(body) {
		return false
	}
	return true
}
