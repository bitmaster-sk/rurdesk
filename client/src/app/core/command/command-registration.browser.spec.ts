import { describe, it, expect, vi } from 'vitest';
import { BehaviorSubject, of } from 'rxjs';
import { Router } from '@angular/router';
import { TestBed } from '@angular/core/testing';
import { TranslateService } from '@ngx-translate/core';
import { IssueSearchCommandProvider } from '../../issue/command/issue-search.command-provider';
import { IssueActionCommandProvider } from '../../issue/command/issue-action.command-provider';
import { NavigationCommandProvider } from '../../project/command/navigation.command-provider';
import { PeopleCommandProvider } from '../../project/command/people.command-provider';
import { IssueService } from '../../issue/issue.service';
import { StateStore } from '../../state/store/state.store';
import { SeverityStore } from '../../severity/store/severity.store';
import { AclStore } from '../../project/store/acl.store';
import { ProjectService } from '../../project/project.service';
import { ProjectMemberStore } from '../../project/project-member.store';
import { UserService } from '../../auth/user.service';
import { NoticeService } from '../../shared/notice/notice.service';
import { CommandPaletteService } from './command-palette.service';
import { Role } from '../../shared/constants/role.enum';
import { Issue } from '../../issue/model/issue.model';

const t = { instant: (k: string, p?: any) => (p ? `${k}:${JSON.stringify(p)}` : k) };
const issues: Issue[] = [
    {
        idIssue: 42,
        idIssuePublic: 428,
        idProject: 1,
        title: 'Login',
        idState: 1,
        idSeverity: null,
        description: '',
        tracked: 0
    }
];

describe('IssueSearchCommandProvider', () => {
    function setup(opts: { states?: any[]; role?: Role; insert?: any } = {}) {
        const router = { navigate: vi.fn() };
        const issueService = {
            loadIssues: vi.fn(() => of(issues)),
            insertIssue: opts.insert ?? vi.fn(() => of({ idIssuePublic: 99, idProject: 1 }))
        };
        TestBed.configureTestingModule({
            providers: [
                IssueSearchCommandProvider,
                AclStore,
                { provide: Router, useValue: router },
                { provide: IssueService, useValue: issueService },
                { provide: StateStore, useValue: { states$: of(opts.states ?? []) } },
                { provide: TranslateService, useValue: t }
            ]
        });
        TestBed.inject(AclStore).setRole(opts.role ?? Role.Member);
        return { provider: TestBed.inject(IssueSearchCommandProvider), router, issueService };
    }

    it('offers jump commands for the primed project', () => {
        const { provider } = setup();
        provider.prime({ idProject: 1, issue: null }).subscribe();
        expect(provider.getCommands({ idProject: 1, issue: null }).map(c => c.id)).toContain(
            'issue.jump.428'
        );
    });

    it('returns no jumps for a different project than the primed cache (no stale cross-project links)', () => {
        const { provider } = setup();
        provider.prime({ idProject: 1, issue: null }).subscribe();
        expect(provider.getCommands({ idProject: 2, issue: null })).toEqual([]);
    });

    it('creates with the START state (selected by flag, not array order) and navigates to the new issue', () => {
        const insert = vi.fn(() => of({ idIssuePublic: 99, idProject: 1 }));
        const { provider, router } = setup({
            role: Role.Member,
            insert,
            states: [
                {
                    idState: 1,
                    idProject: 1,
                    name: 'Backlog',
                    start: false,
                    final: false,
                    protected: false,
                    orderRank: 0
                },
                {
                    idState: 5,
                    idProject: 1,
                    name: 'Todo',
                    start: true,
                    final: false,
                    protected: false,
                    orderRank: 9
                }
            ]
        });
        provider.createFromQuery('bug', { idProject: 1, issue: null })!.run();
        expect(insert).toHaveBeenCalledWith(
            expect.objectContaining({
                idProject: 1,
                title: 'bug',
                description: 'bug',
                idState: 5,
                idSeverity: null
            })
        );
        expect(router.navigate).toHaveBeenCalledWith(['/project', 1, 'issue', 99]);
    });

    it('still creates (idState null) and navigates when states are unloaded (Option A, no silent no-op)', () => {
        const insert = vi.fn(() => of({ idIssuePublic: 99, idProject: 1 }));
        const { provider, router } = setup({ role: Role.Member, insert, states: [] });
        provider.createFromQuery('x', { idProject: 1, issue: null })!.run();
        expect(insert).toHaveBeenCalledWith(expect.objectContaining({ idState: null }));
        expect(router.navigate).toHaveBeenCalledWith(['/project', 1, 'issue', 99]);
    });

    it('offers no create command for a viewer', () => {
        const { provider } = setup({ role: Role.Viewer });
        expect(provider.createFromQuery('x', { idProject: 1, issue: null })).toBeNull();
    });
});

describe('NavigationCommandProvider', () => {
    function setup(role: Role) {
        const router = { navigate: vi.fn() };
        TestBed.configureTestingModule({
            providers: [
                NavigationCommandProvider,
                AclStore,
                { provide: Router, useValue: router },
                {
                    provide: ProjectService,
                    useValue: { loadProjects: vi.fn(() => of([{ idProject: 7, name: 'Website' }])) }
                },
                { provide: CommandPaletteService, useValue: { openHelp: vi.fn() } },
                {
                    provide: UserService,
                    useValue: {
                        user: new BehaviorSubject(null),
                        logout: () => of(null),
                        deleteAuthLocal: vi.fn()
                    }
                },
                { provide: TranslateService, useValue: t }
            ]
        });
        TestBed.inject(AclStore).setRole(role);
        return TestBed.inject(NavigationCommandProvider);
    }

    it('omits settings and create for a viewer', () => {
        const ids = setup(Role.Viewer)
            .getCommands({ idProject: 3, issue: null })
            .map(c => c.id);
        expect(ids).not.toContain('nav.settings');
        expect(ids).not.toContain('global.create');
    });

    it('offers settings and create for an owner', () => {
        const ids = setup(Role.Owner)
            .getCommands({ idProject: 3, issue: null })
            .map(c => c.id);
        expect(ids).toContain('nav.settings');
        expect(ids).toContain('global.create');
    });
});

describe('IssueActionCommandProvider', () => {
    const issue: Issue = {
        idIssue: 15,
        idProject: 1,
        idIssuePublic: 5,
        title: 'Login',
        description: 'd',
        idState: 3,
        idSeverity: 2,
        assignedTo: 7,
        tracked: 0
    } as Issue;
    function setup(states: any[] = []) {
        const router = { navigate: vi.fn() };
        const insertIssue = vi.fn(() => of({ idIssuePublic: 99, idProject: 1 }));
        const updateIssue = vi.fn((i: Issue) => of(i));
        const emitIssue = vi.fn();
        TestBed.configureTestingModule({
            providers: [
                IssueActionCommandProvider,
                AclStore,
                { provide: Router, useValue: router },
                { provide: IssueService, useValue: { insertIssue, updateIssue } },
                { provide: StateStore, useValue: { states$: of(states) } },
                { provide: SeverityStore, useValue: { severities$: of([]) } },
                { provide: ProjectMemberStore, useValue: { users$: of([]) } },
                { provide: UserService, useValue: { user: new BehaviorSubject(null) } },
                { provide: NoticeService, useValue: { emitIssue } },
                { provide: TranslateService, useValue: t }
            ]
        });
        TestBed.inject(AclStore).setRole(Role.Member);
        return {
            provider: TestBed.inject(IssueActionCommandProvider),
            router,
            insertIssue,
            updateIssue,
            emitIssue
        };
    }

    it('offers a clone command that copies the open task and navigates to the new one', () => {
        const { provider, router, insertIssue } = setup();
        provider
            .getCommands({ idProject: 1, issue })
            .find(c => c.id === 'issue.clone')!
            .run();
        expect(insertIssue).toHaveBeenCalledWith(
            expect.objectContaining({
                idProject: 1,
                title: 'ISSUE.COPY_SUFFIX:{"title":"Login"}',
                description: 'd',
                idState: 3,
                idSeverity: 2,
                assignedTo: 7
            })
        );
        expect(router.navigate).toHaveBeenCalledWith(['/project', 1, 'issue', 99]);
    });

    it('emits the saved task after a state patch so the open detail refreshes', () => {
        const state = {
            idState: 11,
            idProject: 1,
            name: 'Done',
            start: false,
            final: true,
            protected: false,
            orderRank: 9
        };
        const { provider, updateIssue, emitIssue } = setup([state]);
        provider
            .getCommands({ idProject: 1, issue })
            .find(c => c.id === 'issue.state.11')!
            .run();
        expect(updateIssue).toHaveBeenCalledWith(expect.objectContaining({ idState: 11 }));
        expect(emitIssue).toHaveBeenCalled();
    });
});

describe('PeopleCommandProvider', () => {
    const member = { idUser: 9, name: 'Petra', email: 'p@x' };
    function setup() {
        const router = { navigate: vi.fn() };
        const updateIssue = vi.fn((i: Issue) => of(i));
        const emitIssue = vi.fn();
        TestBed.configureTestingModule({
            providers: [
                PeopleCommandProvider,
                AclStore,
                { provide: Router, useValue: router },
                { provide: ProjectMemberStore, useValue: { users$: of([member]) } },
                { provide: IssueService, useValue: { updateIssue } },
                { provide: NoticeService, useValue: { emitIssue } },
                { provide: TranslateService, useValue: t }
            ]
        });
        TestBed.inject(AclStore).setRole(Role.Member);
        return { provider: TestBed.inject(PeopleCommandProvider), router, updateIssue, emitIssue };
    }

    it('assigns the picked person to the open issue on a detail and emits the change (not navigate)', () => {
        const issue = { idProject: 1, idIssuePublic: 5, title: 'X' } as Issue;
        const { provider, router, updateIssue, emitIssue } = setup();
        provider.getCommands({ idProject: 1, issue })[0].run();
        expect(updateIssue).toHaveBeenCalledWith(expect.objectContaining({ assignedTo: 9 }));
        expect(emitIssue).toHaveBeenCalled(); // open detail refreshes at once
        expect(router.navigate).not.toHaveBeenCalled();
    });

    it('navigates to the table when there is no open issue (list context)', () => {
        const { provider, router, updateIssue } = setup();
        provider.getCommands({ idProject: 1, issue: null })[0].run();
        expect(updateIssue).not.toHaveBeenCalled();
        expect(router.navigate).toHaveBeenCalledWith(['/project', 1, 'issue', 'view', 'table']);
    });
});
