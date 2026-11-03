import { TestBed } from '@angular/core/testing';
import { of } from 'rxjs';
import { describe, expect, it } from 'vitest';
import { User } from 'src/app/auth/model/user.model';
import { Role } from 'src/app/shared/constants/role.enum';
import { TeamApi } from 'src/app/team/api/team.api.service';
import { UserApi } from 'src/app/user/api/user.api.service';
import { ProjectMemberApi } from '../../api/project-member.api.service';
import { AroUser, ProjectMembersRes } from '../../model/project-members.model';
import { AclStore } from '../../store/acl.store';
import { ProjectMembersComponent } from './project-members.component';

interface MembersView {
    availableUsers(): User[];
    isLastOwner(user: AroUser): boolean;
}

function aro(overrides: Partial<AroUser>): AroUser {
    return {
        idUser: 1,
        name: 'Ada',
        email: '',
        colorAvatarBg: '#000',
        role: Role.Member,
        isDirect: true,
        idsTeams: [],
        ...overrides
    };
}

async function render(members: ProjectMembersRes, allUsers: User[]): Promise<MembersView> {
    await TestBed.configureTestingModule({
        declarations: [ProjectMembersComponent],
        providers: [
            { provide: ProjectMemberApi, useValue: { load$: () => of(members) } },
            { provide: UserApi, useValue: { load$: () => of(allUsers) } },
            { provide: TeamApi, useValue: { load$: () => of([]) } },
            { provide: AclStore, useValue: {} }
        ]
    })
        .overrideComponent(ProjectMembersComponent, { set: { template: '' } })
        .compileComponents();

    const fixture = TestBed.createComponent(ProjectMembersComponent);
    fixture.componentRef.setInput('project', { idProject: 7 });
    fixture.detectChanges();
    return fixture.componentInstance as unknown as MembersView;
}

describe('ProjectMembersComponent — disabled users (browser)', () => {
    it('does not offer a disabled user to add', async () => {
        const view = await render({ users: [], teams: [] }, [
            { idUser: 5, name: 'Active', email: '', colorAvatarBg: '#000' },
            { idUser: 6, name: 'Gone', email: '', colorAvatarBg: '#000', isActive: false }
        ]);

        expect(view.availableUsers().map(user => user.idUser)).toEqual([5]);
    });

    it('does not count a disabled owner, so the active owner is the last one', async () => {
        const active = aro({ idUser: 1, role: Role.Owner });
        const disabled = aro({ idUser: 2, role: Role.Owner, isActive: false });
        const view = await render({ users: [active, disabled], teams: [] }, []);

        expect(view.isLastOwner(active)).toBe(true);
        expect(view.isLastOwner(disabled)).toBe(false);
    });
});
