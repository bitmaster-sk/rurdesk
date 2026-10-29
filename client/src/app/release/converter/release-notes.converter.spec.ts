import { ReleaseNotesConverter } from './release-notes.converter';

describe('ReleaseNotesConverter', () => {
    it('writes one heading per section and one bullet per task with its number', () => {
        const markdown = ReleaseNotesConverter.toMarkdown(
            [
                {
                    idIssueType: 2,
                    name: 'Feature',
                    issues: [
                        {
                            idIssue: 1,
                            idIssuePublic: 104,
                            title: 'Release notes',
                            idIssueType: 2,
                            issueTypeName: 'Feature'
                        },
                        {
                            idIssue: 2,
                            idIssuePublic: 105,
                            title: 'Dark mode',
                            idIssueType: 2,
                            issueTypeName: 'Feature'
                        }
                    ]
                },
                {
                    idIssueType: null,
                    name: null,
                    issues: [
                        {
                            idIssue: 3,
                            idIssuePublic: 7,
                            title: 'Bump deps',
                            idIssueType: null,
                            issueTypeName: null
                        }
                    ]
                }
            ],
            'Other'
        );

        expect(markdown).toBe(
            '## Feature\n\n- Release notes (#104)\n- Dark mode (#105)\n\n## Other\n\n- Bump deps (#7)\n'
        );
    });

    it('is empty when there is nothing to release', () => {
        expect(ReleaseNotesConverter.toMarkdown([], 'Other')).toBe('');
    });
});
