package github

import (
	"time"
)

// SyncStatus represents the outcome status of a sync operation.
type SyncStatus string

const (
	StatusSuccess SyncStatus = "success"
	StatusFailure SyncStatus = "failure"
	StatusSkipped SyncStatus = "skipped"
)

// SyncResult is the base result type for all sync operations.
type SyncResult struct {
	Repo          string        `json:"repo"`
	Status        SyncStatus    `json:"status"`
	DryRun        bool          `json:"dry_run"`
	StartedAt     time.Time     `json:"started_at"`
	CompletedAt   time.Time     `json:"completed_at"`
	Duration      time.Duration `json:"duration"`
	SkippedReason string        `json:"skipped_reason,omitempty"`
	ErrorMessage  string        `json:"error_message,omitempty"`
}

// LabelsSyncResult extends SyncResult with labels-specific fields.
type LabelsSyncResult struct {
	SyncResult
	Created int `json:"created"`
	Updated int `json:"updated"`
	Deleted int `json:"deleted"`
}

// FilesSyncResult extends SyncResult with files-specific fields.
type FilesSyncResult struct {
	SyncResult
	PRNumber         int      `json:"pr_number,omitempty"`
	PRURL            string   `json:"pr_url,omitempty"`
	CreatedFiles     []string `json:"created_files,omitempty"`
	UpdatedFiles     []string `json:"updated_files,omitempty"`
	DeletedFiles     []string `json:"deleted_files,omitempty"`
	HasDeletionsWarn bool     `json:"has_deletions_warn,omitempty"`
}

// SettingsSyncResult extends SyncResult with settings-specific fields.
type SettingsSyncResult struct {
	SyncResult
	ChangesApplied int `json:"changes_applied"`
}

// SmyklotSyncResult extends SyncResult with smyklot-specific fields.
type SmyklotSyncResult struct {
	SyncResult
	PRNumber         int      `json:"pr_number,omitempty"`
	PRURL            string   `json:"pr_url,omitempty"`
	InstalledFiles   []string `json:"installed_files,omitempty"`
	ReplacedFiles    []string `json:"replaced_files,omitempty"`
	VersionOnlyFiles []string `json:"version_only_files,omitempty"`
}

// WorkflowSummary aggregates results from a single workflow run.
type WorkflowSummary struct {
	SyncType       string        `json:"sync_type"`
	WorkflowRunID  int64         `json:"workflow_run_id"`
	WorkflowRunURL string        `json:"workflow_run_url"`
	DryRun         bool          `json:"dry_run"`
	StartedAt      time.Time     `json:"started_at"`
	CompletedAt    time.Time     `json:"completed_at"`
	Duration       time.Duration `json:"duration"`
	TotalRepos     int           `json:"total_repos"`
	SuccessCount   int           `json:"success_count"`
	FailureCount   int           `json:"failure_count"`
	SkippedCount   int           `json:"skipped_count"`
	Results        []any         `json:"results"`
}

// NewLabelsSyncResult creates a new LabelsSyncResult with initialized timing.
func NewLabelsSyncResult(repo string, dryRun bool) *LabelsSyncResult {
	return &LabelsSyncResult{
		SyncResult: SyncResult{
			Repo:      repo,
			DryRun:    dryRun,
			StartedAt: time.Now(),
		},
	}
}

// NewFilesSyncResult creates a new FilesSyncResult with initialized timing.
func NewFilesSyncResult(repo string, dryRun bool) *FilesSyncResult {
	return &FilesSyncResult{
		SyncResult: SyncResult{
			Repo:      repo,
			DryRun:    dryRun,
			StartedAt: time.Now(),
		},
	}
}

// NewSettingsSyncResult creates a new SettingsSyncResult with initialized timing.
func NewSettingsSyncResult(repo string, dryRun bool) *SettingsSyncResult {
	return &SettingsSyncResult{
		SyncResult: SyncResult{
			Repo:      repo,
			DryRun:    dryRun,
			StartedAt: time.Now(),
		},
	}
}

// NewSmyklotSyncResult creates a new SmyklotSyncResult with initialized timing.
func NewSmyklotSyncResult(repo string, dryRun bool) *SmyklotSyncResult {
	return &SmyklotSyncResult{
		SyncResult: SyncResult{
			Repo:      repo,
			DryRun:    dryRun,
			StartedAt: time.Now(),
		},
	}
}

// Complete finalizes the result with completion time and status.
func (r *SyncResult) Complete(status SyncStatus) {
	r.CompletedAt = time.Now()
	r.Duration = r.CompletedAt.Sub(r.StartedAt)
	r.Status = status
}

// CompleteWithError finalizes the result with failure status and error message.
func (r *SyncResult) CompleteWithError(err error) {
	r.Complete(StatusFailure)

	if err != nil {
		r.ErrorMessage = err.Error()
	}
}

// CompleteSkipped finalizes the result with skipped status and reason.
func (r *SyncResult) CompleteSkipped(reason string) {
	r.Complete(StatusSkipped)
	r.SkippedReason = reason
}
