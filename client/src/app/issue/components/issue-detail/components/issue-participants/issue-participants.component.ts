import {
    ChangeDetectionStrategy,
    Component,
    DestroyRef,
    computed,
    effect,
    inject,
    input,
    signal
} from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { User } from 'src/app/auth/model/user.model';
import {
    IssueParticipantModel,
    ParticipantSource
} from 'src/app/issue/model/issue-participant.model';
import { IssueParticipantService } from 'src/app/issue/service/issue-participant.service';

@Component({
    selector: 'app-issue-participants',
    templateUrl: './issue-participants.component.html',
    styleUrls: ['./issue-participants.component.scss'],
    changeDetection: ChangeDetectionStrategy.OnPush,
    standalone: false,
    providers: [IssueParticipantService]
})
export class IssueParticipantsComponent {
    public readonly idProject = input.required<number>();
    public readonly idIssuePublic = input<number | null>(null);
    public readonly idIssue = input<number | null>(null);
    public readonly currentUserId = input.required<number>();
    public readonly members = input<User[]>([]);

    protected readonly participantService = inject(IssueParticipantService);
    private readonly destroyRef = inject(DestroyRef);

    public readonly participants = this.participantService.participants;

    public readonly participantCount = computed(() => this.participants().length);

    /** First few participants shown as an overlapping avatar stack in the header. */
    public readonly headerAvatars = computed(() => this.participants().slice(0, 5));
    /** Count of participants beyond the header stack (rendered as "+N"). */
    public readonly overflowCount = computed(() => Math.max(0, this.participants().length - 5));

    public readonly isCollapsed = signal(true);
    public readonly memberSearch = signal('');

    /** Members that are NOT yet participants — available for the add popover. */
    public readonly availableMembers = computed(() => {
        const participantIds = new Set(this.participants().map(p => p.idUser));
        const search = this.memberSearch().toLowerCase();
        return this.members().filter(
            m => !participantIds.has(m.idUser) && m.name.toLowerCase().includes(search)
        );
    });

    public constructor() {
        // React to idIssuePublic changes (e.g. navigating between issues)
        effect(() => {
            const idIssuePublic = this.idIssuePublic();
            const idIssue = this.idIssue();
            const idProject = this.idProject();
            if (idIssuePublic != null && idIssue != null) {
                this.participantService.load(idProject, idIssuePublic, idIssue);
            }
        });
    }

    public onToggleCollapse(): void {
        this.isCollapsed.update(v => !v);
    }

    public onAddParticipant(member: User, popover: { hide: () => void }): void {
        const idIssuePublic = this.idIssuePublic();
        if (idIssuePublic == null) return;
        this.participantService
            .add$(this.idProject(), idIssuePublic, member.idUser)
            .pipe(takeUntilDestroyed(this.destroyRef))
            .subscribe(() => {
                popover.hide();
                this.memberSearch.set('');
                // List refresh comes via WS participant$ broadcast — no manual reload needed.
            });
    }

    public onToggleNotification(participant: IssueParticipantModel): void {
        const idIssuePublic = this.idIssuePublic();
        if (idIssuePublic == null) return;
        const newEnabled = !participant.hasNotificationsEnabled;
        this.participantService
            .setMyNotifications$(this.idProject(), idIssuePublic, newEnabled)
            .pipe(takeUntilDestroyed(this.destroyRef))
            .subscribe();
    }

    public getRoleLabelKey(participant: IssueParticipantModel): string {
        if (participant.isBot) return 'ISSUE.PARTICIPANTS.ROLE.BOT';
        switch (participant.source) {
            case ParticipantSource.Creator:
                return 'ISSUE.PARTICIPANTS.ROLE.CREATOR';
            case ParticipantSource.Assignee:
                return 'ISSUE.PARTICIPANTS.ROLE.ASSIGNEE';
            case ParticipantSource.Comment:
                return 'ISSUE.PARTICIPANTS.ROLE.COMMENT';
            case ParticipantSource.Mention:
                return 'ISSUE.PARTICIPANTS.ROLE.MENTION';
            default:
                return 'ISSUE.PARTICIPANTS.ROLE.MANUAL';
        }
    }

    public getRoleBadgeClass(participant: IssueParticipantModel): string {
        if (participant.isBot) return 'role-badge role-badge--bot';
        switch (participant.source) {
            case ParticipantSource.Creator:
                return 'role-badge role-badge--creator';
            case ParticipantSource.Assignee:
                return 'role-badge role-badge--assignee';
            case ParticipantSource.Comment:
                return 'role-badge role-badge--comment';
            case ParticipantSource.Mention:
                return 'role-badge role-badge--mention';
            default:
                return 'role-badge role-badge--manual';
        }
    }
}
