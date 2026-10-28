import { describe, expect, it } from 'vitest';
import { IssueLinkConverter } from './issue-link.converter';

describe('IssueLinkConverter', () => {
    it('turns a task reference into an issue link with the project route', () => {
        expect(IssueLinkConverter.toMarkdown('fixed in #11 (and #12)', 7)).toBe(
            'fixed in [#11](/project/7/issue/11) (and [#12](/project/7/issue/12))'
        );
    });

    it('requires a boundary before the hash so headings stay plain', () => {
        expect(IssueLinkConverter.toMarkdown('# Title\nsee #11', 7)).toBe(
            '# Title\nsee [#11](/project/7/issue/11)'
        );
    });

    it('leaves code blocks and inline code untouched', () => {
        const body = '`#5` and\n```\n#5 code\n```\nsee #6';
        expect(IssueLinkConverter.toMarkdown(body, 7)).toBe(
            '`#5` and\n```\n#5 code\n```\nsee [#6](/project/7/issue/6)'
        );
    });

    it('does not convert numbers inside existing markdown links and anchors', () => {
        expect(IssueLinkConverter.toMarkdown('[year](#2026) and [x](/issue/8)', 7)).toBe(
            '[year](#2026) and [x](/issue/8)'
        );
    });

    it('builds a standalone issue link carrying the task project', () => {
        expect(
            IssueLinkConverter.toLinkMarkdown({ idIssuePublic: 12, idProject: 7, title: 'Fix' })
        ).toBe('[#12](/project/7/issue/12)');
    });

    it('filters picker options by the typed number and caps the list', () => {
        const options = [1, 12, 123, 1234, 2, 21].map(idIssuePublic => ({
            idIssuePublic,
            idProject: 7,
            title: `Task ${idIssuePublic}`
        }));
        expect(IssueLinkConverter.toPickerOptions(options, '12')).toEqual([
            { idIssuePublic: 12, idProject: 7, title: 'Task 12' },
            { idIssuePublic: 123, idProject: 7, title: 'Task 123' },
            { idIssuePublic: 1234, idProject: 7, title: 'Task 1234' }
        ]);
        expect(
            IssueLinkConverter.toPickerOptions(options, '').map(option => option.idIssuePublic)
        ).toEqual([1, 12, 123, 1234, 2, 21]);
        expect(IssueLinkConverter.toPickerOptions(options, '9')).toEqual([]);
        expect(
            IssueLinkConverter.toPickerOptions(
                Array.from({ length: 12 }, (_, index) => ({
                    idIssuePublic: index + 1,
                    idProject: 7,
                    title: `Task ${index + 1}`
                })),
                ''
            )
        ).toHaveLength(8);
    });
});
