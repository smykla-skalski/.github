#!/usr/bin/env bash

set -euo pipefail

repository="${1:?repository is required}"
dry_run="${2:-false}"

migrations=(
  'smyklot:pending-ci|smyklot:pending:ci'
  'smyklot:pending-ci:squash|smyklot:pending:ci:squash'
  'smyklot:pending-ci:rebase|smyklot:pending:ci:rebase'
  'smyklot:pending-ci:required|smyklot:pending:ci:required'
  'smyklot:pending-ci:squash:required|smyklot:pending:ci:squash:required'
  'smyklot:pending-ci:rebase:required|smyklot:pending:ci:rebase:required'
)

label_path() {
  jq -rn --arg label "$1" '$label | @uri'
}

label_exists() {
  local label
  label="$(label_path "$1")"
  gh api --silent "repos/${repository}/labels/${label}" >/dev/null 2>&1
}

merge_label_assignments() {
  local old_label="$1"
  local new_label="$2"
  local issue

  while IFS= read -r issue; do
    [[ -n "$issue" ]] || continue
    jq -nc --arg label "$new_label" '{labels: [$label]}' |
      gh api --silent --method POST "repos/${repository}/issues/${issue}/labels" --input -
  done < <(
    gh api --paginate --method GET "repos/${repository}/issues" \
      -f state=all -f labels="$old_label" -f per_page=100 --jq '.[].number'
  )
}

for migration in "${migrations[@]}"; do
  old_label="${migration%%|*}"
  new_label="${migration#*|}"
  if ! label_exists "$old_label"; then
    continue
  fi
  if [[ "$dry_run" == "true" ]]; then
    echo "would migrate ${repository}: ${old_label} -> ${new_label}"
    continue
  fi

  old_path="$(label_path "$old_label")"
  if label_exists "$new_label"; then
    merge_label_assignments "$old_label" "$new_label"
    gh api --silent --method DELETE "repos/${repository}/labels/${old_path}"
    echo "merged ${repository}: ${old_label} -> ${new_label}"
  else
    gh api --silent --method PATCH "repos/${repository}/labels/${old_path}" \
      -f new_name="$new_label"
    echo "renamed ${repository}: ${old_label} -> ${new_label}"
  fi
done
