import { Component, forwardRef, input, output } from '@angular/core';
import { ComponentFixture, TestBed } from '@angular/core/testing';
import { ControlValueAccessor, FormsModule, NG_VALUE_ACCESSOR } from '@angular/forms';
import { ActivatedRoute, RouterModule, convertToParamMap, provideRouter } from '@angular/router';
import { TranslateModule } from '@ngx-translate/core';
import { Subject, of } from 'rxjs';
import { User } from 'src/app/auth/model/user.model';
import { ProjectMemberStore } from 'src/app/project/project-member.store';
import { NoticeService } from 'src/app/shared/notice/notice.service';
import { TablerIconStub, UiLoaderStub, UiTagStub } from 'src/testing/stubs';
import { WikiApi } from '../../api/wiki.api.service';
import { WikiProposalKind } from '../../constants/wiki-proposal-kind.enum';
import { WikiProposalState } from '../../constants/wiki-proposal-state.enum';
import { WikiSpaceKind } from '../../constants/wiki-space-kind.enum';
import { WikiProposal } from '../../model/wiki-proposal.model';
import { WikiProposalsPage } from './wiki-proposals.page';

@Component({ selector: 'app-wiki-proposal-detail', template: '', standalone: true })
class WikiProposalDetailStub {
    public readonly idProject = input<number>(0);
    public readonly idProposal = input<number>(0);
    public readonly decided = output<WikiProposal>();
}

@Component({ selector: 'app-wiki-tree-show', template: '', standalone: true })
class WikiTreeShowStub {}

@Component({
    selector: 'ui-choice',
    template: '',
    standalone: true,
    providers: [
        { provide: NG_VALUE_ACCESSOR, useExisting: forwardRef(() => UiChoiceStub), multi: true }
    ]
})
class UiChoiceStub implements ControlValueAccessor {
    public readonly options = input<unknown[]>([]);
    public readonly optionLabel = input<string>('');
    public readonly optionValue = input<string>('');
    public readonly size = input<string>('default');
    public readonly allowEmpty = input<boolean>(true);
    public onChange: (value: unknown) => void = () => undefined;

    public writeValue(): void {
        return;
    }

    public registerOnChange(fn: (value: unknown) => void): void {
        this.onChange = fn;
    }

    public registerOnTouched(): void {
        return;
    }
}

function proposal(
    idProposal: number,
    state: WikiProposalState,
    spaceKind = WikiSpaceKind.Project
): WikiProposal {
    return {
        idProposal,
        idRun: 19,
        idUserAgent: 3,
        idIssue: 175,
        idIssuePublic: 175,
        issueTitle: 'Reuse the branch',
        idProject: 7,
        idSpace: 2,
        spaceKind,
        kind: WikiProposalKind.Update,
        idPage: 40,
        slug: `page-${idProposal}`,
        title: `Page ${idProposal}`,
        summary: '',
        body: 'new',
        idParent: null,
        parentSlug: null,
        parentTitle: null,
        reason: 'r',
        baseVersion: 1,
        agentAccess: null,
        state,
        decidedBy: null,
        decidedAt: null,
        decisionNote: null,
        resultVersion: null,
        pageVersion: 1,
        pageTitle: `Page ${idProposal}`,
        pageParentTitle: null,
        isPageLive: true,
        createAt: '2026-10-08T10:00:00Z',
        updateAt: '2026-10-08T10:00:00Z'
    };
}

describe('WikiProposalsPage', () => {
    let fixture: ComponentFixture<WikiProposalsPage>;
    let notices: Subject<{ payload: { idProject: number; idIssue: number } }>;
    let loadOpen: ReturnType<typeof vi.fn>;

    beforeEach(async () => {
        notices = new Subject();
        loadOpen = vi.fn(() =>
            of([
                proposal(1, WikiProposalState.Open),
                proposal(2, WikiProposalState.Ready),
                proposal(3, WikiProposalState.Ready, WikiSpaceKind.Instance)
            ])
        );
        const params = convertToParamMap({ idProject: '7' });
        await TestBed.configureTestingModule({
            declarations: [WikiProposalsPage],
            imports: [
                TranslateModule.forRoot(),
                RouterModule,
                FormsModule,
                TablerIconStub,
                UiLoaderStub,
                UiTagStub,
                UiChoiceStub,
                WikiProposalDetailStub,
                WikiTreeShowStub
            ],
            providers: [
                provideRouter([]),
                {
                    provide: ActivatedRoute,
                    useValue: { snapshot: { paramMap: params }, paramMap: of(params) }
                },
                { provide: WikiApi, useValue: { loadOpenProposals$: loadOpen } },
                { provide: NoticeService, useValue: { wikiProposal$: notices.asObservable() } },
                {
                    provide: ProjectMemberStore,
                    useValue: {
                        usersMap$: of(
                            new Map<number, User>([
                                [3, { idUser: 3, name: 'Kimi', email: '', colorAvatarBg: '' }]
                            ])
                        )
                    }
                }
            ]
        }).compileComponents();
        fixture = TestBed.createComponent(WikiProposalsPage);
        fixture.detectChanges();
    });

    function titles(): string[] {
        return Array.from(
            (fixture.nativeElement as HTMLElement).querySelectorAll(
                '[data-testid="wiki-proposal-item"] b'
            )
        ).map(item => item.textContent?.trim() ?? '');
    }

    function selected(): number | undefined {
        return fixture.debugElement
            .query(debug => debug.name === 'app-wiki-proposal-detail')
            ?.injector.get(WikiProposalDetailStub)
            .idProposal();
    }

    it('lists the ready proposals before the ones waiting for the merge and opens the first ready one', () => {
        expect(titles()).toEqual(['Page 2', 'Page 3', 'Page 1']);
        expect(selected()).toBe(2);
        expect((fixture.nativeElement as HTMLElement).textContent).toContain('Kimi');
    });

    it('filters the list by space', () => {
        fixture.debugElement
            .query(debug => debug.name === 'ui-choice')
            .injector.get(UiChoiceStub)
            .onChange(WikiSpaceKind.Instance);
        fixture.detectChanges();

        expect(titles()).toEqual(['Page 3']);
        expect(selected()).toBe(3);
    });

    it('reloads when a proposal of this project changes', async () => {
        notices.next({ payload: { idProject: 99, idIssue: 1 } });
        notices.next({ payload: { idProject: 7, idIssue: 1 } });
        await new Promise(resolve => setTimeout(resolve, 350));

        expect(loadOpen).toHaveBeenCalledTimes(2);
    });
});
