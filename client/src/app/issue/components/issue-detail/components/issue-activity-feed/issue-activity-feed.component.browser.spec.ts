import { Component, Pipe, PipeTransform, input, output, signal } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { MessageEditorStub, TablerIconStub } from 'src/testing/stubs';
import { IssueActivityFeedComponent } from './issue-activity-feed.component';
import { User } from 'src/app/auth/model/user.model';
import { MessageService } from 'src/app/message/message.service';
import { TrackerService } from 'src/app/shared/tracker/tracker.service';
import { NoticeService } from 'src/app/shared/notice/notice.service';
import { ProjectMemberStore } from 'src/app/project/project-member.store';
import { AuthStore } from 'src/app/auth/store/auth.store';
import { NEVER } from 'rxjs';
import { TranslateModule } from '@ngx-translate/core';
import { AgentThinkingApi } from 'src/app/agent/api/agent-thinking.api.service';

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

@Component({ selector: 'app-agent-thinking-row', template: '', standalone: true })
class AgentThinkingRowStub {
    public readonly idRun = input<number>(0);
    public readonly stage = input<unknown>(undefined);
    public readonly isLive = input<boolean>(false);
    public readonly creator = input<unknown>(undefined);
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
        Message: NEVER,
        agentThinking$: NEVER
    };

    const stubProjectMemberStore = {
        load: () => {},
        usersMap$: NEVER
    };

    const stubUser = {
        user: signal({ idUser: 1, name: 'Me', email: '', colorAvatarBg: '' }),
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
                ActivityTimeItemStub,
                AgentThinkingRowStub
            ],
            providers: [
                { provide: AgentThinkingApi, useValue: { loadStageThinking$: () => NEVER } },
                { provide: MessageService, useValue: stubMessage },
                { provide: TrackerService, useValue: stubTracker },
                { provide: NoticeService, useValue: stubNotice },
                { provide: ProjectMemberStore, useValue: stubProjectMemberStore },
                { provide: AuthStore, useValue: stubUser }
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

    it('keeps the thinking rows whatever the chips select', () => {
        const fixture = TestBed.createComponent(IssueActivityFeedComponent);
        fixture.componentRef.setInput('idIssue', 1);
        fixture.componentRef.setInput('idProject', 10);
        fixture.componentRef.setInput('agentRun', {
            idRun: 5,
            phase: 'in_progress',
            stages: [
                {
                    stage: 'design',
                    status: 'done',
                    idResultMessage: 42,
                    hasThinking: true
                }
            ]
        });
        fixture.detectChanges();

        const comp = fixture.componentInstance;
        expect(comp.thinkingRowsByMessage().get(42)).toHaveLength(1);
        expect(comp.currentAgentStage()).not.toBeNull();
        expect(fixture.nativeElement.querySelector('app-agent-thinking-row')).not.toBeNull();

        comp.setFilter('time');
        fixture.detectChanges();

        expect(comp.thinkingRowsByMessage().get(42)).toHaveLength(1);
        expect(comp.currentAgentStage()).not.toBeNull();
        expect(fixture.nativeElement.querySelector('app-agent-thinking-row')).not.toBeNull();
    });

    // The first thinking row appears before the agent has posted anything, so it
    // cannot wait for one of the agent's own messages to learn its avatar.
    it('takes the agent avatar from the members, before any agent message exists', () => {
        const fixture = TestBed.createComponent(IssueActivityFeedComponent);
        fixture.componentRef.setInput('idIssue', 1);
        fixture.componentRef.setInput('idProject', 10);
        fixture.componentRef.setInput('agentRun', { idRun: 5, idUserBot: 7, phase: 'in_progress' });
        fixture.detectChanges();

        const comp = fixture.componentInstance;
        expect(comp.agentCreator()).toBeNull();

        const bot = makeUser(7, 'Kimi');
        (comp as unknown as { usersMap: { set: (m: Map<number, User>) => void } }).usersMap.set(
            new Map([[7, bot]])
        );

        expect(comp.agentCreator()).toBe(bot);
    });

    it('offers only the comment and time chips', () => {
        const fixture = TestBed.createComponent(IssueActivityFeedComponent);
        fixture.componentRef.setInput('idIssue', 1);
        fixture.componentRef.setInput('idProject', 10);
        fixture.detectChanges();

        const chips = [...fixture.nativeElement.querySelectorAll('.chip')].map(
            (chip: HTMLElement) => chip.textContent.trim()
        );
        expect(chips).toEqual(['All', 'Comments', 'Time']);
    });

    it('does not offer a thinking row for a stage that produced none', () => {
        const fixture = TestBed.createComponent(IssueActivityFeedComponent);
        fixture.componentRef.setInput('idIssue', 1);
        fixture.componentRef.setInput('idProject', 10);
        fixture.componentRef.setInput('agentRun', {
            idRun: 5,
            phase: 'in_progress',
            stages: [{ stage: 'design', status: 'done', idResultMessage: 42, hasThinking: false }]
        });
        fixture.detectChanges();

        expect(fixture.componentInstance.thinkingRowsByMessage().size).toBe(0);
    });

    // A failed stage posts no comment, so its thinking has no comment to hang
    // under — and that is exactly the thinking a reader wants after a failure.
    it('offers a thinking row for a stage that produced no comment', () => {
        const fixture = TestBed.createComponent(IssueActivityFeedComponent);
        fixture.componentRef.setInput('idIssue', 1);
        fixture.componentRef.setInput('idProject', 10);
        fixture.componentRef.setInput('agentRun', {
            idRun: 5,
            phase: 'failed',
            stages: [
                {
                    stage: 'design',
                    status: 'failed',
                    idResultMessage: null,
                    hasThinking: true
                }
            ]
        });
        fixture.detectChanges();

        const comp = fixture.componentInstance;
        expect(comp.trailingThinkingRows()).toEqual([
            { stage: expect.objectContaining({ stage: 'design' }), isLive: false }
        ]);
        expect(fixture.nativeElement.querySelector('app-agent-thinking-row')).not.toBeNull();
    });

    // The live row already renders the running stage; repeating it as an orphan
    // would show the same stage twice while it works.
    it('leaves the running stage to the live row', () => {
        const fixture = TestBed.createComponent(IssueActivityFeedComponent);
        fixture.componentRef.setInput('idIssue', 1);
        fixture.componentRef.setInput('idProject', 10);
        fixture.componentRef.setInput('agentRun', {
            idRun: 5,
            phase: 'in_progress',
            stages: [
                {
                    stage: 'design',
                    status: 'active',
                    idResultMessage: null,
                    hasThinking: true
                }
            ]
        });
        fixture.detectChanges();

        const rows = fixture.componentInstance.trailingThinkingRows();
        expect(rows.map(row => [row.stage.stage, row.isLive])).toEqual([['design', true]]);
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
