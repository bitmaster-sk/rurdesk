import { DOCUMENT } from '@angular/common';
import { Injectable, inject } from '@angular/core';
import { Issue } from 'src/app/issue/model/issue.model';

export const DEFAULT_TITLE = 'RuRdesk';
export const ISSUE_TITLE_MAX_LENGTH = 60;

@Injectable({ providedIn: 'root' })
export class BrowserTitleService {
    private readonly document = inject(DOCUMENT);

    public setDefault(): void {
        this.setTitle(DEFAULT_TITLE);
    }

    public setIssueTitle(issue: Issue): void {
        if (!issue.idIssuePublic) {
            this.setDefault();
            return;
        }
        this.setTitle(buildIssueTitle(issue));
    }

    private setTitle(title: string): void {
        this.document.title = title;
    }
}

export function buildIssueTitle(issue: Issue): string {
    const title = truncateIssueTitle(issue.title);
    return `#${issue.idIssuePublic} ${title} · ${DEFAULT_TITLE}`;
}

export function truncateIssueTitle(title: string | null | undefined, maxLength = ISSUE_TITLE_MAX_LENGTH): string {
    const safeTitle = title ?? '';
    if (safeTitle.length <= maxLength) {
        return safeTitle;
    }
    return `${safeTitle.slice(0, maxLength)}…`;
}
