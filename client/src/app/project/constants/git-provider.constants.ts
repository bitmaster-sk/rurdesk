import { HostType } from '../model/git-integration.model';

export interface GitProvider {
    value: HostType;
    label: string;
    icon: string;
}

/** Single source of truth for git-host presentation (label + brand icon).
 *  Icons are navy-tinted mono/duotone SVGs served from src/assets, matching
 *  --ui-color-text — rendered as-is (no currentColor tint) to keep gitea's
 *  white teabag and gitlab's shades. Used by the settings select and list. */
export const GIT_PROVIDERS: readonly GitProvider[] = [
    { value: HostType.GitHub, label: 'GitHub', icon: 'assets/image/icons/github.svg' },
    { value: HostType.GitLab, label: 'GitLab', icon: 'assets/image/icons/gitlab.svg' },
    { value: HostType.Gitea, label: 'Gitea', icon: 'assets/image/icons/gitea.svg' }
];

export const GIT_PROVIDER_BY_TYPE = Object.fromEntries(
    GIT_PROVIDERS.map(provider => [provider.value, provider])
) as Record<HostType, GitProvider>;
