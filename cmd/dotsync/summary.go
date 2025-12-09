package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/spf13/cobra"

	"github.com/smykla-labs/.github/pkg/github"
	"github.com/smykla-labs/.github/pkg/logger"
)

const (
	syncTypeAll      = "all"
	syncTypeLabels   = "labels"
	syncTypeFiles    = "files"
	syncTypeSettings = "settings"
	syncTypeSmyklot  = "smyklot"

	filterAll       = "all"
	filterFailures  = "failures"
	filterSuccesses = "successes"
	filterSkipped   = "skipped"

	secondsPerMinute = 60
	minutesPerHour   = 60
)

var summaryCmd = &cobra.Command{
	Use:   "summary",
	Short: "Summary generation commands",
	Long:  "Commands for generating sync operation summaries",
}

var summaryGenerateCmd = &cobra.Command{
	Use:   "generate",
	Short: "Generate sync summary",
	Long:  "Generate summary report from sync operation results",
	RunE: func(cmd *cobra.Command, _ []string) error {
		ctx := cmd.Context()
		log := logger.FromContext(ctx)

		syncType := getStringFlagWithEnvFallback(cmd, "type", "")
		resultsDir := getStringFlagWithEnvFallback(cmd, "results-dir", "")
		output := getStringFlagWithEnvFallback(cmd, "output", "")
		format := getStringFlagWithEnvFallback(cmd, "format", "")
		filter := getStringFlagWithEnvFallback(cmd, "filter", "")
		dryRun := getPersistentBoolFlagWithEnvFallback(cmd, "dry-run")

		// Validate required parameters
		if resultsDir == "" {
			return errors.New("results-dir is required")
		}

		// Infer sync type from result files if not provided
		if syncType == "" {
			inferredType, err := inferSyncType(log, resultsDir)
			if err != nil {
				return err
			}

			syncType = inferredType
			log.Info("inferred sync type from result files", "type", syncType)
		}

		log.Info("generating summary",
			"type", syncType,
			"results_dir", resultsDir,
			"format", format,
			"filter", filter,
			"dry_run", dryRun,
		)

		// Handle type "all" separately - cross-workflow summary
		if syncType == syncTypeAll {
			return generateCrossWorkflowSummary(log, resultsDir, output, filter, dryRun)
		}

		// Generate single workflow summary
		return generateSingleWorkflowSummary(
			log,
			syncType,
			resultsDir,
			output,
			format,
			filter,
			dryRun,
		)
	},
}

// generateSingleWorkflowSummary generates summary for a single sync workflow.
func generateSingleWorkflowSummary(
	log *logger.Logger,
	syncType string,
	resultsDir string,
	output string,
	format string,
	filter string,
	dryRun bool,
) error {
	// Read result files for this sync type
	results, err := readResultFiles(log, resultsDir, syncType)
	if err != nil {
		return err
	}

	// Apply filter
	filteredResults := applyFilterToResults(results, filter)

	log.Info("results loaded",
		"total", len(results),
		"filtered", len(filteredResults),
	)

	switch format {
	case "markdown":
		return generateMarkdown(log, syncType, filteredResults, output, dryRun)
	case "json":
		return generateWorkflowJSON(log, syncType, results, output, dryRun)
	default:
		return errors.Newf("unsupported format: %s", format)
	}
}

// inferSyncType infers the sync type from result file names in the directory.
func inferSyncType(log *logger.Logger, resultsDir string) (string, error) {
	// Check for each type pattern
	types := []string{syncTypeLabels, syncTypeFiles, syncTypeSettings, syncTypeSmyklot}

	for _, syncType := range types {
		pattern := filepath.Join(resultsDir, syncType+"-result-*.json")

		files, err := filepath.Glob(pattern)
		if err != nil {
			return "", errors.Wrap(err, "globbing result files")
		}

		if len(files) > 0 {
			log.Debug("found result files for type", "type", syncType, "count", len(files))
			return syncType, nil
		}
	}

	// Check if directory has workflow summary files (indicates "all" type)
	summaryPattern := filepath.Join(resultsDir, "workflow-summary-*.json")

	files, err := filepath.Glob(summaryPattern)
	if err != nil {
		return "", errors.Wrap(err, "globbing workflow summary files")
	}

	if len(files) > 0 {
		log.Debug("found workflow summary files, using type 'all'", "count", len(files))
		return syncTypeAll, nil
	}

	return "", errors.New("unable to infer sync type: no matching result files found")
}

// generateCrossWorkflowSummary generates consolidated summary from multiple workflows.
func generateCrossWorkflowSummary(
	log *logger.Logger,
	resultsDir string,
	output string,
	filter string,
	dryRun bool,
) error {
	// Read workflow summary files
	summaries, err := readWorkflowSummaries(log, resultsDir)
	if err != nil {
		return err
	}

	log.Info("workflow summaries loaded", "count", len(summaries))

	// Generate markdown with sections per sync type
	return generateCrossWorkflowMarkdown(log, summaries, output, filter, dryRun)
}

// readResultFiles reads per-repo result JSON files from the results directory.
func readResultFiles(log *logger.Logger, resultsDir string, syncType string) ([]any, error) {
	pattern := filepath.Join(resultsDir, syncType+"-result-*.json")

	files, err := filepath.Glob(pattern)
	if err != nil {
		return nil, errors.Wrap(err, "globbing result files")
	}

	if len(files) == 0 {
		log.Warn("no result files found", "pattern", pattern)
		return []any{}, nil
	}

	results := make([]any, 0, len(files))

	for _, file := range files {
		log.Debug("reading result file", "file", file)

		//nolint:gosec // file path is controlled by function caller
		data, err := os.ReadFile(file)
		if err != nil {
			return nil, errors.Wrapf(err, "reading file %s", file)
		}

		// Parse based on sync type
		result, err := parseResultByType(syncType, data)
		if err != nil {
			return nil, errors.Wrapf(err, "parsing result from %s", file)
		}

		results = append(results, result)
	}

	return results, nil
}

// parseResultByType parses result JSON based on sync type.
func parseResultByType(syncType string, data []byte) (any, error) {
	switch syncType {
	case syncTypeLabels:
		var result github.LabelsSyncResult
		if err := json.Unmarshal(data, &result); err != nil {
			return nil, err
		}

		return &result, nil

	case syncTypeFiles:
		var result github.FilesSyncResult
		if err := json.Unmarshal(data, &result); err != nil {
			return nil, err
		}

		return &result, nil

	case syncTypeSettings:
		var result github.SettingsSyncResult
		if err := json.Unmarshal(data, &result); err != nil {
			return nil, err
		}

		return &result, nil

	case syncTypeSmyklot:
		var result github.SmyklotSyncResult
		if err := json.Unmarshal(data, &result); err != nil {
			return nil, err
		}

		return &result, nil

	default:
		return nil, errors.Newf("unknown sync type: %s", syncType)
	}
}

// readWorkflowSummaries reads workflow summary JSON files.
func readWorkflowSummaries(
	log *logger.Logger,
	resultsDir string,
) ([]*github.WorkflowSummary, error) {
	pattern := filepath.Join(resultsDir, "summary-*.json")

	files, err := filepath.Glob(pattern)
	if err != nil {
		return nil, errors.Wrap(err, "globbing workflow summary files")
	}

	if len(files) == 0 {
		log.Warn("no workflow summary files found", "pattern", pattern)
		return []*github.WorkflowSummary{}, nil
	}

	summaries := make([]*github.WorkflowSummary, 0, len(files))

	for _, file := range files {
		log.Debug("reading workflow summary", "file", file)

		//nolint:gosec // file path is controlled by function caller
		data, err := os.ReadFile(file)
		if err != nil {
			return nil, errors.Wrapf(err, "reading file %s", file)
		}

		var summary github.WorkflowSummary
		if err := json.Unmarshal(data, &summary); err != nil {
			return nil, errors.Wrapf(err, "parsing workflow summary from %s", file)
		}

		summaries = append(summaries, &summary)
	}

	return summaries, nil
}

// applyFilterToResults filters results based on status.
func applyFilterToResults(results []any, filter string) []any {
	if filter == filterAll {
		return results
	}

	filtered := make([]any, 0, len(results))

	for _, result := range results {
		status := extractStatus(result)

		switch filter {
		case filterFailures:
			if status == github.StatusFailure {
				filtered = append(filtered, result)
			}
		case filterSuccesses:
			if status == github.StatusSuccess {
				filtered = append(filtered, result)
			}
		case filterSkipped:
			if status == github.StatusSkipped {
				filtered = append(filtered, result)
			}
		}
	}

	return filtered
}

// extractStatus extracts status from any result type.
func extractStatus(result any) github.SyncStatus {
	switch r := result.(type) {
	case *github.LabelsSyncResult:
		return r.Status
	case *github.FilesSyncResult:
		return r.Status
	case *github.SettingsSyncResult:
		return r.Status
	case *github.SmyklotSyncResult:
		return r.Status
	default:
		return ""
	}
}

// generateMarkdown generates markdown summary.
func generateMarkdown(
	log *logger.Logger,
	syncType string,
	results []any,
	output string,
	dryRun bool,
) error {
	var builder strings.Builder

	// Header with emoji
	emoji := getSyncTypeEmoji(syncType)
	fmt.Fprintf(&builder, "# %s %s Sync Summary\n\n", emoji, capitalizeFirst(syncType))

	// Dry-run banner
	if dryRun {
		builder.WriteString("**⚠️ DRY RUN MODE - No changes were applied**\n\n")
	}

	// Stats
	stats := calculateStats(results)

	builder.WriteString("## Summary\n\n")
	fmt.Fprintf(&builder, "- Total repositories: %d\n", stats.total)
	fmt.Fprintf(&builder, "- ✅ Successful: %d\n", stats.success)
	fmt.Fprintf(&builder, "- ❌ Failed: %d\n", stats.failure)
	fmt.Fprintf(&builder, "- ⏭️ Skipped: %d\n", stats.skipped)

	if !stats.startedAt.IsZero() && !stats.completedAt.IsZero() {
		fmt.Fprintf(&builder, "- ⏱️ Duration: %s\n", formatDuration(stats.duration))
	}

	builder.WriteString("\n")

	// Per-repo results
	if len(results) > 0 {
		builder.WriteString("## Repository Results\n\n")

		for _, result := range results {
			formatted := formatResult(syncType, result)
			builder.WriteString(formatted)
			builder.WriteString("\n")
		}
	}

	// Write output
	return writeOutput(log, &builder, output)
}

// generateWorkflowJSON generates workflow summary JSON.
func generateWorkflowJSON(
	log *logger.Logger,
	syncType string,
	results []any,
	output string,
	dryRun bool,
) error {
	// Calculate aggregated stats
	stats := calculateStats(results)

	// Build WorkflowSummary
	summary := github.WorkflowSummary{
		SyncType:       syncType,
		WorkflowRunID:  getEnvInt64("GITHUB_RUN_ID"),
		WorkflowRunURL: getWorkflowRunURL(),
		DryRun:         dryRun,
		StartedAt:      stats.startedAt,
		CompletedAt:    stats.completedAt,
		Duration:       github.Duration(stats.duration),
		TotalRepos:     stats.total,
		SuccessCount:   stats.success,
		FailureCount:   stats.failure,
		SkippedCount:   stats.skipped,
		Results:        results,
	}

	// Serialize to JSON
	data, err := json.MarshalIndent(summary, "", "  ")
	if err != nil {
		return errors.Wrap(err, "marshaling workflow summary")
	}

	// Write to file
	if output == "" {
		return errors.New("output file required for JSON format")
	}

	//nolint:mnd,gosec // 0o644 required for GitHub Actions artifact upload
	err = os.WriteFile(output, data, 0o644)
	if err != nil {
		return errors.Wrap(err, "writing workflow summary")
	}

	log.Info("workflow summary written", "file", output)

	return nil
}

// generateCrossWorkflowMarkdown generates consolidated markdown from multiple workflows.
func generateCrossWorkflowMarkdown(
	log *logger.Logger,
	summaries []*github.WorkflowSummary,
	output string,
	filter string,
	dryRun bool,
) error {
	var builder strings.Builder

	// Header
	builder.WriteString("# 🔄 Consolidated Sync Summary\n\n")

	// Dry-run banner
	if dryRun {
		builder.WriteString("**⚠️ DRY RUN MODE - No changes were applied**\n\n")
	}

	// Overall stats
	overallStats := calculateOverallStats(summaries)
	writeOverallStats(&builder, overallStats)

	// Per-workflow sections
	writeWorkflowSections(&builder, summaries, filter)

	// Write output
	return writeOutput(log, &builder, output)
}

// overallStats holds aggregated stats across workflows.
type overallStats struct {
	totalRepos     int
	totalSuccess   int
	totalFailure   int
	totalSkipped   int
	earliestStart  time.Time
	latestComplete time.Time
}

// calculateOverallStats aggregates statistics from multiple workflows.
func calculateOverallStats(summaries []*github.WorkflowSummary) overallStats {
	stats := overallStats{}

	for _, summary := range summaries {
		stats.totalRepos += summary.TotalRepos
		stats.totalSuccess += summary.SuccessCount
		stats.totalFailure += summary.FailureCount
		stats.totalSkipped += summary.SkippedCount

		if stats.earliestStart.IsZero() || summary.StartedAt.Before(stats.earliestStart) {
			stats.earliestStart = summary.StartedAt
		}

		if stats.latestComplete.IsZero() || summary.CompletedAt.After(stats.latestComplete) {
			stats.latestComplete = summary.CompletedAt
		}
	}

	return stats
}

// writeOverallStats writes overall statistics to builder.
func writeOverallStats(builder *strings.Builder, stats overallStats) {
	builder.WriteString("## Overall Summary\n\n")
	fmt.Fprintf(builder, "- Total repositories: %d\n", stats.totalRepos)
	fmt.Fprintf(builder, "- ✅ Successful: %d\n", stats.totalSuccess)
	fmt.Fprintf(builder, "- ❌ Failed: %d\n", stats.totalFailure)
	fmt.Fprintf(builder, "- ⏭️ Skipped: %d\n", stats.totalSkipped)

	if !stats.earliestStart.IsZero() && !stats.latestComplete.IsZero() {
		duration := stats.latestComplete.Sub(stats.earliestStart)
		fmt.Fprintf(builder, "- ⏱️ Total Duration: %s\n", formatDuration(duration))
	}

	builder.WriteString("\n")
}

// writeWorkflowSections writes per-workflow sections to builder.
func writeWorkflowSections(
	builder *strings.Builder,
	summaries []*github.WorkflowSummary,
	filter string,
) {
	for _, summary := range summaries {
		// Filter workflow results
		filteredResults := applyFilterToResults(summary.Results, filter)

		// Skip if filter removes all results
		if filter != filterAll && len(filteredResults) == 0 {
			continue
		}

		emoji := getSyncTypeEmoji(summary.SyncType)
		fmt.Fprintf(builder, "## %s %s Sync\n\n", emoji, capitalizeFirst(summary.SyncType))

		// Workflow link
		if summary.WorkflowRunURL != "" {
			fmt.Fprintf(builder, "**Workflow:** [View Run](%s)\n\n", summary.WorkflowRunURL)
		}

		// Workflow stats
		fmt.Fprintf(builder, "- Total: %d | ✅ Success: %d | ❌ Failed: %d | ⏭️ Skipped: %d\n",
			summary.TotalRepos, summary.SuccessCount, summary.FailureCount, summary.SkippedCount)
		fmt.Fprintf(
			builder,
			"- ⏱️ Duration: %s\n\n",
			formatDuration(time.Duration(summary.Duration)),
		)

		// Show filtered results
		if len(filteredResults) > 0 {
			builder.WriteString("### Repository Results\n\n")

			for _, result := range filteredResults {
				formatted := formatResult(summary.SyncType, result)
				builder.WriteString(formatted)
				builder.WriteString("\n")
			}
		}
	}
}

// formatResult formats a single result based on sync type.
func formatResult(syncType string, result any) string {
	switch syncType {
	case syncTypeLabels:
		if r, ok := result.(*github.LabelsSyncResult); ok {
			return formatLabelsResult(r)
		}
	case syncTypeFiles:
		if r, ok := result.(*github.FilesSyncResult); ok {
			return formatFilesResult(r)
		}
	case syncTypeSettings:
		if r, ok := result.(*github.SettingsSyncResult); ok {
			return formatSettingsResult(r)
		}
	case syncTypeSmyklot:
		if r, ok := result.(*github.SmyklotSyncResult); ok {
			return formatSmyklotResult(r)
		}
	}

	return ""
}

// formatLabelsResult formats a labels sync result.
func formatLabelsResult(result *github.LabelsSyncResult) string {
	var builder strings.Builder

	statusEmoji := getStatusEmoji(result.Status)
	fmt.Fprintf(&builder, "### %s %s\n\n", statusEmoji, result.Repo)
	fmt.Fprintf(&builder, "**Status:** %s", result.Status)

	if result.Status == github.StatusSkipped {
		fmt.Fprintf(&builder, " (%s)", result.SkippedReason)
	}

	builder.WriteString("\n")

	if result.Status == github.StatusSuccess {
		fmt.Fprintf(&builder, "- Created: %d\n", result.Created)
		fmt.Fprintf(&builder, "- Updated: %d\n", result.Updated)
		fmt.Fprintf(&builder, "- Deleted: %d\n", result.Deleted)
	}

	if result.Status == github.StatusFailure && result.ErrorMessage != "" {
		fmt.Fprintf(&builder, "- **Error:** %s\n", result.ErrorMessage)
	}

	fmt.Fprintf(&builder, "- Duration: %s\n", formatDuration(time.Duration(result.Duration)))

	return builder.String()
}

// formatFilesResult formats a files sync result.
func formatFilesResult(result *github.FilesSyncResult) string {
	var builder strings.Builder

	statusEmoji := getStatusEmoji(result.Status)
	fmt.Fprintf(&builder, "### %s %s\n\n", statusEmoji, result.Repo)
	fmt.Fprintf(&builder, "**Status:** %s", result.Status)

	if result.Status == github.StatusSkipped {
		fmt.Fprintf(&builder, " (%s)", result.SkippedReason)
	}

	builder.WriteString("\n")

	if result.Status == github.StatusSuccess {
		if result.PRURL != "" {
			fmt.Fprintf(&builder, "- **PR:** [#%d](%s)\n", result.PRNumber, result.PRURL)
		}

		fmt.Fprintf(&builder, "- Created: %d files\n", len(result.CreatedFiles))
		fmt.Fprintf(&builder, "- Updated: %d files\n", len(result.UpdatedFiles))
		fmt.Fprintf(&builder, "- Deleted: %d files\n", len(result.DeletedFiles))

		if result.HasDeletionsWarn {
			builder.WriteString("- ⚠️ Contains deletions - review carefully\n")
		}
	}

	if result.Status == github.StatusFailure && result.ErrorMessage != "" {
		fmt.Fprintf(&builder, "- **Error:** %s\n", result.ErrorMessage)
	}

	fmt.Fprintf(&builder, "- Duration: %s\n", formatDuration(time.Duration(result.Duration)))

	return builder.String()
}

// formatSettingsResult formats a settings sync result.
func formatSettingsResult(result *github.SettingsSyncResult) string {
	var builder strings.Builder

	statusEmoji := getStatusEmoji(result.Status)
	fmt.Fprintf(&builder, "### %s %s\n\n", statusEmoji, result.Repo)
	fmt.Fprintf(&builder, "**Status:** %s", result.Status)

	if result.Status == github.StatusSkipped {
		fmt.Fprintf(&builder, " (%s)", result.SkippedReason)
	}

	builder.WriteString("\n")

	if result.Status == github.StatusSuccess {
		fmt.Fprintf(&builder, "- Changes applied: %d\n", result.ChangesApplied)
	}

	if result.Status == github.StatusFailure && result.ErrorMessage != "" {
		fmt.Fprintf(&builder, "- **Error:** %s\n", result.ErrorMessage)
	}

	fmt.Fprintf(&builder, "- Duration: %s\n", formatDuration(time.Duration(result.Duration)))

	return builder.String()
}

// formatSmyklotResult formats a smyklot sync result.
func formatSmyklotResult(result *github.SmyklotSyncResult) string {
	var builder strings.Builder

	statusEmoji := getStatusEmoji(result.Status)
	fmt.Fprintf(&builder, "### %s %s\n\n", statusEmoji, result.Repo)
	fmt.Fprintf(&builder, "**Status:** %s", result.Status)

	if result.Status == github.StatusSkipped {
		fmt.Fprintf(&builder, " (%s)", result.SkippedReason)
	}

	builder.WriteString("\n")

	if result.Status == github.StatusSuccess {
		if result.PRURL != "" {
			fmt.Fprintf(&builder, "- **PR:** [#%d](%s)\n", result.PRNumber, result.PRURL)
		}

		fmt.Fprintf(&builder, "- Installed: %d workflows\n", len(result.InstalledFiles))
		fmt.Fprintf(&builder, "- Replaced: %d workflows\n", len(result.ReplacedFiles))
		fmt.Fprintf(&builder, "- Version-only: %d workflows\n", len(result.VersionOnlyFiles))
	}

	if result.Status == github.StatusFailure && result.ErrorMessage != "" {
		fmt.Fprintf(&builder, "- **Error:** %s\n", result.ErrorMessage)
	}

	fmt.Fprintf(&builder, "- Duration: %s\n", formatDuration(time.Duration(result.Duration)))

	return builder.String()
}

// stats holds aggregated statistics.
type stats struct {
	total       int
	success     int
	failure     int
	skipped     int
	startedAt   time.Time
	completedAt time.Time
	duration    time.Duration
}

// calculateStats aggregates statistics from results.
func calculateStats(results []any) stats {
	s := stats{
		total: len(results),
	}

	for _, result := range results {
		status := extractStatus(result)

		switch status {
		case github.StatusSuccess:
			s.success++
		case github.StatusFailure:
			s.failure++
		case github.StatusSkipped:
			s.skipped++
		}

		// Extract timing
		switch r := result.(type) {
		case *github.LabelsSyncResult:
			s.updateTiming(r.StartedAt, r.CompletedAt)
		case *github.FilesSyncResult:
			s.updateTiming(r.StartedAt, r.CompletedAt)
		case *github.SettingsSyncResult:
			s.updateTiming(r.StartedAt, r.CompletedAt)
		case *github.SmyklotSyncResult:
			s.updateTiming(r.StartedAt, r.CompletedAt)
		}
	}

	if !s.startedAt.IsZero() && !s.completedAt.IsZero() {
		s.duration = s.completedAt.Sub(s.startedAt)
	}

	return s
}

// updateTiming updates earliest start and latest completion times.
func (s *stats) updateTiming(startedAt, completedAt time.Time) {
	if s.startedAt.IsZero() || startedAt.Before(s.startedAt) {
		s.startedAt = startedAt
	}

	if s.completedAt.IsZero() || completedAt.After(s.completedAt) {
		s.completedAt = completedAt
	}
}

// writeOutput writes content to file or GITHUB_STEP_SUMMARY.
func writeOutput(log *logger.Logger, builder *strings.Builder, output string) error {
	content := builder.String()

	// Determine output destination
	outputFile := output
	if outputFile == "" {
		outputFile = os.Getenv("GITHUB_STEP_SUMMARY")
	}

	if outputFile == "" {
		// Write to stdout
		fmt.Print(content)
		return nil
	}

	// Write to file
	//nolint:mnd,gosec // 0o644 required for GitHub Actions artifact upload
	err := os.WriteFile(outputFile, []byte(content), 0o644)
	if err != nil {
		return errors.Wrap(err, "writing output file")
	}

	log.Info("summary written", "file", outputFile)

	return nil
}

// Helper functions

// getEnvInt64 gets an int64 value from environment variable.
func getEnvInt64(key string) int64 {
	val := os.Getenv(key)
	if val == "" {
		return 0
	}

	intVal, err := strconv.ParseInt(val, 10, 64)
	if err != nil {
		return 0
	}

	return intVal
}

// getWorkflowRunURL constructs workflow run URL from environment.
func getWorkflowRunURL() string {
	serverURL := os.Getenv("GITHUB_SERVER_URL")
	repo := os.Getenv("GITHUB_REPOSITORY")
	runID := os.Getenv("GITHUB_RUN_ID")

	if serverURL == "" || repo == "" || runID == "" {
		return ""
	}

	return fmt.Sprintf("%s/%s/actions/runs/%s", serverURL, repo, runID)
}

// getSyncTypeEmoji returns emoji for sync type.
func getSyncTypeEmoji(syncType string) string {
	switch syncType {
	case syncTypeLabels:
		return "🏷️"
	case syncTypeFiles:
		return "📄"
	case syncTypeSettings:
		return "⚙️"
	case syncTypeSmyklot:
		return "🤖"
	default:
		return "🔄"
	}
}

// getStatusEmoji returns emoji for status.
func getStatusEmoji(status github.SyncStatus) string {
	switch status {
	case github.StatusSuccess:
		return "✅"
	case github.StatusFailure:
		return "❌"
	case github.StatusSkipped:
		return "⏭️"
	default:
		return "❓"
	}
}

// capitalizeFirst capitalizes first letter of string.
func capitalizeFirst(s string) string {
	if s == "" {
		return s
	}

	return strings.ToUpper(s[:1]) + s[1:]
}

// formatDuration formats duration in human-readable format.
func formatDuration(d time.Duration) string {
	if d == 0 {
		return "0s"
	}

	// Round to seconds
	seconds := int(d.Seconds())

	if seconds < secondsPerMinute {
		return fmt.Sprintf("%ds", seconds)
	}

	minutes := seconds / secondsPerMinute
	remainingSeconds := seconds % secondsPerMinute

	if minutes < minutesPerHour {
		if remainingSeconds > 0 {
			return fmt.Sprintf("%dm %ds", minutes, remainingSeconds)
		}

		return fmt.Sprintf("%dm", minutes)
	}

	hours := minutes / minutesPerHour
	remainingMinutes := minutes % minutesPerHour

	if remainingMinutes > 0 {
		return fmt.Sprintf("%dh %dm", hours, remainingMinutes)
	}

	return fmt.Sprintf("%dh", hours)
}

func init() {
	// Configure summary generate command flags
	summaryGenerateCmd.Flags().String("type", "", "Sync type (labels|files|settings|smyklot|all)")
	summaryGenerateCmd.Flags().String("results-dir", "", "Directory containing result JSON files")
	summaryGenerateCmd.Flags().String(
		"output",
		"",
		"Output file path (defaults to $GITHUB_STEP_SUMMARY for markdown)",
	)
	summaryGenerateCmd.Flags().String("format", "markdown", "Output format (markdown|json)")
	summaryGenerateCmd.Flags().String(
		"filter",
		"all",
		"Filter results (all|failures|successes|skipped)",
	)

	// Add subcommands
	summaryCmd.AddCommand(summaryGenerateCmd)

	// Add to root command (this will be done in main.go init)
	rootCmd.AddCommand(summaryCmd)
}
