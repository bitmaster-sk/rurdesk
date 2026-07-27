import { HostType } from 'src/app/project/model/git-integration.model';

/**
 * Returns the i18n key for the "merge request" / "pull request" label that
 * matches the given git host's own terminology. GitHub and Gitea call them
 * pull requests; GitLab calls them merge requests. When the host is unknown
 * (issue not yet linked, runtime data missing), we fall back to a neutral
 * "Change request" label so the UI doesn't pick a side it doesn't have
 * evidence for.
 *
 * All i18n keys live under `GIT_INTEGRATION.PR_MR.BY_HOST.*` (label only)
 * and `GIT_INTEGRATION.PR_MR.LINK_TITLE_BY_HOST.*` (dialog headers etc.)
 * so adding a new host means adding one entry per locale and one case here.
 */
export function prMrTermKey(hostType: HostType | null | undefined): string {
    switch (hostType) {
        case HostType.GitHub:
            return 'GIT_INTEGRATION.PR_MR.BY_HOST.GITHUB';
        case HostType.GitLab:
            return 'GIT_INTEGRATION.PR_MR.BY_HOST.GITLAB';
        case HostType.Gitea:
            return 'GIT_INTEGRATION.PR_MR.BY_HOST.GITEA';
        default:
            return 'GIT_INTEGRATION.PR_MR.BY_HOST.GENERIC';
    }
}

/**
 * Returns the i18n key for a dialog title / header that names the host's
 * change-request flow (e.g. "Link pull request" vs "Link merge request").
 * Same host mapping as prMrTermKey; consumers translate independently.
 */
export function prMrLinkTitleKey(hostType: HostType | null | undefined): string {
    switch (hostType) {
        case HostType.GitHub:
            return 'GIT_INTEGRATION.PR_MR.LINK_TITLE_BY_HOST.GITHUB';
        case HostType.GitLab:
            return 'GIT_INTEGRATION.PR_MR.LINK_TITLE_BY_HOST.GITLAB';
        case HostType.Gitea:
            return 'GIT_INTEGRATION.PR_MR.LINK_TITLE_BY_HOST.GITEA';
        default:
            return 'GIT_INTEGRATION.PR_MR.LINK_TITLE_BY_HOST.GENERIC';
    }
}
