import { TestBed } from '@angular/core/testing';
import { of } from 'rxjs';
import { AdminUser } from '../../model/admin-user.model';
import { Team } from '../../../team/model/team.model';
import { TeamService } from '../../../team/team.service';
import { AdminApi } from '../../api/admin.api.service';
import { AdminTeamsComponent } from './admin-teams.component';

/**
 * Regression for the pDroppable → native HTML5 drop migration. The template is
 * trimmed to the two real drop targets so we exercise the REAL bindings
 * (`(dragover)`, `(drop)`) against the real component. Native `drop` only fires
 * when `dragover` calls preventDefault — the gating logic decides when that
 * happens, so we assert `defaultPrevented` and the resulting API call.
 */
describe('AdminTeamsComponent — native drop targets (browser)', () => {
    const team = { idTeam: 3, name: 'Core', color: '#fff' };
    const user: AdminUser = {
        idUser: 7,
        name: 'Ada',
        email: 'ada@x.io',
        colorAvatarBg: '#123456',
        isBot: false,
        isAdmin: false
    };
    let addTeamMember$: ReturnType<typeof vi.fn>;

    beforeEach(async () => {
        addTeamMember$ = vi.fn(() => of(void 0));
        await TestBed.configureTestingModule({
            declarations: [AdminTeamsComponent],
            providers: [
                { provide: TeamService, useValue: { loadTeams: () => of([team]) } },
                {
                    provide: AdminApi,
                    useValue: { listTeamMembers$: () => of([]), addTeamMember$ }
                }
            ]
        })
            .overrideComponent(AdminTeamsComponent, {
                set: {
                    template: `@for (t of teams(); track t.idTeam) {
                        <div class="team-row" (dragover)="onDragOver($event)"
                             (drop)="onDropOnTeam($event, t)"></div>
                    }
                    <div class="members-panel" (dragover)="onMembersDragOver($event)"
                         (drop)="onDropOnMembers($event)"></div>`
                }
            })
            .compileComponents();
    });

    function render(dragged: AdminUser | null) {
        const fixture = TestBed.createComponent(AdminTeamsComponent);
        fixture.componentRef.setInput('users', []);
        fixture.componentRef.setInput('draggedUser', dragged);
        fixture.detectChanges();
        const el = fixture.nativeElement as HTMLElement;
        return {
            fixture,
            comp: fixture.componentInstance as unknown as { onSelectTeam: (t: Team) => void },
            teamRow: el.querySelector('.team-row') as HTMLElement,
            membersPanel: el.querySelector('.members-panel') as HTMLElement
        };
    }

    function dragover(target: HTMLElement): boolean {
        const ev = new DragEvent('dragover', {
            dataTransfer: new DataTransfer(),
            cancelable: true,
            bubbles: true
        });
        target.dispatchEvent(ev);
        return ev.defaultPrevented;
    }

    function drop(target: HTMLElement): void {
        target.dispatchEvent(
            new DragEvent('drop', {
                dataTransfer: new DataTransfer(),
                cancelable: true,
                bubbles: true
            })
        );
    }

    it('team row allows the drop only while a user is being dragged', () => {
        expect(dragover(render(user).teamRow)).toBe(true);
        expect(dragover(render(null).teamRow)).toBe(false);
    });

    it('members panel allows the drop only with a dragged user AND a selected team', () => {
        const withTeam = render(user);
        withTeam.comp.onSelectTeam(team);
        withTeam.fixture.detectChanges();
        expect(dragover(withTeam.membersPanel)).toBe(true);

        // dragged user but no team selected → blocked
        expect(dragover(render(user).membersPanel)).toBe(false);
    });

    it('dropping a user on a team row adds them to that team', () => {
        drop(render(user).teamRow);
        expect(addTeamMember$).toHaveBeenCalledWith(team.idTeam, user.idUser);
    });

    it('dropping with no dragged user is a no-op', () => {
        drop(render(null).teamRow);
        expect(addTeamMember$).not.toHaveBeenCalled();
    });

    it('dropping on the members panel with no selected team is a no-op', () => {
        drop(render(user).membersPanel);
        expect(addTeamMember$).not.toHaveBeenCalled();
    });
});
