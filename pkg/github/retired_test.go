package github

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	gogithub "github.com/google/go-github/v90/github"

	"github.com/smykla-skalski/.github/pkg/logger"
)

// testClient points a client at a stub API whose .github/workflows directory
// holds the given paths. calls counts the requests it served.
func testClient(t *testing.T, present []string, calls *int) *Client {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls != nil {
			*calls++
		}

		path := r.URL.Path
		if i := strings.Index(path, "/contents/"); i >= 0 {
			path = path[i+len("/contents/"):]
		}

		if path != workflowsDir {
			w.WriteHeader(http.StatusNotFound)
			fmt.Fprint(w, `{"message": "Not Found"}`)

			return
		}

		// A repository with no workflows directory at all
		if present == nil {
			w.WriteHeader(http.StatusNotFound)
			fmt.Fprint(w, `{"message": "Not Found"}`)

			return
		}

		entries := make([]string, 0, len(present))
		for _, p := range present {
			entries = append(entries,
				fmt.Sprintf(`{"type":"file","name":%q,"path":%q}`, path, p))
		}

		fmt.Fprintf(w, "[%s]", strings.Join(entries, ","))
	}))
	t.Cleanup(server.Close)

	gh, err := gogithub.NewClient(gogithub.WithEnterpriseURLs(server.URL, server.URL))
	if err != nil {
		t.Fatalf("creating stub client: %v", err)
	}

	return &Client{Client: gh, log: logger.New("error")}
}

// The listing is what makes this cheap. A fetch per retired name would cost one
// request per name on every sync of every repository, for as long as the check
// exists, and the usual answer is that there is nothing to delete.
func TestCheckRetiredWorkflowsCostsOneRequest(t *testing.T) {
	var calls int

	stats := &FileSyncStats{MergedFiles: make(map[string]string)}
	client := testClient(t, []string{".github/workflows/test.yml"}, &calls)

	changes := checkRetiredWorkflows(
		context.Background(), logger.New("error"), client, "org", "repo", stats,
	)

	if len(changes) != 0 {
		t.Errorf("changes = %d, want 0", len(changes))
	}

	if calls != 1 {
		t.Errorf("requests = %d, want 1", calls)
	}
}

func TestCheckRetiredWorkflows(t *testing.T) {
	tests := []struct {
		name    string
		present []string
		want    []string
	}{
		{
			name:    "deletes a retired workflow that is still there",
			present: []string{".github/workflows/smyklot-pr-commands.yml"},
			want:    []string{".github/workflows/smyklot-pr-commands.yml"},
		},
		{
			name: "deletes both current names",
			present: []string{
				".github/workflows/smyklot-pr-commands.yml",
				".github/workflows/smyklot-poll.yml",
			},
			want: []string{
				".github/workflows/smyklot-pr-commands.yml",
				".github/workflows/smyklot-poll.yml",
			},
		},
		// Repositories synced before the rename still carry the old names, and
		// those run the Action just as happily
		{
			name:    "deletes the pre-rename names",
			present: []string{".github/workflows/poll-reactions.yaml"},
			want:    []string{".github/workflows/poll-reactions.yaml"},
		},
		{
			name:    "does nothing for a repository that has none",
			present: nil,
			want:    nil,
		},
		// A repository is not a candidate for deletion just because it has
		// workflows. Only the retired ones go
		{
			name:    "leaves unrelated workflows alone",
			present: []string{".github/workflows/test.yml", ".github/workflows/release.yml"},
			want:    nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stats := &FileSyncStats{MergedFiles: make(map[string]string)}
			client := testClient(t, tt.present, nil)

			changes := checkRetiredWorkflows(
				context.Background(), logger.New("error"), client, "org", "repo", stats,
			)

			var got []string

			for _, c := range changes {
				if c.Action != fileActionDelete {
					t.Errorf("action = %q, want %q", c.Action, fileActionDelete)
				}

				got = append(got, c.Path)
			}

			if !slices.Equal(got, tt.want) {
				t.Errorf("deleted paths = %v, want %v", got, tt.want)
			}

			if stats.Deleted != len(tt.want) {
				t.Errorf("stats.Deleted = %d, want %d", stats.Deleted, len(tt.want))
			}
		})
	}
}
