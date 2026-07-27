import { HostType } from 'src/app/project/model/git-integration.model';
import { prMrLinkTitleKey, prMrTermKey } from './pr-mr-term';

describe('prMrTermKey', () => {
    it('maps GitHub to a github-specific key', () => {
        expect(prMrTermKey(HostType.GitHub)).toBe('GIT_INTEGRATION.PR_MR.BY_HOST.GITHUB');
    });

    it('maps GitLab to a gitlab-specific key', () => {
        expect(prMrTermKey(HostType.GitLab)).toBe('GIT_INTEGRATION.PR_MR.BY_HOST.GITLAB');
    });

    it('maps Gitea to a gitea-specific key', () => {
        expect(prMrTermKey(HostType.Gitea)).toBe('GIT_INTEGRATION.PR_MR.BY_HOST.GITEA');
    });

    it('falls back to a generic key for null host', () => {
        expect(prMrTermKey(null)).toBe('GIT_INTEGRATION.PR_MR.BY_HOST.GENERIC');
    });

    it('falls back to a generic key for undefined host', () => {
        expect(prMrTermKey(undefined)).toBe('GIT_INTEGRATION.PR_MR.BY_HOST.GENERIC');
    });
});

describe('prMrLinkTitleKey', () => {
    it('produces distinct keys per host', () => {
        const seen = new Set([
            prMrLinkTitleKey(HostType.GitHub),
            prMrLinkTitleKey(HostType.GitLab),
            prMrLinkTitleKey(HostType.Gitea),
            prMrLinkTitleKey(null)
        ]);
        expect(seen.size).toBe(4);
    });
});
