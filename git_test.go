// Copyright 2016 Giulio Iotti. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package umarell

import (
	"testing"
)

const gitLogOutput = `0ff715f31f275dcdc16762ae9e80c0afbb6c1be0 56e044f8fa828eada0d19cd025507309fe4900d2 b72759cacd2848ce0828a2921b93cb9157948297
b72759cacd2848ce0828a2921b93cb9157948297 56e044f8fa828eada0d19cd025507309fe4900d2
`

const gitShortOutput = `0ff715f31f275dcdc16762ae9e80c0afbb6c1be0 56e044f8fa828eada0d19cd025507309fe4900d2`

func firstCommitIs(c *gitcommits, hash string) bool {
	if len(c.commits) == 0 {
		return false
	}
	return string(c.commits[0].hash) == hash
}

func TestGitLogParsing(t *testing.T) {
	commits := newGitcommits()
	commits.scanCommits([]byte(gitLogOutput))
	if !firstCommitIs(commits, "0ff715f31f275dcdc16762ae9e80c0afbb6c1be0") {
		t.Error("expected first commit 0ff715 not found")
	}

	commits = newGitcommits()
	commits.scanCommits([]byte(gitShortOutput))
	if !firstCommitIs(commits, "0ff715f31f275dcdc16762ae9e80c0afbb6c1be0") {
		t.Error("expected first commit 0ff715 not found")
	}
}
