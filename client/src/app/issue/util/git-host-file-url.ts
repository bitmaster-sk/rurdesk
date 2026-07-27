import { HostType } from 'src/app/project/model/git-integration.model';

/**
 * Builds a web URL pointing at the file-diff view of a merge/pull request on
 * the source host. We deliberately link to the MR/PR diff page rather than
 * the file at `headSha` so opening the link drops the user into the same
 * change view they're inspecting locally (with inline comments, viewed
 * state, etc.), instead of the post-merge file blob where the diff context
 * is gone.
 *
 * Per-file anchors (e.g. GitHub's `#diff-<sha256(path)>`) would be a nice
 * deep-link addition but require async hashing — skipped for now: every
 * file's icon points to the MR files index and the host UI handles
 * scrolling once the user is there.
 *
 * Path layouts:
 *   - GitHub: `{base}/{repo}/pull/{mrId}/files`
 *   - GitLab: `{base}/{repo}/-/merge_requests/{mrId}/diffs`
 *   - Gitea:  `{base}/{repo}/pulls/{mrId}/files`
 */
export function buildGitHostMrFilesUrl(
    hostType: HostType,
    baseUrl: string,
    repoPath: string,
    mrId: string
): string {
    const base = baseUrl.replace(/\/+$/, '');
    const repo = repoPath.replace(/^\/+|\/+$/g, '');
    const id = encodeURIComponent(mrId);
    switch (hostType) {
        case HostType.GitHub:
            return `${base}/${repo}/pull/${id}/files`;
        case HostType.GitLab:
            return `${base}/${repo}/-/merge_requests/${id}/diffs`;
        case HostType.Gitea:
            return `${base}/${repo}/pulls/${id}/files`;
    }
}
