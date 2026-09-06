import { describe, it, expect, beforeEach } from 'vitest';
import { TestBed } from '@angular/core/testing';
import { BrowserTitleService, DEFAULT_TITLE } from './browser-title.service';
import { Issue } from 'src/app/issue/model/issue.model';
import { Fixtures } from 'src/testing/fixtures';

const makeIssue = (overrides: Partial<Issue> = {}): Issue =>
    Fixtures.issue({ idIssue: 10, idIssuePublic: 5, title: 'X', idState: 1, ...overrides });

describe('BrowserTitleService', () => {
    beforeEach(() => {
        document.title = DEFAULT_TITLE;
    });

    it('puts the issue number first in the title', () => {
        TestBed.inject(BrowserTitleService).setIssueTitle(makeIssue());
        expect(document.title).toBe('#5 X · RuRdesk');
    });

    it('truncates long titles and keeps the issue number visible', () => {
        TestBed.inject(BrowserTitleService).setIssueTitle(makeIssue({ title: 'a'.repeat(80) }));
        expect(document.title.startsWith('#5 ')).toBe(true);
        expect(document.title.endsWith('… · RuRdesk')).toBe(true);
    });

    it('does not truncate short titles', () => {
        TestBed.inject(BrowserTitleService).setIssueTitle(makeIssue({ title: 'Short title' }));
        expect(document.title).toBe('#5 Short title · RuRdesk');
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
