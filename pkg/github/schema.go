package github

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"os/exec"
	"strings"

	"github.com/cockroachdb/errors"
	"github.com/google/go-github/v79/github"
)

// VerifyAndCommitSchema verifies schema is in sync and commits if needed.
//
//nolint:funlen // complex function with API calls and error handling
func VerifyAndCommitSchema(
	ctx context.Context,
	log *slog.Logger,
	client *Client,
	repo string,
	branch string,
	schemaFile string,
	dryRun bool,
) (bool, error) {
	const ownerNameParts = 2

	// Split repo into owner/name
	parts := strings.SplitN(repo, "/", ownerNameParts)
	if len(parts) != ownerNameParts {
		return false, errors.Newf("invalid repo format: %s (expected owner/name)", repo)
	}

	owner, name := parts[0], parts[1]

	// Generate current schema
	log.Info("generating schema")

	cmd := exec.CommandContext(ctx, "./bin/dotsync", "config", "schema")

	output, err := cmd.Output()
	if err != nil {
		return false, errors.Wrap(err, "generating schema")
	}

	// Read current schema file
	log.Info("reading committed schema", "file", schemaFile)

	//nolint:gosec // schemaFile is controlled input from CLI flags
	currentSchema, err := os.ReadFile(schemaFile)
	if err != nil {
		return false, errors.Wrap(err, "reading schema file")
	}

	// Compare schemas (normalize JSON for comparison)
	var generated, current map[string]any

	if err = json.Unmarshal(output, &generated); err != nil {
		return false, errors.Wrap(err, "unmarshaling generated schema")
	}

	if err = json.Unmarshal(currentSchema, &current); err != nil {
		return false, errors.Wrap(err, "unmarshaling current schema")
	}

	// Compare by marshaling both back to JSON (normalized)
	generatedJSON, err := json.Marshal(generated)
	if err != nil {
		return false, errors.Wrap(err, "marshaling generated schema")
	}

	currentJSON, err := json.Marshal(current)
	if err != nil {
		return false, errors.Wrap(err, "marshaling current schema")
	}

	if string(generatedJSON) == string(currentJSON) {
		log.Info("schema is up to date")

		return false, nil
	}

	log.Info("schema is out of sync, creating commit")

	if dryRun {
		log.Info("dry run: would commit schema changes")

		return true, nil
	}

	// Get current ref SHA
	ref, _, err := client.Git.GetRef(ctx, owner, name, "refs/heads/"+branch)
	if err != nil {
		return false, errors.Wrap(err, "getting ref")
	}

	currentSHA := ref.Object.GetSHA()

	// Get current tree SHA
	commit, _, err := client.Git.GetCommit(ctx, owner, name, currentSHA)
	if err != nil {
		return false, errors.Wrap(err, "getting commit")
	}

	treeSHA := commit.Tree.GetSHA()

	// Create blob for updated schema
	blob := github.Blob{
		Content:  github.Ptr(string(output)),
		Encoding: github.Ptr("utf-8"),
	}

	createdBlob, _, err := client.Git.CreateBlob(ctx, owner, name, blob)
	if err != nil {
		return false, errors.Wrap(err, "creating blob")
	}

	blobSHA := createdBlob.GetSHA()

	// Create new tree with updated schema
	treeEntries := []*github.TreeEntry{
		{
			Path: github.Ptr(schemaFile),
			Mode: github.Ptr("100644"),
			Type: github.Ptr("blob"),
			SHA:  github.Ptr(blobSHA),
		},
	}

	createdTree, _, err := client.Git.CreateTree(ctx, owner, name, treeSHA, treeEntries)
	if err != nil {
		return false, errors.Wrap(err, "creating tree")
	}

	newTreeSHA := createdTree.GetSHA()

	// Create commit
	commitMessage := `chore(schema): regenerate sync-config schema

Schema was out of sync with Go types. Regenerated using:
./bin/dotsync config schema > schemas/sync-config.schema.json`

	newCommit := github.Commit{
		Message: github.Ptr(commitMessage),
		Tree:    &github.Tree{SHA: github.Ptr(newTreeSHA)},
		Parents: []*github.Commit{{SHA: github.Ptr(currentSHA)}},
	}

	createdCommit, _, err := client.Git.CreateCommit(ctx, owner, name, newCommit, nil)
	if err != nil {
		return false, errors.Wrap(err, "creating commit")
	}

	commitSHA := createdCommit.GetSHA()

	// Update ref
	updateRef := github.UpdateRef{
		SHA:   commitSHA,
		Force: github.Ptr(false),
	}

	_, _, err = client.Git.UpdateRef(ctx, owner, name, "heads/"+branch, updateRef)
	if err != nil {
		return false, errors.Wrap(err, "updating ref")
	}

	log.Info("schema committed", "commit", commitSHA[:7])

	return true, nil
}
