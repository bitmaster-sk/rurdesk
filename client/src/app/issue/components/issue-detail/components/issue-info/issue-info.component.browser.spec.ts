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

    beforeEach(async () => {
        mrStatus$ = new Subject<unknown>();
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
                    useValue: { getStatus$: () => NEVER, getDiff$: () => NEVER }
                },
                { provide: GitIntegrationApi, useValue: { get$: () => NEVER } },
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
});
