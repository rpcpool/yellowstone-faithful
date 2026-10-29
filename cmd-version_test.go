package main

import "testing"

func TestUserAgent(t *testing.T) {
	defer func(tag, commit string) { GitTag, GitCommit = tag, commit }(GitTag, GitCommit)

	cases := []struct{ tag, commit, want string }{
		{"v1.2.3", "9ca9ec1", "faithful-cli/v1.2.3"},
		{"", "9ca9ec1", "faithful-cli/9ca9ec1"},
		{"", "", "faithful-cli/unknown"},
	}
	for _, c := range cases {
		GitTag, GitCommit = c.tag, c.commit
		if got := userAgent(); got != c.want {
			t.Errorf("userAgent() with tag %q, commit %q = %q, want %q", c.tag, c.commit, got, c.want)
		}
	}
}
