import { Injector, runInInjectionContext } from '@angular/core';
import { Subject } from 'rxjs';
import { describe, expect, it, vi } from 'vitest';
import { Issue } from 'src/app/issue/model/issue.model';
import { NoticeAction } from 'src/app/shared/notice/constant/notice-action.enum';
import { NoticeSubject } from 'src/app/shared/notice/constant/notice-subject.enum';
import { Notice } from 'src/app/shared/notice/model/notice.model';
import { NoticeService } from 'src/app/shared/notice/notice.service';
import { IssueLinkApi } from '../api/issue-link.api.service';
import { IssueLinkInfo, IssueLinkRef } from '../model/issue-link.model';
import { IssueLinkInfoStore } from './issue-link-info.store';

const info: IssueLinkInfo = {
    issues: [
        {
            idProject: 7,
            idIssuePublic: 12,
            title: 'Fix login',
            idState: 1,
            idSeverity: 4,
            idIssueType: 3,
            assignedTo: 5
        }
    ],
    projects: [{ idProject: 7, name: 'Rurdesk' }],
    states: [
        {
            idState: 1,
            idProject: 7,
            name: 'Open',
            start: true,
            final: false,
            protected: false,
            orderRank: 1
        },
        {
            idState: 2,
            idProject: 7,
            name: 'Done',
            start: false,
            final: true,
            protected: false,
            orderRank: 2
        }
    ],
    severities: [
        {
            idSeverity: 4,
            idProject: 7,
            title: 'High',
            color: '#ff0000',
            protected: false,
            orderRank: 1
        },
        {
            idSeverity: 6,
            idProject: 7,
            title: 'Low',
            color: '#00ff00',
            protected: false,
            orderRank: 2
        }
    ],
    issueTypes: [{ idIssueType: 3, idProject: 7, name: 'Bug', protected: false, orderRank: 1 }],
    users: [{ idUser: 5, name: 'Ann', colorAvatarBg: '#fff' }]
};

const issue = (patch: Partial<Issue>): Issue => ({
    idIssue: 100,
    idIssuePublic: 12,
    idProject: 7,
    idState: 1,
    idSeverity: 4,
    idIssueType: 3,
    title: 'Fix login',
    description: '',
    tracked: 0,
    assignedTo: 5,
    ...patch
});

function buildStore(): {
    store: IssueLinkInfoStore;
    loadInfo: ReturnType<typeof vi.fn>;
    notices: Subject<Notice<Issue>>;
} {
    const loadInfo = vi.fn().mockImplementation(() => new Subject<IssueLinkInfo>());
    const notices = new Subject<Notice<Issue>>();
    const injector = Injector.create({
        providers: [
            { provide: IssueLinkApi, useValue: { loadInfo$: loadInfo } },
            { provide: NoticeService, useValue: { issue$: notices } }
        ]
    });
    return {
        store: runInInjectionContext(injector, () => new IssueLinkInfoStore()),
        loadInfo,
        notices
    };
}

const ref: IssueLinkRef = { idProject: 7, idIssuePublic: 12 };

async function flushRequests(): Promise<void> {
    await Promise.resolve();
}

describe('IssueLinkInfoStore', () => {
    it('loads the links of one render in a single request', async () => {
        const { store, loadInfo } = buildStore();
        store.request([ref]);
        store.request([{ idProject: 9, idIssuePublic: 3 }, ref]);
        await flushRequests();

        expect(loadInfo).toHaveBeenCalledTimes(1);
        expect(loadInfo).toHaveBeenCalledWith([ref, { idProject: 9, idIssuePublic: 3 }]);
    });

    it('does not ask again for a link it already requested', async () => {
        const { store, loadInfo } = buildStore();
        store.request([ref]);
        await flushRequests();
        loadInfo.mock.results[0].value.next(info);

        store.request([ref]);
        await flushRequests();

        expect(loadInfo).toHaveBeenCalledTimes(1);
    });

    it('describes a loaded task with its project, state, type and assignee', async () => {
        const { store, loadInfo } = buildStore();
        store.request([ref]);
        await flushRequests();
        expect(store.view(ref)).toBeNull();

        loadInfo.mock.results[0].value.next(info);

        expect(store.view(ref)).toEqual({
            idProject: 7,
            idIssuePublic: 12,
            title: 'Fix login',
            projectName: 'Rurdesk',
            state: info.states[0],
            isFinal: false,
            severity: info.severities[0],
            issueType: info.issueTypes[0],
            assignee: { idUser: 5, name: 'Ann', colorAvatarBg: '#fff' }
        });
    });

    it('marks the task as closed when an update notice moves it into a final state', async () => {
        const { store, loadInfo, notices } = buildStore();
        store.request([ref]);
        await flushRequests();
        loadInfo.mock.results[0].value.next(info);

        notices.next({
            subject: NoticeSubject.Issue,
            action: NoticeAction.Update,
            payload: issue({ idState: 2, idSeverity: 6, title: 'Fix login flow', assignedTo: null })
        });

        expect(store.view(ref)).toMatchObject({
            title: 'Fix login flow',
            state: info.states[1],
            isFinal: true,
            severity: info.severities[1],
            assignee: null
        });
        expect(loadInfo).toHaveBeenCalledTimes(1);
    });

    it('forgets a task when it is deleted', async () => {
        const { store, loadInfo, notices } = buildStore();
        store.request([ref]);
        await flushRequests();
        loadInfo.mock.results[0].value.next(info);

        notices.next({
            subject: NoticeSubject.Issue,
            action: NoticeAction.Delete,
            payload: issue({})
        });

        expect(store.view(ref)).toBeNull();
    });

    it('ignores notices about tasks that no link shows', async () => {
        const { store, loadInfo, notices } = buildStore();
        store.request([ref]);
        await flushRequests();
        loadInfo.mock.results[0].value.next(info);

        notices.next({
            subject: NoticeSubject.Issue,
            action: NoticeAction.Update,
            payload: issue({ idIssuePublic: 99, title: 'Other' })
        });
        notices.next({
            subject: NoticeSubject.Issue,
            action: NoticeAction.Create,
            payload: 13 as unknown as Issue
        });

        expect(store.view({ idProject: 7, idIssuePublic: 99 })).toBeNull();
        expect(store.view(ref)?.title).toBe('Fix login');
        expect(loadInfo).toHaveBeenCalledTimes(1);
    });

    it('allows a retry after a failed load', async () => {
        const { store, loadInfo } = buildStore();
        store.request([ref]);
        await flushRequests();
        loadInfo.mock.results[0].value.error(new Error('boom'));

        store.request([ref]);
        await flushRequests();

        expect(loadInfo).toHaveBeenCalledTimes(2);
    });
});
