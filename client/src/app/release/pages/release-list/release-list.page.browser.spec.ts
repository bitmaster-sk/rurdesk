import { TestBed } from '@angular/core/testing';
import { ActivatedRoute, convertToParamMap } from '@angular/router';
import { TranslateModule } from '@ngx-translate/core';
import { Subject, of } from 'rxjs';
import { User } from 'src/app/auth/model/user.model';
import { ProjectMemberStore } from 'src/app/project/project-member.store';
import { AclStore } from 'src/app/project/store/acl.store';
import { NoticeService } from 'src/app/shared/notice/notice.service';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { ReleaseApi } from '../../api/release.api.service';
import { ReleaseOpenIssues } from '../../constants/release-open-issues.enum';
import { ReleaseStatus } from '../../constants/release-status.enum';
import { ReleaseView } from '../../model/release.model';
import { ReleaseListPage } from './release-list.page';

const MEMBERS: User[] = [
    { idUser: 1, name: 'Owner', email: 'o@x', colorAvatarBg: '#000' },
    { idUser: 2, name: 'Releaser', email: 'r@x', colorAvatarBg: '#000' }
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

describe('ReleaseListPage', () => {
    let api: { load$: ReturnType<typeof vi.fn> };
    let releaseNotices: Subject<unknown>;

    beforeEach(() => {
        api = { load$: vi.fn().mockReturnValue(of([DRAFT])) };
        releaseNotices = new Subject();
    });

    async function setup() {
        await TestBed.configureTestingModule({
            imports: [TranslateModule.forRoot()],
            declarations: [ReleaseListPage],
            providers: [
                { provide: ReleaseApi, useValue: api },
                {
                    provide: ActivatedRoute,
                    useValue: { snapshot: { paramMap: convertToParamMap({ idProject: '3' }) } }
                },
                { provide: AclStore, useValue: { canUpdateProject: () => true } },
                { provide: ProjectMemberStore, useValue: { users$: of(MEMBERS) } },
                {
                    provide: NoticeService,
                    useValue: { release$: releaseNotices.asObservable(), agentRun$: new Subject() }
                }
            ]
        })
            .overrideComponent(ReleaseListPage, { set: { template: '' } })
            .compileComponents();
        const fixture = TestBed.createComponent(ReleaseListPage);
        fixture.detectChanges();
        return fixture.componentInstance;
    }

    it('lists drafts and releases with who released them, or who started the draft', async () => {
        const page = await setup();

        expect(api.load$).toHaveBeenCalledWith(3);
        expect(page['releases']()).toEqual([DRAFT]);
        expect(page['authorName'](DRAFT)).toBe('Owner');
        expect(
            page['authorName']({ ...DRAFT, status: ReleaseStatus.Released, releasedBy: 2 })
        ).toBe('Releaser');
    });

    it('reloads when another member changes a release', async () => {
        vi.useFakeTimers();
        try {
            await setup();
            api.load$.mockClear();

            releaseNotices.next({});
            vi.advanceTimersByTime(300);

            expect(api.load$).toHaveBeenCalledTimes(1);
        } finally {
            vi.useRealTimers();
        }
    });
});
