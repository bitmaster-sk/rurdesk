import { GitHostUrl } from './git-host-file-url';
import { HostType } from 'src/app/project/model/git-integration.model';

describe('GitHostUrl.buildMrFilesUrl', () => {
    it('builds the GitHub PR files URL', () => {
        expect(
            GitHostUrl.buildMrFilesUrl(HostType.GitHub, 'https://github.com', 'acme/repo', '42')
        ).toBe('https://github.com/acme/repo/pull/42/files');
    });

    it('builds the GitLab merge request diffs URL', () => {
        expect(
            GitHostUrl.buildMrFilesUrl(HostType.GitLab, 'https://gitlab.com', 'acme/repo', '42')
        ).toBe('https://gitlab.com/acme/repo/-/merge_requests/42/diffs');
    });

    it('builds the Gitea pulls files URL', () => {
        expect(
            GitHostUrl.buildMrFilesUrl(HostType.Gitea, 'https://gitea.local', 'acme/repo', '42')
        ).toBe('https://gitea.local/acme/repo/pulls/42/files');
    });

    it('trims surrounding slashes from base URL and repo path', () => {
        expect(
            GitHostUrl.buildMrFilesUrl(HostType.GitHub, 'https://github.com/', '/acme/repo/', '42')
        ).toBe('https://github.com/acme/repo/pull/42/files');
    });

    it('URL-encodes the MR id', () => {
        expect(
            GitHostUrl.buildMrFilesUrl(HostType.GitHub, 'https://github.com', 'acme/repo', 'a b')
        ).toBe('https://github.com/acme/repo/pull/a%20b/files');
    });
});
