import { Component, Pipe, PipeTransform, input, output } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { MessageEditorStub, TablerIconStub } from 'src/testing/stubs';
import { IssueActivityFeedComponent } from './issue-activity-feed.component';
import { User } from 'src/app/auth/model/user.model';
import { MessageService } from 'src/app/message/message.service';
import { TrackerService } from 'src/app/shared/tracker/tracker.service';
import { NoticeService } from 'src/app/shared/notice/notice.service';
import { ProjectMemberStore } from 'src/app/project/project-member.store';
import { UserService } from 'src/app/auth/user.service';
import { BehaviorSubject, of, NEVER } from 'rxjs';
import { TranslateModule } from '@ngx-translate/core';

@Pipe({ name: 'translate', standalone: false })
class StubTranslatePipe implements PipeTransform {
    public transform(v: string): string {
        return v;
    }
}

@Pipe({ name: 'date', standalone: false })
class StubDatePipe implements PipeTransform {
    public transform(v: unknown): string {
        return String(v);
    }
}

@Component({ selector: 'app-activity-comment-item', template: '', standalone: true })
class ActivityCommentItemStub {
    public readonly message = input<unknown>(undefined);
    public readonly isOwnMessage = input<boolean>(false);
    public readonly isEditing = input<boolean>(false);
    public readonly anchoredChildren = input<unknown[]>([]);
    public readonly agentRun = input<unknown>(undefined);
    public readonly isLatestPlanForStage = input<boolean>(false);
    public readonly candidates = input<unknown>(undefined);
    public readonly mentionCandidates = input<User[]>([]);
    public readonly editRequest = output<void>();
    public readonly editSave = output<string>();
    public readonly editCancel = output<void>();
    public readonly addAnchor = output<unknown>();
    public readonly approve = output<void>();
    public readonly approveMockup = output<string>();
}

@Component({ selector: 'app-activity-time-item', template: '', standalone: true })
class ActivityTimeItemStub {
    public readonly track = input<unknown>(undefined);
    public readonly user = input<unknown>(undefined);
}

function makeUser(id: number, name: string): User {
    return { idUser: id, name, email: '', colorAvatarBg: '' };
}

describe('IssueActivityFeedComponent mentionCandidates (browser)', () => {
    const stubMessage = {
        loadMessages: () => NEVER
    };

    const stubTracker = {
        loadTracks: () => NEVER
    };

    const stubNotice = {
        Message: NEVER
    };

    const stubProjectMemberStore = {
        load: () => {},
        usersMap$: NEVER
    };

    const stubUser = {
        user: new BehaviorSubject({ idUser: 1, name: 'Me', email: '', colorAvatarBg: '' }),
        getUser: () => ({ idUser: 1, name: 'Me', email: '', colorAvatarBg: '' })
    };

    beforeEach(async () => {
        await TestBed.configureTestingModule({
            declarations: [IssueActivityFeedComponent, StubTranslatePipe, StubDatePipe],
            imports: [
                TranslateModule.forRoot(),
                MessageEditorStub,
                TablerIconStub,
                ActivityCommentItemStub,
                ActivityTimeItemStub
            ],
            providers: [
                { provide: MessageService, useValue: stubMessage },
                { provide: TrackerService, useValue: stubTracker },
                { provide: NoticeService, useValue: stubNotice },
                { provide: ProjectMemberStore, useValue: stubProjectMemberStore },
                { provide: UserService, useValue: stubUser }
            ]
        }).compileComponents();
    });

    it('mentionCandidates() returns a User[] matching the values of usersMap', () => {
        const fixture = TestBed.createComponent(IssueActivityFeedComponent);
        fixture.componentRef.setInput('idIssue', 1);
        fixture.componentRef.setInput('idProject', 10);
        fixture.detectChanges();

        const comp = fixture.componentInstance;
        const alice = makeUser(1, 'Alice');
        const bob = makeUser(2, 'Bob');

        // Set usersMap signal directly via the protected signal.
        // The computed mentionCandidates() must reflect the same values as an array.
        (comp as unknown as { usersMap: { set: (m: Map<number, User>) => void } }).usersMap.set(
            new Map([
                [1, alice],
                [2, bob]
            ])
        );

        const candidates = comp.mentionCandidates();

        expect(candidates).toHaveLength(2);
        expect(candidates).toContain(alice);
        expect(candidates).toContain(bob);
    });

    it('mentionCandidates() returns an empty array when usersMap is empty', () => {
        const fixture = TestBed.createComponent(IssueActivityFeedComponent);
        fixture.componentRef.setInput('idIssue', 1);
        fixture.componentRef.setInput('idProject', 10);
        fixture.detectChanges();

        const comp = fixture.componentInstance;
        const candidates = comp.mentionCandidates();

        expect(candidates).toHaveLength(0);
    });
});
