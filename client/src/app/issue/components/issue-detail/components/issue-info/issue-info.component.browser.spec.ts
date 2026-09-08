import { TestBed } from '@angular/core/testing';
import { Router } from '@angular/router';
import { NEVER, Subject, of } from 'rxjs';
import { AuthStore } from 'src/app/auth/store/auth.store';
import { IssueTypeStore } from 'src/app/issue-type/store/issue-type.store';
import { GitIntegrationApi } from 'src/app/project/api/git-integration.api.service';
import { ProjectMemberStore } from 'src/app/project/project-member.store';
import { ProjectStore } from 'src/app/project/project.store';
import { SeverityStore } from 'src/app/severity/store/severity.store';
import { StateStore } from 'src/app/state/store/state.store';
import { NoticeService } from 'src/app/shared/notice/notice.service';
import { PinService } from 'src/app/pin/pin.service';
import { MrDiffApi } from 'src/app/issue/api/mr-diff.api.service';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { IssueService } from '../../../../issue.service';
import { Issue } from '../../../../model/issue.model';
import { IssueInfoComponent } from './issue-info.component';
import { CiStatus, MrState, MrStatus } from 'src/app/project/model/git-integration.model';
import { MrStatusNotice } from 'src/app/shared/notice/model/mr-status-notice.model';

const ISSUE: Issue = {
    idIssue: 10,
    idIssuePublic: 42,
    idProject: 7,
    title: 'Some issue',
    description: 'body',
    assignedTo: null,
    estimated: 0,
    idGitIntegration: 3,
    mrId: 'mr-123'
} as unknown as Issue;

describe('IssueInfoComponent — live MR status notice (browser)', () => {
    let mrStatus$: Subject<unknown>;
    let getStatus$: ReturnType<typeof vi.fn>;
    let getIntegration$: ReturnType<typeof vi.fn>;

    beforeEach(async () => {
        mrStatus$ = new Subject<unknown>();
        getStatus$ = vi.fn(() => NEVER);
        getIntegration$ = vi.fn(() => NEVER);
        await TestBed.configureTestingModule({
            declarations: [IssueInfoComponent],
            providers: [
                {
                    provide: IssueService,
                    useValue: { updateIssue: vi.fn().mockReturnValue(of(ISSUE)) }
                },
                { provide: StateStore, useValue: { statesByProject$: () => of([]) } },
                { provide: SeverityStore, useValue: { severitiesByProject$: () => of([]) } },
                { provide: IssueTypeStore, useValue: { issueTypesByProject$: () => of([]) } },
                {
                    provide: ProjectMemberStore,
                    useValue: { users$: of([]), usersMap$: of(new Map()) }
                },
                { provide: ProjectStore, useValue: { project$: of({ idProject: 7, name: 'p' }) } },
                { provide: AuthStore, useValue: { getUser: () => ({ idUser: 1 }) } },
                { provide: PinService, useValue: { insertPin: () => NEVER } },
                {
                    provide: MrDiffApi,
                    useValue: { getStatus$, getDiff$: () => NEVER }
                },
                { provide: GitIntegrationApi, useValue: { loadOne$: getIntegration$ } },
                { provide: Router, useValue: { navigate: vi.fn() } },
                { provide: NoticeService, useValue: { mrStatus$: mrStatus$.asObservable() } }
            ]
        })
            .overrideComponent(IssueInfoComponent, { set: { template: '' } })
            .compileComponents();
    });

    it('patches the linked MR badge from a matching mr_status notice', () => {
        const fixture = TestBed.createComponent(IssueInfoComponent);
        fixture.componentRef.setInput('issue', ISSUE);
        fixture.detectChanges();

        const notice: MrStatusNotice = {
            idIssue: 10,
            idGitIntegration: 3,
            idMr: 'mr-123',
            state: 'open',
            approved: true,
            ciStatus: 'success',
            webUrl: 'https://host/mr/123',
            headSha: 'def456'
        };
        mrStatus$.next({
            subject: 'mr_status',
            action: 'u',
            payload: notice
        });
        fixture.detectChanges();

        const expectedStatus: MrStatus = {
            state: MrState.Open,
            approved: true,
            ciStatus: CiStatus.Success,
            webUrl: 'https://host/mr/123',
            headSha: 'def456'
        };
        expect(
            (fixture.componentInstance as unknown as { mrStatus: () => MrStatus | null }).mrStatus()
        ).toEqual(expectedStatus);
    });

    it('patches the badge to merged from a terminal mr_status notice', () => {
        const fixture = TestBed.createComponent(IssueInfoComponent);
        fixture.componentRef.setInput('issue', ISSUE);
        fixture.detectChanges();

        mrStatus$.next({
            subject: 'mr_status',
            action: 'u',
            payload: {
                idIssue: 10,
                idGitIntegration: 3,
                idMr: 'mr-123',
                state: 'merged',
                approved: true,
                ciStatus: 'success',
                webUrl: 'https://host/mr/123',
                headSha: 'def456'
            } satisfies MrStatusNotice
        });
        fixture.detectChanges();

        expect(
            (fixture.componentInstance as unknown as { mrStatus: () => MrStatus | null }).mrStatus()
                ?.state
        ).toBe(MrState.Merged);
    });

    it('keeps badge, panel and requests untouched when an issue notice swaps the object', () => {
        const fixture = TestBed.createComponent(IssueInfoComponent);
        fixture.componentRef.setInput('issue', ISSUE);
        fixture.detectChanges();

        mrStatus$.next({
            subject: 'mr_status',
            action: 'u',
            payload: {
                idIssue: 10,
                idGitIntegration: 3,
                idMr: 'mr-123',
                state: 'merged',
                approved: true,
                ciStatus: 'success',
                webUrl: 'https://host/mr/123',
                headSha: 'def456'
            } satisfies MrStatusNotice
        });
        fixture.detectChanges();

        const component = fixture.componentInstance as unknown as {
            mrStatus: () => MrStatus | null;
            isPrPanelCollapsed: { (): boolean; set: (value: boolean) => void };
        };
        component.isPrPanelCollapsed.set(false);
        const patched = component.mrStatus();

        fixture.componentRef.setInput('issue', { ...ISSUE, idState: 3, title: 'renamed' });
        fixture.detectChanges();

        expect(component.mrStatus()).toBe(patched);
        expect(component.isPrPanelCollapsed()).toBe(false);
        expect(getStatus$).toHaveBeenCalledTimes(1);
        expect(getIntegration$).toHaveBeenCalledTimes(1);
    });

    it('resets and reloads when the issue points at a different MR', () => {
        const fixture = TestBed.createComponent(IssueInfoComponent);
        fixture.componentRef.setInput('issue', ISSUE);
        fixture.detectChanges();

        mrStatus$.next({
            subject: 'mr_status',
            action: 'u',
            payload: {
                idIssue: 10,
                idGitIntegration: 3,
                idMr: 'mr-123',
                state: 'open',
                approved: false,
                ciStatus: 'pending',
                webUrl: 'https://host/mr/123',
                headSha: 'def456'
            } satisfies MrStatusNotice
        });
        fixture.detectChanges();

        fixture.componentRef.setInput('issue', { ...ISSUE, mrId: 'mr-999' });
        fixture.detectChanges();

        const component = fixture.componentInstance as unknown as {
            mrStatus: () => MrStatus | null;
        };
        expect(component.mrStatus()).toBeNull();
        expect(getStatus$).toHaveBeenCalledTimes(2);
        expect(getIntegration$).toHaveBeenCalledTimes(1);
    });
});
