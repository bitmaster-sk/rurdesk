import {
    ChangeDetectionStrategy,
    Component,
    OnInit,
    ViewChild,
    computed,
    inject,
    input,
    signal
} from '@angular/core';
import { combineLatest } from 'rxjs';
import { UiPopoverComponent } from 'src/app/ui/components/popover/popover.component';
import { Project } from '../../model/project.model';
import { AroUser, AroTeam, ProjectMembersRes } from '../../model/project-members.model';
import { ProjectMemberApi } from '../../api/project-member.api.service';
import { AclStore } from '../../store/acl.store';
import { Role } from '../../../shared/constants/role.enum';
import { TeamService } from '../../../team/team.service';
import { Team } from '../../../team/model/team.model';
import { User } from '../../../auth/model/user.model';
import { UserApi } from '../../../user/api/user.api.service';
import { UiSaveState } from '../../../ui/components/save-status/save-status-chip.component';

@Component({
    selector: 'app-project-members',
    templateUrl: './project-members.component.html',
    standalone: false,
    changeDetection: ChangeDetectionStrategy.OnPush
})
export class ProjectMembersComponent implements OnInit {
    public readonly project = input.required<Project>();

    private readonly memberApi = inject(ProjectMemberApi);
    private readonly teamService = inject(TeamService);
    private readonly userApi = inject(UserApi);
    protected readonly aclStore = inject(AclStore);

    protected readonly members = signal<ProjectMembersRes | null>(null);
    protected readonly allUsers = signal<User[]>([]);
    protected readonly allTeams = signal<Team[]>([]);
    protected readonly roles: Role[] = [Role.Viewer, Role.Member, Role.Owner];
    protected readonly pendingUserRole = signal<Role>(Role.Member);
    protected readonly pendingTeamRole = signal<Role>(Role.Member);

    /** Per-row auto-save status for the role dropdowns, keyed by id. */
    private readonly userRoleStatus = signal<Record<number, UiSaveState>>({});
    private readonly teamRoleStatus = signal<Record<number, UiSaveState>>({});

    @ViewChild('addUserPop') private readonly addUserPop!: UiPopoverComponent;
    @ViewChild('addTeamPop') private readonly addTeamPop!: UiPopoverComponent;

    protected readonly sortedUsers = computed(() =>
        [...(this.members()?.users ?? [])].sort((a, b) => a.name.localeCompare(b.name))
    );

    protected readonly sortedTeams = computed(() =>
        [...(this.members()?.teams ?? [])].sort((a, b) => a.name.localeCompare(b.name))
    );

    protected readonly availableUsers = computed(() => {
        const m = this.members();
        if (!m) return this.allUsers();
        const assignedIds = new Set(m.users.map(u => u.idUser));
        return this.allUsers().filter(u => !assignedIds.has(u.idUser));
    });

    protected readonly availableTeams = computed(() => {
        const m = this.members();
        if (!m) return this.allTeams();
        const assignedIds = new Set(m.teams.map(t => t.idTeam));
        return this.allTeams().filter(t => !assignedIds.has(t.idTeam));
    });

    protected get hasOneOwner(): boolean {
        const m = this.members();
        if (!m) return false;
        const ownerUsers = m.users.filter(u => u.role === Role.Owner && u.isDirect).length;
        const ownerTeams = m.teams.filter(t => t.role === Role.Owner).length;
        return ownerUsers + ownerTeams === 1;
    }

    protected isLastOwner(user: AroUser): boolean {
        return this.hasOneOwner && user.role === Role.Owner && user.isDirect;
    }

    public ngOnInit(): void {
        combineLatest([this.userApi.loadUsers$(), this.teamService.loadTeams()]).subscribe(
            ([users, teams]) => {
                this.allUsers.set(users);
                this.allTeams.set(teams);
            }
        );
        this.loadMembers();
    }

    protected loadMembers(): void {
        this.memberApi.getMembers(this.project().idProject).subscribe({
            next: res => this.members.set(res)
        });
    }

    public onAddUserToggle(event: Event): void {
        this.addUserPop.toggle(event);
    }

    public onAddTeamToggle(event: Event): void {
        this.addTeamPop.toggle(event);
    }

    protected onAddUser(user: User): void {
        this.memberApi
            .addUser(this.project().idProject, user.idUser, this.pendingUserRole())
            .subscribe(() => {
                this.pendingUserRole.set(Role.Member);
                this.addUserPop.hide();
                this.loadMembers();
            });
    }

    protected userRoleSaveStatus(idUser: number): UiSaveState {
        return this.userRoleStatus()[idUser] ?? UiSaveState.Idle;
    }

    protected teamRoleSaveStatus(idTeam: number): UiSaveState {
        return this.teamRoleStatus()[idTeam] ?? UiSaveState.Idle;
    }

    protected onUpdateUserRole(user: AroUser, role: Role): void {
        this.setUserRoleStatus(user.idUser, UiSaveState.Saving);
        this.memberApi.updateUserRole(this.project().idProject, user.idUser, role).subscribe({
            next: () => {
                this.setUserRoleStatus(user.idUser, UiSaveState.Saved);
                this.loadMembers();
            },
            error: () => this.setUserRoleStatus(user.idUser, UiSaveState.Error)
        });
    }

    private setUserRoleStatus(idUser: number, status: UiSaveState): void {
        this.userRoleStatus.update(m => ({ ...m, [idUser]: status }));
    }

    private setTeamRoleStatus(idTeam: number, status: UiSaveState): void {
        this.teamRoleStatus.update(m => ({ ...m, [idTeam]: status }));
    }

    protected onRemoveUser(user: AroUser): void {
        this.memberApi
            .removeUser(this.project().idProject, user.idUser)
            .subscribe(() => this.loadMembers());
    }

    protected onAddTeam(team: Team): void {
        this.memberApi
            .addTeam(this.project().idProject, team.idTeam, this.pendingTeamRole())
            .subscribe(() => {
                this.pendingTeamRole.set(Role.Member);
                this.addTeamPop.hide();
                this.loadMembers();
            });
    }

    protected onUpdateTeamRole(team: AroTeam, role: Role): void {
        this.setTeamRoleStatus(team.idTeam, UiSaveState.Saving);
        this.memberApi.updateTeamRole(this.project().idProject, team.idTeam, role).subscribe({
            next: () => {
                this.setTeamRoleStatus(team.idTeam, UiSaveState.Saved);
                this.loadMembers();
            },
            error: () => this.setTeamRoleStatus(team.idTeam, UiSaveState.Error)
        });
    }

    protected onRemoveTeam(team: AroTeam): void {
        this.memberApi
            .removeTeam(this.project().idProject, team.idTeam)
            .subscribe(() => this.loadMembers());
    }

    protected onOverrideUser(user: AroUser): void {
        this.memberApi
            .addUser(this.project().idProject, user.idUser, user.role)
            .subscribe(() => this.loadMembers());
    }

    protected getRoleSeverity(role: Role): 'warn' | 'success' | 'info' {
        switch (role) {
            case Role.Owner:
                return 'warn';
            case Role.Member:
                return 'success';
            case Role.Viewer:
                return 'info';
        }
    }
}
