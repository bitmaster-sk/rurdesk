import { HostType } from 'src/app/project/model/git-integration.model';
import { GitHostTerminology } from './git-host-terminology';

describe('GitHostTerminology.termKey', () => {
    it('maps GitHub to a github-specific key', () => {
        expect(GitHostTerminology.termKey(HostType.GitHub)).toBe(
            'GIT_INTEGRATION.PR_MR.BY_HOST.GITHUB'
        );
    });

    it('maps GitLab to a gitlab-specific key', () => {
        expect(GitHostTerminology.termKey(HostType.GitLab)).toBe(
            'GIT_INTEGRATION.PR_MR.BY_HOST.GITLAB'
        );
    });

    it('maps Gitea to a gitea-specific key', () => {
        expect(GitHostTerminology.termKey(HostType.Gitea)).toBe(
            'GIT_INTEGRATION.PR_MR.BY_HOST.GITEA'
        );
    });

    it('falls back to a generic key for null host', () => {
        expect(GitHostTerminology.termKey(null)).toBe('GIT_INTEGRATION.PR_MR.BY_HOST.GENERIC');
    });

    it('falls back to a generic key for undefined host', () => {
        expect(GitHostTerminology.termKey(undefined)).toBe('GIT_INTEGRATION.PR_MR.BY_HOST.GENERIC');
    });
});

describe('GitHostTerminology.linkTitleKey', () => {
    it('produces distinct keys per host', () => {
        const seen = new Set([
            GitHostTerminology.linkTitleKey(HostType.GitHub),
            GitHostTerminology.linkTitleKey(HostType.GitLab),
            GitHostTerminology.linkTitleKey(HostType.Gitea),
            GitHostTerminology.linkTitleKey(null)
        ]);
        expect(seen.size).toBe(4);
    });
});
