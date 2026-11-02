export enum RunKindName {
    Issue = 'issue',
    ReleaseNotes = 'release_notes'
}

export const RUN_KIND_LABELS: Record<string, string> = {
    [RunKindName.Issue]: 'AGENT.RUN_KIND.ISSUE',
    [RunKindName.ReleaseNotes]: 'AGENT.RUN_KIND.RELEASE_NOTES'
};
