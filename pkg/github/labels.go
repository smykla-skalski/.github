package github

import (
	"context"
	"os"

	"github.com/cockroachdb/errors"
	"github.com/google/go-github/v80/github"
	"go.yaml.in/yaml/v4"

	"github.com/smykla-labs/.github/internal/configtypes"
	"github.com/smykla-labs/.github/pkg/logger"
)

// Label represents a GitHub label definition.
type Label struct {
	Name        string `json:"name"                  yaml:"name"`
	Color       string `json:"color"                 yaml:"color"`
	Description string `json:"description,omitempty" yaml:"description"`
}

// LabelsFile represents the structure of a labels YAML file.
type LabelsFile struct {
	Labels []Label `yaml:"-"`
}

// UnmarshalYAML implements custom YAML unmarshaling for LabelsFile.
func (lf *LabelsFile) UnmarshalYAML(value *yaml.Node) error {
	return value.Decode(&lf.Labels)
}

// SyncLabels synchronizes labels from a YAML file to a target repository.
func SyncLabels(
	ctx context.Context,
	log *logger.Logger,
	client *Client,
	org string,
	repo string,
	labelsFile string,
	syncConfig *configtypes.SyncConfig,
	dryRun bool,
) (*LabelsSyncResult, error) {
	result := NewLabelsSyncResult(repo, dryRun)

	// Check if sync is skipped
	if syncConfig.Sync.Skip || syncConfig.Sync.Labels.Skip {
		log.Info("label sync skipped by config")
		result.CompleteSkipped("sync disabled by config")

		return result, nil
	}

	// Parse labels file
	desiredLabels, err := parseLabelsFile(labelsFile)
	if err != nil {
		result.CompleteWithError(errors.Wrap(err, "parsing labels file"))

		return result, errors.Wrap(err, "parsing labels file")
	}

	log.Debug("parsed labels file",
		"file", labelsFile,
		"count", len(desiredLabels),
	)

	// Apply exclusions
	desiredLabels = applyExclusions(desiredLabels, syncConfig.Sync.Labels.Exclude)

	log.Debug("labels after exclusions", "count", len(desiredLabels))

	// Fetch current labels from repository
	currentLabels, err := fetchCurrentLabels(ctx, client, org, repo)
	if err != nil {
		result.CompleteWithError(errors.Wrap(err, "fetching current labels"))

		return result, errors.Wrap(err, "fetching current labels")
	}

	log.Debug("fetched current labels", "count", len(currentLabels))

	// Compute diff
	toCreate, toUpdate, toDelete := computeLabelDiff(
		desiredLabels,
		currentLabels,
		syncConfig.Sync.Labels.AllowRemoval,
	)

	log.Info("computed label diff",
		"to_create", len(toCreate),
		"to_update", len(toUpdate),
		"to_delete", len(toDelete),
	)

	// Set counts in result
	result.Created = len(toCreate)
	result.Updated = len(toUpdate)
	result.Deleted = len(toDelete)

	// Apply changes
	if dryRun {
		log.Info("dry-run mode: skipping label changes")
		logLabelChanges(log, toCreate, toUpdate, toDelete)
		result.Complete(StatusSuccess)

		return result, nil
	}

	if err := applyLabelChanges(ctx, client, org, repo, toCreate, toUpdate, toDelete); err != nil {
		result.CompleteWithError(errors.Wrap(err, "applying label changes"))

		return result, errors.Wrap(err, "applying label changes")
	}

	log.Info("label sync completed successfully")
	result.Complete(StatusSuccess)

	return result, nil
}

// parseLabelsFile reads and parses a YAML labels file.
func parseLabelsFile(path string) ([]Label, error) {
	//nolint:gosec // File path is provided by user via CLI flag
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, errors.Wrap(err, "reading labels file")
	}

	var labelsFile LabelsFile
	if err := yaml.Unmarshal(data, &labelsFile); err != nil {
		return nil, errors.Wrap(err, "unmarshaling labels YAML")
	}

	// Validate labels
	for i, label := range labelsFile.Labels {
		if label.Name == "" {
			return nil, errors.Newf("label at index %d has empty name", i)
		}

		if label.Color == "" {
			return nil, errors.Newf("label %q has empty color", label.Name)
		}

		// Normalize color: ensure it starts with #
		if label.Color[0] != '#' {
			labelsFile.Labels[i].Color = "#" + label.Color
		}
	}

	return labelsFile.Labels, nil
}

// applyExclusions filters out excluded labels.
func applyExclusions(labels []Label, exclude []string) []Label {
	if len(exclude) == 0 {
		return labels
	}

	excludeMap := make(map[string]struct{}, len(exclude))
	for _, name := range exclude {
		excludeMap[name] = struct{}{}
	}

	filtered := make([]Label, 0, len(labels))
	for _, label := range labels {
		if _, excluded := excludeMap[label.Name]; !excluded {
			filtered = append(filtered, label)
		}
	}

	return filtered
}

const (
	labelsPerPage = 100
)

// fetchCurrentLabels retrieves all labels from a repository.
func fetchCurrentLabels(
	ctx context.Context,
	client *Client,
	org string,
	repo string,
) (map[string]*github.Label, error) {
	opts := &github.ListOptions{PerPage: labelsPerPage}
	allLabels := make(map[string]*github.Label)

	for {
		labels, resp, err := client.Issues.ListLabels(ctx, org, repo, opts)
		if err != nil {
			return nil, errors.Wrap(err, "listing labels")
		}

		for _, label := range labels {
			allLabels[label.GetName()] = label
		}

		if resp.NextPage == 0 {
			break
		}

		opts.Page = resp.NextPage
	}

	return allLabels, nil
}

// computeLabelDiff computes which labels need to be created, updated, or deleted.
func computeLabelDiff(
	desired []Label,
	current map[string]*github.Label,
	allowRemoval bool,
) (toCreate, toUpdate []Label, toDelete []*github.Label) {
	// Build desired map for efficient lookup
	desiredMap := make(map[string]Label, len(desired))
	for _, label := range desired {
		desiredMap[label.Name] = label
	}

	// Find creates and updates
	toCreate, toUpdate = findCreatesAndUpdates(desired, current)

	// Find deletes (only if allow_removal is true)
	if allowRemoval {
		for name, label := range current {
			if _, exists := desiredMap[name]; !exists {
				toDelete = append(toDelete, label)
			}
		}
	}

	return toCreate, toUpdate, toDelete
}

// findCreatesAndUpdates identifies labels that need to be created or updated.
func findCreatesAndUpdates(
	desired []Label,
	current map[string]*github.Label,
) (toCreate, toUpdate []Label) {
	for _, label := range desired {
		currentLabel, exists := current[label.Name]
		if !exists {
			toCreate = append(toCreate, label)

			continue
		}

		// Check if update needed (color or description changed)
		if labelNeedsUpdate(label, currentLabel) {
			toUpdate = append(toUpdate, label)
		}
	}

	return toCreate, toUpdate
}

// labelNeedsUpdate checks if a label needs to be updated.
func labelNeedsUpdate(desired Label, current *github.Label) bool {
	// Normalize colors for comparison (both should have #)
	currentColor := current.GetColor()
	if currentColor != "" && currentColor[0] != '#' {
		currentColor = "#" + currentColor
	}

	desiredColor := desired.Color
	if desiredColor != "" && desiredColor[0] != '#' {
		desiredColor = "#" + desiredColor
	}

	if currentColor != desiredColor {
		return true
	}

	if current.GetDescription() != desired.Description {
		return true
	}

	return false
}

// applyLabelChanges applies label creates, updates, and deletes to a repository.
func applyLabelChanges(
	ctx context.Context,
	client *Client,
	org string,
	repo string,
	toCreate []Label,
	toUpdate []Label,
	toDelete []*github.Label,
) error {
	// Create new labels
	for _, label := range toCreate {
		ghLabel := &github.Label{
			Name:        github.Ptr(label.Name),
			Color:       github.Ptr(label.Color),
			Description: github.Ptr(label.Description),
		}

		_, _, err := client.Issues.CreateLabel(ctx, org, repo, ghLabel)
		if err != nil {
			return errors.Wrapf(err, "creating label %q", label.Name)
		}

		client.log.Debug("created label", "name", label.Name)
	}

	// Update existing labels
	for _, label := range toUpdate {
		ghLabel := &github.Label{
			Name:        github.Ptr(label.Name),
			Color:       github.Ptr(label.Color),
			Description: github.Ptr(label.Description),
		}

		_, _, err := client.Issues.EditLabel(ctx, org, repo, label.Name, ghLabel)
		if err != nil {
			return errors.Wrapf(err, "updating label %q", label.Name)
		}

		client.log.Debug("updated label", "name", label.Name)
	}

	// Delete labels
	for _, label := range toDelete {
		_, err := client.Issues.DeleteLabel(ctx, org, repo, label.GetName())
		if err != nil {
			return errors.Wrapf(err, "deleting label %q", label.GetName())
		}

		client.log.Debug("deleted label", "name", label.GetName())
	}

	return nil
}

// logLabelChanges logs the planned label changes in dry-run mode.
func logLabelChanges(
	log *logger.Logger,
	toCreate []Label,
	toUpdate []Label,
	toDelete []*github.Label,
) {
	if len(toCreate) > 0 {
		log.Info("labels to create:")

		for _, label := range toCreate {
			log.Info("  + "+label.Name, "color", label.Color, "description", label.Description)
		}
	}

	if len(toUpdate) > 0 {
		log.Info("labels to update:")

		for _, label := range toUpdate {
			log.Info("  ~ "+label.Name, "color", label.Color, "description", label.Description)
		}
	}

	if len(toDelete) > 0 {
		log.Info("labels to delete:")

		for _, label := range toDelete {
			log.Info("  - " + label.GetName())
		}
	}

	if len(toCreate)+len(toUpdate)+len(toDelete) == 0 {
		log.Info("no label changes needed")
	}
}
