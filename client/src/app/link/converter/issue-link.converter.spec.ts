import { describe, expect, it } from 'vitest';
import { IssueLinkConverter } from './issue-link.converter';

describe('IssueLinkConverter', () => {
    it('builds a markdown link that shows only the task number and carries its project', () => {
        expect(IssueLinkConverter.toLinkMarkdown({ idProject: 7, idIssuePublic: 12 })).toBe(
            '[#12](/project/7/issue/12)'
        );
    });

    it('reads the task reference back from a link target', () => {
        expect(IssueLinkConverter.toRef('/project/7/issue/12')).toEqual({
            idProject: 7,
            idIssuePublic: 12
        });
    });

    it('does not treat other app routes or external links as task links', () => {
        expect(IssueLinkConverter.toRef('/project/7/issue/12/edit')).toBeNull();
        expect(IssueLinkConverter.toRef('/project/7/wiki/12')).toBeNull();
        expect(IssueLinkConverter.toRef('https://example.com/project/7/issue/12')).toBeNull();
    });

    it('splits text into plain parts and task links', () => {
        expect(
            IssueLinkConverter.toParts(
                'see [#12](/project/7/issue/12) and [#3](/project/9/issue/3)'
            )
        ).toEqual([
            { type: 'text', text: 'see ' },
            { type: 'issue', idProject: 7, idIssuePublic: 12 },
            { type: 'text', text: ' and ' },
            { type: 'issue', idProject: 9, idIssuePublic: 3 }
        ]);
    });

    it('keeps a bare task number and a relabeled link as plain text', () => {
        expect(
            IssueLinkConverter.toParts(
                '#12 and [the bug](/project/7/issue/12) and [#5](/project/7/issue/6)'
            )
        ).toEqual([
            {
                type: 'text',
                text: '#12 and [the bug](/project/7/issue/12) and [#5](/project/7/issue/6)'
            }
        ]);
    });
});
