import {
    ChangeDetectionStrategy,
    Component,
    OnInit,
    computed,
    inject,
    input,
    signal
} from '@angular/core';
import { AdminSsoApi } from '../../api/admin-sso.api.service';
import { AdminTeamApi } from '../../api/admin-team.api.service';
import { ToastNotificationService } from 'src/app/core/toast-notification.service';
import { AdminUser } from '../../model/admin-user.model';
import { Team } from '../../../team/model/team.model';
import { User } from '../../../auth/model/user.model';
import { TeamApi } from '../../../team/api/team.api.service';

/**
 * AdminTeamsComponent manages teams and their members (admin screen, bottom panel).
 * Members are added via the search popover or by dragging a user row from the
 * users table (top panel) onto a team row.
 */
@Component({
    selector: 'app-admin-teams',
    templateUrl: './admin-teams.component.html',
    styleUrls: ['./admin-teams.component.scss'],
    standalone: false,
    changeDetection: ChangeDetectionStrategy.OnPush
})
export class AdminTeamsComponent implements OnInit {
    /** all users, provided by the parent admin page (drag source + add dropdown) */
    public readonly users = input.required<AdminUser[]>();
    /** user row currently dragged from the parent users table (null when none) */
    public readonly draggedUser = input<AdminUser | null>(null);

    private readonly adminTeamApi = inject(AdminTeamApi);
    private readonly teamApi = inject(TeamApi);
    private readonly adminSsoApi = inject(AdminSsoApi);
    private readonly sToast = inject(ToastNotificationService);

    protected readonly teams = signal<Team[]>([]);
    protected readonly selectedTeam = signal<Team | null>(null);
    protected readonly members = signal<User[]>([]);
    protected readonly showTeamDialog = signal(false);
    protected readonly editedTeam = signal<Team | null>(null);
    protected readonly ssoTeamIds = signal<ReadonlySet<number>>(new Set());

    protected readonly availableUsers = computed<User[]>(() => {
        const memberIds = new Set(this.members().map(m => m.idUser));
        // AdminUser is the admin-list shape of User — safe to hand to the
        // selector, which renders name and avatar only.
        return (this.users() as unknown as User[]).filter(
            u => u.isActive !== false && !memberIds.has(u.idUser)
        );
    });

    public ngOnInit(): void {
        this.loadTeams();
        this.adminSsoApi.loadMappings$().subscribe({
            next: mappings => this.ssoTeamIds.set(new Set(mappings.map(mapping => mapping.idTeam))),
            error: () => this.ssoTeamIds.set(new Set())
        });
    }

    private loadTeams(): void {
        this.teamApi.load$().subscribe(teams => this.teams.set(teams));
    }

    protected onSelectTeam(team: Team): void {
        this.selectedTeam.set(team);
        this.loadMembers(team.idTeam);
    }

    private loadMembers(idTeam: number): void {
        this.adminTeamApi.loadMembers$(idTeam).subscribe(members => this.members.set(members));
    }

    protected onNewTeam(): void {
        this.editedTeam.set(null);
        this.showTeamDialog.set(true);
    }

    protected onEditTeam(team: Team): void {
        this.editedTeam.set(team);
        this.showTeamDialog.set(true);
    }

    protected onTeamSaved(team: Team): void {
        this.loadTeams();
        this.onSelectTeam(team);
    }

    protected onConfirmDeleteTeam(team: Team): void {
        this.adminTeamApi.delete$(team.idTeam).subscribe(() => {
            if (this.selectedTeam()?.idTeam === team.idTeam) {
                this.selectedTeam.set(null);
                this.members.set([]);
            }
            this.loadTeams();
        });
    }

    protected onAddMember(user: User | AdminUser, team: Team | null = this.selectedTeam()): void {
        if (!team) return;
        this.adminTeamApi.insertMember$(team.idTeam, user.idUser).subscribe(() => {
            this.warnIfSsoManaged(user.idUser, team.idTeam);
            if (this.selectedTeam()?.idTeam === team.idTeam) {
                this.loadMembers(team.idTeam);
            }
        });
    }

    /** Allow a drop on a team row only while a user is actually being dragged. */
    protected onDragOver(event: DragEvent): void {
        if (this.draggedUser()) this.allowDrop(event);
    }

    /** Members panel accepts a drop only with a dragged user AND a selected team. */
    protected onMembersDragOver(event: DragEvent): void {
        if (this.draggedUser() && this.selectedTeam()) this.allowDrop(event);
    }

    private allowDrop(event: DragEvent): void {
        event.preventDefault(); // without this the native `drop` event never fires
        if (event.dataTransfer) event.dataTransfer.dropEffect = 'copy';
    }

    /** Drop target on a team row — adds the user dragged from the users table */
    protected onDropOnTeam(event: DragEvent, team: Team): void {
        event.preventDefault();
        const user = this.draggedUser();
        if (!user) return;
        if (
            this.members().some(m => m.idUser === user.idUser) &&
            this.selectedTeam()?.idTeam === team.idTeam
        ) {
            return;
        }
        this.selectedTeam.set(team);
        this.onAddMember(user, team);
    }

    /** Drop target on the members table — adds the dragged user to the selected team */
    protected onDropOnMembers(event: DragEvent): void {
        event.preventDefault();
        const team = this.selectedTeam();
        const user = this.draggedUser();
        if (!team || !user) return;
        if (this.members().some(m => m.idUser === user.idUser)) return;
        this.onAddMember(user, team);
    }

    protected onConfirmRemoveMember(user: User): void {
        const team = this.selectedTeam();
        if (!team) return;
        this.adminTeamApi.deleteMember$(team.idTeam, user.idUser).subscribe(() => {
            this.warnIfSsoManaged(user.idUser, team.idTeam);
            this.loadMembers(team.idTeam);
        });
    }

    private warnIfSsoManaged(idUser: number, idTeam: number): void {
        const isSsoUser = this.users().some(user => user.idUser === idUser && user.hasSsoIdentity);
        if (isSsoUser && this.ssoTeamIds().has(idTeam)) {
            this.sToast.showInfo('ADMIN.SSO_TEAM.OVERRIDE_WARNING', 6000);
        }
    }
}
