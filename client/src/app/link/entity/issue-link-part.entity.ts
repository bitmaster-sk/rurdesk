export type IssueLinkPart =
    { type: 'text'; text: string } | { type: 'issue'; idProject: number; idIssuePublic: number };
