import { TestBed } from '@angular/core/testing';
import { provideRouter, RouterModule } from '@angular/router';
import { TranslateModule } from '@ngx-translate/core';
import { describe, expect, it } from 'vitest';
import { WikiProposalKind } from '../../constants/wiki-proposal-kind.enum';
import { WikiProposalState } from '../../constants/wiki-proposal-state.enum';
import { WikiSpaceKind } from '../../constants/wiki-space-kind.enum';
import { WikiProposal } from '../../model/wiki-proposal.model';
import { WikiProposalOriginComponent } from './wiki-proposal-origin.component';

function proposal(overrides: Partial<WikiProposal>): WikiProposal {
    return {
        idProposal: 5,
        idRun: 19,
        idUserAgent: 3,
        createBy: 3,
        authorName: 'Kimi',
        idIssue: 175,
        idIssuePublic: 175,
        issueTitle: 'Reuse the branch',
        idProject: 7,
        idSpace: 2,
        spaceKind: WikiSpaceKind.Project,
        kind: WikiProposalKind.Update,
        idPage: 40,
        slug: 'agent-run',
        title: 'Agent run',
        summary: '',
        body: 'new text',
        idParent: null,
        parentSlug: null,
        parentTitle: null,
        reason: 'the branch name changed',
        baseVersion: 4,
        agentAccess: null,
        state: WikiProposalState.Ready,
        decidedBy: null,
        decidedAt: null,
        decisionNote: null,
        resultVersion: null,
        pageVersion: 4,
        pageTitle: 'Agent run',
        pageParentTitle: null,
        isPageLive: true,
        createAt: '2026-10-08T10:00:00Z',
        updateAt: '2026-10-08T10:00:00Z',
        ...overrides
    };
}

async function render(value: WikiProposal, linkIssue = false): Promise<HTMLElement> {
    await TestBed.configureTestingModule({
        declarations: [WikiProposalOriginComponent],
        imports: [TranslateModule.forRoot(), RouterModule],
        providers: [provideRouter([])]
    }).compileComponents();
    const fixture = TestBed.createComponent(WikiProposalOriginComponent);
    fixture.componentRef.setInput('proposal', value);
    fixture.componentRef.setInput('linkIssue', linkIssue);
    fixture.detectChanges();
    return fixture.nativeElement as HTMLElement;
}

describe('WikiProposalOriginComponent', () => {
    it('names the task, the agent and the run of an agent proposal', async () => {
        const el = await render(proposal({}));

        expect(el.textContent).toContain('#175');
        expect(el.textContent).toContain('Kimi');
        expect(el.textContent).toContain('WIKI.PROPOSAL.RUN');
        expect(el.querySelector('a')).toBeNull();
    });

    it('links the task with its title when asked to', async () => {
        const el = await render(proposal({}), true);

        const link = el.querySelector('[data-testid="wiki-proposal-origin-issue"]');
        expect(link?.getAttribute('href')).toBe('/project/7/issue/175');
        expect(link?.textContent).toContain('Reuse the branch');
    });

    it('names only the author of a proposal made without a run or a task', async () => {
        const el = await render(
            proposal({
                idRun: null,
                idIssue: null,
                idIssuePublic: null,
                issueTitle: null,
                authorName: 'Tomas'
            }),
            true
        );

        expect(el.textContent?.trim()).toBe('Tomas');
        expect(el.querySelector('a')).toBeNull();
    });
});
