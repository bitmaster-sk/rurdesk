import { describe, it, expect, beforeEach } from 'vitest';
import { TestBed } from '@angular/core/testing';
import { BrowserTitleService, DEFAULT_TITLE, buildIssueTitle, truncateIssueTitle } from './browser-title.service';
import { Issue } from 'src/app/issue/model/issue.model';

const makeIssue = (overrides: Partial<Issue> = {}): Issue => ({
    idProject: 1,
    idIssue: 10,
    idIssuePublic: 5,
    title: 'X',
    idState: 1,
    idSeverity: null,
    description: '',
    tracked: 0,
    ...overrides
});

describe('BrowserTitleService', () => {
    beforeEach(() => {
        document.title = DEFAULT_TITLE;
    });

    it('builds the issue title with number first', () => {
        expect(buildIssueTitle(makeIssue())).toBe('#5 X · RuRdesk');
    });

    it('truncates long titles and keeps the issue number visible', () => {
        const longTitle = 'a'.repeat(80);
        const result = buildIssueTitle(makeIssue({ title: longTitle }));
        expect(result.startsWith('#5 ')).toBe(true);
        expect(result.endsWith('… · RuRdesk')).toBe(true);
    });

    it('does not truncate short titles', () => {
        expect(truncateIssueTitle('Short title')).toBe('Short title');
    });

    it('sets the default title', () => {
        const service = TestBed.inject(BrowserTitleService);
        document.title = 'other';
        service.setDefault();
        expect(document.title).toBe(DEFAULT_TITLE);
    });

    it('sets the issue title', () => {
        const service = TestBed.inject(BrowserTitleService);
        service.setIssueTitle(makeIssue({ title: 'My issue' }));
        expect(document.title).toBe('#5 My issue · RuRdesk');
    });

    it('falls back to default when issue has no public id', () => {
        const service = TestBed.inject(BrowserTitleService);
        service.setIssueTitle(makeIssue({ idIssuePublic: 0 }));
        expect(document.title).toBe(DEFAULT_TITLE);
    });
});
