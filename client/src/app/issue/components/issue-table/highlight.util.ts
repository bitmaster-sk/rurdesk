/**
 * Resolve the row index of the highlighted issue by its stable public id.
 *
 * Keyboard selection is tracked by `idIssuePublic` (a stable identity), NOT by
 * position — so when the row list is replaced/re-ordered (a websocket edit, a
 * sort change, a filter refresh) the highlight follows the same task instead of
 * pointing at whatever now sits at the old index. Returns null when nothing is
 * selected or the selected task is no longer present (e.g. filtered out).
 */
export function resolveHighlightIndex(
    rows: readonly { issue: { idIssuePublic?: number } }[],
    idIssuePublic: number | null
): number | null {
    if (idIssuePublic === null) return null;
    const index = rows.findIndex(row => row.issue.idIssuePublic === idIssuePublic);
    return index === -1 ? null : index;
}
