import { TestBed } from '@angular/core/testing';
import { ActivatedRoute, Router, convertToParamMap } from '@angular/router';
import { TranslateModule } from '@ngx-translate/core';
import { Subject, of, throwError } from 'rxjs';
import { AgentRunApi } from 'src/app/agent/api/agent-run.api.service';
import { AgentPhase } from 'src/app/agent/model/agent-phase.enum';
import { User } from 'src/app/auth/model/user.model';
import { ProjectMemberStore } from 'src/app/project/project-member.store';
import { AclStore } from 'src/app/project/store/acl.store';
import { I18nService } from 'src/app/shared/i18n/i18n.service';
import { NoticeAction } from 'src/app/shared/notice/constant/notice-action.enum';
import { NoticeService } from 'src/app/shared/notice/notice.service';
import { TagStore } from 'src/app/tag/store/tag.store';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { ReleaseApi } from '../../api/release.api.service';
import { ReleaseTaskService } from '../../service/release-task.service';
import { ReleaseNotesMode } from '../../constants/release-notes-mode.enum';
import { ReleaseOpenIssues } from '../../constants/release-open-issues.enum';
import { ReleaseStatus } from '../../constants/release-status.enum';
import { ReleaseStep } from '../../constants/release-step.enum';
import { ReleaseDraft, ReleaseView } from '../../model/release.model';
import { ReleasePage } from './release.page';
import { Fixtures } from 'src/testing/fixtures';

const MEMBERS: User[] = [
    { idUser: 1, name: 'Owner', email: 'o@x', colorAvatarBg: '#000' },
    { idUser: 4, name: 'Bot', email: 'bot@x', colorAvatarBg: '#000', isAgent: true }
];

const DRAFT: ReleaseView = {
    idRelease: 5,
    idProject: 3,
    idTag: 9,
    name: 'v1.1',
    status: ReleaseStatus.Draft,
    notes: '',
    recreateTag: true,
    openIssues: ReleaseOpenIssues.Move,
    issueCount: 0,
    createAt: '2026-10-27T10:00:00Z',
    createBy: 1,
    releasedAt: null,
    releasedBy: null,
    tagName: 'next',
    idRunNotes: null,
    notesRunPhase: null,
    notesRunError: null,
    isNotesPending: false
};

const TASKS: ReleaseDraft = {
    sections: [
        {
            idIssueType: 2,
            name: 'Feature',
            issues: [
                {
                    idIssue: 20,
                    idIssuePublic: 104,
                    title: 'Release notes',
                    idIssueType: 2,
                    issueTypeName: 'Feature'
                }
            ]
        }
    ],
    openIssues: []
};

describe('ReleasePage', () => {
    let api: Record<string, ReturnType<typeof vi.fn>>;
    let runApi: Record<string, ReturnType<typeof vi.fn>>;
    let loadRows$: ReturnType<typeof vi.fn>;
    let releaseNotices: Subject<unknown>;
    let navigate: ReturnType<typeof vi.fn>;
    let canUpdateProject: boolean;

    beforeEach(() => {
        vi.useFakeTimers();
        api = {
            loadOne$: vi.fn().mockReturnValue(of(DRAFT)),
            loadDraft$: vi.fn().mockReturnValue(of(TASKS)),
            update$: vi.fn().mockImplementation((_p, _r, body) => of({ ...DRAFT, ...body })),
            publish$: vi
                .fn()
                .mockReturnValue(of({ ...DRAFT, status: ReleaseStatus.Released, notes: 'x' })),
            startAgent$: vi
                .fn()
                .mockReturnValue(of({ ...DRAFT, isNotesPending: true, idRunNotes: 30 })),
            updateNotes$: vi
                .fn()
                .mockImplementation((_p, _r, notes) =>
                    of({ ...DRAFT, status: ReleaseStatus.Released, notes })
                )
        };
        runApi = {
            loadOne$: vi.fn().mockReturnValue(of(Fixtures.agentRun({ idRun: 30, idIssue: null }))),
            cancel$: vi.fn().mockReturnValue(of(Fixtures.agentRun({ idRun: 30, idIssue: null })))
        };
        loadRows$ = vi.fn().mockReturnValue(of([]));
        releaseNotices = new Subject();
        navigate = vi.fn().mockResolvedValue(true);
        canUpdateProject = true;
    });

    afterEach(() => vi.useRealTimers());

    async function setup(step: ReleaseStep = ReleaseStep.Version) {
        await TestBed.configureTestingModule({
            imports: [TranslateModule.forRoot()],
            declarations: [ReleasePage],
            providers: [
                { provide: ReleaseApi, useValue: api },
                { provide: AgentRunApi, useValue: runApi },
                { provide: ReleaseTaskService, useValue: { loadRows$ } },
                {
                    provide: ActivatedRoute,
                    useValue: {
                        snapshot: {
                            paramMap: convertToParamMap({ idProject: '3', idRelease: '5' }),
                            queryParamMap: convertToParamMap({ step: String(step) })
                        }
                    }
                },
                { provide: Router, useValue: { navigate } },
                { provide: I18nService, useValue: { instant: (key: string) => key } },
                { provide: AclStore, useValue: { canUpdateProject: () => canUpdateProject } },
                { provide: ProjectMemberStore, useValue: { users$: of(MEMBERS) } },
                { provide: TagStore, useValue: { tagsByProject$: () => of([]) } },
                {
                    provide: NoticeService,
                    useValue: {
                        release$: releaseNotices.asObservable(),
                        agentRun$: new Subject(),
                        agentThinking$: new Subject()
                    }
                }
            ]
        })
            .overrideComponent(ReleasePage, { set: { template: '' } })
            .compileComponents();
        const fixture = TestBed.createComponent(ReleasePage);
        fixture.detectChanges();
        return { page: fixture.componentInstance, fixture };
    }

    it('fills the form from the draft without saving it again', async () => {
        const { page } = await setup();
        vi.advanceTimersByTime(1000);

        expect(api.loadOne$).toHaveBeenCalledWith(3, 5);
        expect(page['form'].getRawValue().name).toBe('v1.1');
        expect(page['agents']().map(user => user.name)).toEqual(['Bot']);
        expect(api.update$).not.toHaveBeenCalled();
    });

    it('saves the draft a moment after an edit', async () => {
        const { page } = await setup();

        page['form'].controls.name.setValue(' v1.2 ');
        page['form'].controls.openIssues.setValue(ReleaseOpenIssues.Keep);
        vi.advanceTimersByTime(600);

        expect(api.update$).toHaveBeenCalledTimes(1);
        expect(api.update$).toHaveBeenCalledWith(3, 5, {
            name: 'v1.2',
            recreateTag: true,
            openIssues: ReleaseOpenIssues.Keep
        });
    });

    it('sends the notes only when the user changed them', async () => {
        const { page } = await setup();

        page['form'].controls.notes.setValue('## Fixes');
        vi.advanceTimersByTime(600);
        page['form'].controls.recreateTag.setValue(false);
        vi.advanceTimersByTime(600);

        expect(api.update$.mock.calls[0][2]).toMatchObject({ notes: '## Fixes' });
        expect(api.update$.mock.calls[1][2]).not.toHaveProperty('notes');
    });

    it('asks how to write the notes while they are empty', async () => {
        const { page } = await setup(ReleaseStep.Notes);

        expect(page['notesMode']()).toBe(ReleaseNotesMode.Choose);
    });

    it('starts the notes from the closed tasks when the user writes them', async () => {
        const { page } = await setup(ReleaseStep.Notes);

        page['onWriteMyself']();
        vi.advanceTimersByTime(600);

        expect(page['notesMode']()).toBe(ReleaseNotesMode.Editor);
        expect(api.update$).toHaveBeenCalledWith(
            3,
            5,
            expect.objectContaining({
                notes: '## Feature\n\n- Release notes (#104)\n'
            })
        );
    });

    it('opens the editor right away when the draft has notes', async () => {
        api.loadOne$.mockReturnValue(of({ ...DRAFT, notes: '## Fixes' }));
        const { page } = await setup(ReleaseStep.Notes);

        expect(page['notesMode']()).toBe(ReleaseNotesMode.Editor);
    });

    it('says what still blocks the release', async () => {
        const { page } = await setup();

        expect(page['publishBlocker']()).toBe('RELEASE.BLOCKER.NOTES');
        page['form'].controls.notes.setValue('## Fixes');
        expect(page['publishBlocker']()).toBeNull();
        page['form'].controls.name.setValue(' ');
        expect(page['publishBlocker']()).toBe('RELEASE.BLOCKER.NAME');
        page['release'].set({ ...DRAFT, isNotesPending: true });
        expect(page['publishBlocker']()).toBe('RELEASE.BLOCKER.AGENT');
    });

    it('blocks the release of a version named like its tag while the tag is created again', async () => {
        api.loadOne$.mockReturnValue(of({ ...DRAFT, notes: '## Fixes' }));
        const { page } = await setup();

        page['form'].controls.name.setValue('Next');
        expect(page['isNameSameAsTag']()).toBe(true);
        expect(page['publishBlocker']()).toBe('RELEASE.BLOCKER.SAME_NAME');
        page['form'].controls.recreateTag.setValue(false);
        expect(page['publishBlocker']()).toBeNull();
    });

    it('takes the notes the agent wrote without saving them back', async () => {
        api.loadOne$
            .mockReturnValueOnce(of({ ...DRAFT, isNotesPending: true, idRunNotes: 30 }))
            .mockReturnValueOnce(
                of({
                    ...DRAFT,
                    notes: '## From the agent',
                    idRunNotes: 30,
                    notesRunPhase: AgentPhase.Done
                })
            );
        const { page } = await setup(ReleaseStep.Notes);
        expect(page['isNotesPending']()).toBe(true);
        expect(runApi.loadOne$).toHaveBeenCalledWith(30);

        releaseNotices.next({ action: NoticeAction.Update, payload: DRAFT });
        vi.advanceTimersByTime(300);

        expect(page['isNotesPending']()).toBe(false);
        expect(page['notes']()).toBe('## From the agent');
        expect(page['notesMode']()).toBe(ReleaseNotesMode.Editor);
        vi.advanceTimersByTime(1000);
        expect(api.update$).not.toHaveBeenCalled();
    });

    it('saves open changes before it releases', async () => {
        const { page } = await setup();
        page['form'].controls.notes.setValue('## Fixes');

        page['onPublish']();

        expect(api.update$).toHaveBeenCalledTimes(1);
        expect(api.publish$).toHaveBeenCalledWith(3, 5);
        expect(api.update$.mock.invocationCallOrder[0]).toBeLessThan(
            api.publish$.mock.invocationCallOrder[0]
        );
        expect(page['isDraft']()).toBe(false);
    });

    it('does not release when the open changes fail to save', async () => {
        api.update$.mockReturnValue(throwError(() => new Error('409')));
        const { page } = await setup();
        page['form'].controls.notes.setValue('## Fixes');

        page['onPublish']();

        expect(api.publish$).not.toHaveBeenCalled();
        expect(page['isBusy']()).toBe(false);
        expect(page['isDraft']()).toBe(true);
    });

    it('keeps following the release after one reload failed', async () => {
        const { page } = await setup();
        api.loadOne$
            .mockReturnValueOnce(throwError(() => new Error('502')))
            .mockReturnValueOnce(of({ ...DRAFT, notes: '## Later' }));

        releaseNotices.next({ action: NoticeAction.Update, payload: DRAFT });
        vi.advanceTimersByTime(300);
        releaseNotices.next({ action: NoticeAction.Update, payload: DRAFT });
        vi.advanceTimersByTime(300);

        expect(page['notes']()).toBe('## Later');
    });

    it('shows a draft read-only to a member who cannot change the project', async () => {
        canUpdateProject = false;
        const { page } = await setup(ReleaseStep.Notes);

        expect(page['canEdit']()).toBe(false);
        expect(page['form'].disabled).toBe(true);
        page['form'].controls.name.setValue('v9');
        vi.advanceTimersByTime(600);
        expect(api.update$).not.toHaveBeenCalled();
    });

    it('blocks the release of a draft whose tag was deleted', async () => {
        api.loadOne$.mockReturnValue(of({ ...DRAFT, idTag: null, notes: '## Fixes' }));
        const { page } = await setup(ReleaseStep.Tasks);

        expect(page['isTagDeleted']()).toBe(true);
        expect(page['publishBlocker']()).toBe('RELEASE.BLOCKER.TAG');
        expect(api.loadDraft$).not.toHaveBeenCalled();
    });

    it('lists the tasks of a released version and none of a draft', async () => {
        const { page } = await setup();
        expect(loadRows$).not.toHaveBeenCalled();
        expect(page['tasks']()).toBeNull();

        api.loadOne$.mockReturnValue(of({ ...DRAFT, status: ReleaseStatus.Released, notes: 'x' }));
        page['load']();

        expect(loadRows$).toHaveBeenCalledWith(3, 9);
        expect(page['tasks']()).toEqual([]);
    });

    it('saves edited notes of a released version and leaves the editor', async () => {
        api.loadOne$.mockReturnValue(of({ ...DRAFT, status: ReleaseStatus.Released, notes: 'x' }));
        const { page } = await setup();
        page['onEditNotes']();

        page['onSaveNotes']('## Fixed');

        expect(api.updateNotes$).toHaveBeenCalledWith(3, 5, '## Fixed');
        expect(api.update$).not.toHaveBeenCalled();
        expect(page['isEditingNotes']()).toBe(false);
        expect(page['release']()?.notes).toBe('## Fixed');
    });

    it('starts the agent with the chosen run and follows its run', async () => {
        const { page, fixture } = await setup(ReleaseStep.Notes);
        page['onWriteWithAgent']();

        page['onAgentStarted']({ idUserAgent: 4 });
        fixture.detectChanges();

        expect(api.startAgent$).toHaveBeenCalledWith(3, 5, { idUserAgent: 4 });
        expect(page['isNotesPending']()).toBe(true);
        expect(runApi.loadOne$).toHaveBeenCalledWith(30);
        expect(page['notesRun']()?.idRun).toBe(30);
    });

    it('stops the writing agent', async () => {
        api.loadOne$
            .mockReturnValueOnce(of({ ...DRAFT, isNotesPending: true, idRunNotes: 30 }))
            .mockReturnValueOnce(
                of({ ...DRAFT, idRunNotes: 30, notesRunPhase: AgentPhase.Cancelled })
            );
        const { page } = await setup(ReleaseStep.Notes);

        page['onStopAgent']();

        expect(runApi.cancel$).toHaveBeenCalledWith(30);
        expect(page['isNotesPending']()).toBe(false);
        expect(page['notesMode']()).toBe(ReleaseNotesMode.Choose);
    });

    it('tells the user the agent failed until they hide it', async () => {
        api.loadOne$.mockReturnValue(
            of({
                ...DRAFT,
                idRunNotes: 30,
                notesRunPhase: AgentPhase.Failed,
                notesRunError: 'boom'
            })
        );
        const { page } = await setup(ReleaseStep.Notes);

        expect(page['notesRunError']()).toBe('boom');
        page['onDismissRunError']();
        expect(page['notesRunError']()).toBeNull();
    });

    it('goes back to the list when the release is deleted elsewhere', async () => {
        await setup();

        releaseNotices.next({ action: NoticeAction.Delete, payload: DRAFT });

        expect(navigate).toHaveBeenCalledWith(['/project', 3, 'release']);
    });
});
