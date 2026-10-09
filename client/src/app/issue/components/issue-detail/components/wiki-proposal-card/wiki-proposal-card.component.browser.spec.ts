import { ComponentFixture, TestBed } from '@angular/core/testing';
import { provideRouter, RouterModule } from '@angular/router';
import { TranslateModule } from '@ngx-translate/core';
import { By } from '@angular/platform-browser';
import { DatePipe } from '@angular/common';
import { AvatarStub, TablerIconStub, UiButtonStub, UiTagStub } from 'src/testing/stubs';
import { WikiProposalKind } from 'src/app/wiki/constants/wiki-proposal-kind.enum';
import { WikiProposalState } from 'src/app/wiki/constants/wiki-proposal-state.enum';
import { WikiSpaceKind } from 'src/app/wiki/constants/wiki-space-kind.enum';
import { WikiProposal } from 'src/app/wiki/model/wiki-proposal.model';
import { WikiProposalCardComponent } from './wiki-proposal-card.component';

function proposal(overrides: Partial<WikiProposal> = {}): WikiProposal {
    return {
        idProposal: 5,
        idRun: 19,
        idUserAgent: 3,
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
        body: 'new',
        idParent: null,
        parentSlug: null,
        parentTitle: null,
        reason: 'The branch name is not deterministic any more.',
        baseVersion: 4,
        agentAccess: null,
        state: WikiProposalState.Open,
        decidedBy: null,
        decidedAt: null,
        decisionNote: null,
        resultVersion: null,
        pageVersion: 4,
        pageTitle: 'Agent run',
        pageParentTitle: null,
        isPageLive: true,
        createAt: '2026-10-08T10:00:00Z',
        updateAt: '2026-10-08T11:00:00Z',
        ...overrides
    };
}

describe('WikiProposalCardComponent', () => {
    let fixture: ComponentFixture<WikiProposalCardComponent>;

    beforeEach(async () => {
        await TestBed.configureTestingModule({
            declarations: [WikiProposalCardComponent],
            imports: [
                TranslateModule.forRoot(),
                RouterModule,
                DatePipe,
                AvatarStub,
                TablerIconStub,
                UiButtonStub,
                UiTagStub
            ],
            providers: [provideRouter([])]
        }).compileComponents();
        fixture = TestBed.createComponent(WikiProposalCardComponent);
    });

    function render(value: WikiProposal): HTMLElement {
        fixture.componentRef.setInput('proposal', value);
        fixture.componentRef.setInput('agent', {
            idUser: 3,
            name: 'Kimi',
            email: '',
            colorAvatarBg: ''
        });
        fixture.detectChanges();
        return fixture.nativeElement as HTMLElement;
    }

    it('shows what the agent proposes, why, and links to the page', () => {
        const el = render(proposal());

        expect(fixture.debugElement.query(By.directive(AvatarStub)).componentInstance.name()).toBe(
            'Kimi'
        );
        expect(el.querySelector('.author')?.textContent).toContain('Kimi');
        expect(el.querySelector('.kind-badge')?.textContent).toContain('WIKI.PROPOSAL.CARD');
        expect(el.textContent).toContain('WIKI.PROPOSAL.KIND.UPDATE');
        expect(el.textContent).toContain('The branch name is not deterministic any more.');
        expect(el.querySelector('a.page')?.getAttribute('href')).toBe(
            '/project/7/wiki/project/agent-run'
        );
        expect(el.querySelector('[data-testid="wiki-proposal-card-state"]')?.textContent).toContain(
            'WIKI.PROPOSAL.STATE.OPEN'
        );
    });

    it('names a page that does not exist yet without linking it', () => {
        const el = render(
            proposal({
                kind: WikiProposalKind.Create,
                idPage: null,
                isPageLive: false,
                title: 'Retention',
                state: WikiProposalState.Ready
            })
        );

        expect(el.querySelector('a.page')).toBeNull();
        expect(el.querySelector('span.page')?.textContent).toContain('Retention');
        expect(el.querySelector('[data-testid="wiki-proposal-card-state"]')?.textContent).toContain(
            'WIKI.PROPOSAL.STATE.READY'
        );
    });
});
