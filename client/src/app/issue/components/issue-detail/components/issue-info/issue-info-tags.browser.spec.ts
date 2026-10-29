import { TestBed } from '@angular/core/testing';
import { Router } from '@angular/router';
import { NEVER, of } from 'rxjs';
import { Tag } from 'src/app/tag/model/tag.model';
import { AuthStore } from 'src/app/auth/store/auth.store';
import { IssueTypeStore } from 'src/app/issue-type/store/issue-type.store';
import { TagStore } from 'src/app/tag/store/tag.store';
import { GitIntegrationApi } from 'src/app/project/api/git-integration.api.service';
import { ProjectMemberStore } from 'src/app/project/project-member.store';
import { ProjectStore } from 'src/app/project/project.store';
import { SeverityStore } from 'src/app/severity/store/severity.store';
import { StateStore } from 'src/app/state/store/state.store';
import { PinApi } from 'src/app/pin/api/pin.api.service';
import { MrDiffApi } from 'src/app/issue/api/mr-diff.api.service';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { IssueApi } from '../../../../api/issue.api.service';
import { Issue, UpdateIssueReq } from '../../../../model/issue.model';
import { IssueInfoComponent } from './issue-info.component';

const ISSUE = {
    idIssue: 10,
    idIssuePublic: 42,
    idProject: 7,
    title: 'Some issue',
    description: 'body',
    assignedTo: null,
    estimated: 0,
    idsTag: [1]
} as unknown as Issue;

const tag = (idTag: number, name: string): Tag => ({
    idTag,
    idProject: 7,
    name,
    color: '#16a34a',
    orderRank: idTag,
    issueCount: 0
});

describe('IssueInfoComponent — tags (browser)', () => {
    let issueApi: { update$: ReturnType<typeof vi.fn> };
    let tagStore: {
        tagsByProject$: () => unknown;
        insert$: ReturnType<typeof vi.fn>;
        isDeleted: (idTag: number) => boolean;
    };
    let deletedTagIds: Set<number>;

    beforeEach(async () => {
        issueApi = { update$: vi.fn().mockReturnValue(of(ISSUE)) };
        deletedTagIds = new Set();
        tagStore = {
            tagsByProject$: () => of([tag(1, 'gateway'), tag(2, 'next')]),
            insert$: vi.fn(),
            isDeleted: idTag => deletedTagIds.has(idTag)
        };

        await TestBed.configureTestingModule({
            declarations: [IssueInfoComponent],
            providers: [
                { provide: IssueApi, useValue: issueApi },
                { provide: StateStore, useValue: { statesByProject$: () => of([]) } },
                { provide: SeverityStore, useValue: { severitiesByProject$: () => of([]) } },
                { provide: IssueTypeStore, useValue: { issueTypesByProject$: () => of([]) } },
                { provide: TagStore, useValue: tagStore },
                {
                    provide: ProjectMemberStore,
                    useValue: { users$: of([]), usersMap$: of(new Map()) }
                },
                { provide: ProjectStore, useValue: { project$: of({ idProject: 7, name: 'p' }) } },
                { provide: AuthStore, useValue: { getUser: () => ({ idUser: 1 }) } },
                { provide: PinApi, useValue: { insert$: () => NEVER } },
                { provide: MrDiffApi, useValue: { loadStatus$: () => NEVER, load$: () => NEVER } },
                { provide: GitIntegrationApi, useValue: { loadOne$: () => NEVER } },
                { provide: Router, useValue: { navigate: vi.fn() } }
            ]
        })
            .overrideComponent(IssueInfoComponent, { set: { template: '' } })
            .compileComponents();
    });

    function setup(): IssueInfoComponent {
        const fixture = TestBed.createComponent(IssueInfoComponent);
        fixture.componentRef.setInput('issue', ISSUE);
        fixture.componentRef.setInput('project', { idProject: 7, name: 'p' });
        fixture.detectChanges();
        return fixture.componentInstance;
    }

    function lastChanges(): UpdateIssueReq {
        const calls = issueApi.update$.mock.calls;
        return calls[calls.length - 1][2] as UpdateIssueReq;
    }

    it('starts with the tags the task has', () => {
        expect(setup().form.controls.idsTag.value).toEqual([1]);
    });

    it('saves the task as soon as its tags change', () => {
        const component = setup();

        component.form.controls.idsTag.setValue([1, 2]);

        expect(issueApi.update$).toHaveBeenCalledTimes(1);
        expect(lastChanges().idsTag).toEqual([1, 2]);
    });

    it('puts a tag created from the detail straight on the task', () => {
        tagStore.insert$.mockReturnValue(of(tag(9, 'release')));
        const component = setup();

        (component as unknown as { onCreateTag: (r: unknown) => void }).onCreateTag({
            idProject: 7,
            name: 'release',
            color: '#dc2626'
        });

        expect(tagStore.insert$).toHaveBeenCalledWith({
            idProject: 7,
            name: 'release',
            color: '#dc2626'
        });
        expect(lastChanges().idsTag).toEqual([1, 9]);
    });

    it('leaves out a tag deleted while the task is open, so the save still goes through', () => {
        const component = setup();
        deletedTagIds.add(1);

        component.form.controls.title.setValue('Renamed issue');

        expect(lastChanges().idsTag).toEqual([]);
    });

    it('keeps a tag this client has not loaded, so a save never strips it', () => {
        const component = setup();
        component.form.controls.idsTag.setValue([1, 77]);

        component.form.controls.title.setValue('Renamed issue');

        expect(lastChanges().idsTag).toEqual([1, 77]);
    });
});
