import { ApplicationRef } from '@angular/core';
import { Component, input, model } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { provideNoopAnimations } from '@angular/platform-browser/animations';
import { TranslateModule } from '@ngx-translate/core';
import { of, throwError } from 'rxjs';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { ToastNotificationService } from 'src/app/core/toast-notification.service';
import { AuthStore } from 'src/app/auth/store/auth.store';
import { AvatarStub, TablerIconStub } from 'src/testing/stubs';
import { UiModule } from 'src/app/ui/ui.module';
import { AdminUserApi } from '../../api/admin-user.api.service';
import { AdminUser } from '../../model/admin-user.model';
import { AdminUsersComponent } from './admin-users.component';

@Component({ selector: 'app-admin-teams', template: '', standalone: true })
class AdminTeamsStub {
    public readonly users = input<AdminUser[]>([]);
    public readonly draggedUser = input<AdminUser | null>(null);
}

@Component({ selector: 'app-create-user-dialog', template: '', standalone: true })
class CreateUserDialogStub {
    public readonly visible = model<boolean>(false);
}

@Component({ selector: 'app-edit-user-dialog', template: '', standalone: true })
class EditUserDialogStub {
    public readonly user = input<AdminUser | null>(null);
    public readonly visible = model<boolean>(false);
}

@Component({ selector: 'app-agent-keys-dialog', template: '', standalone: true })
class AgentKeysDialogStub {
    public readonly agent = input<AdminUser | null>(null);
    public readonly visible = model<boolean>(false);
}

/**
 * The disable/enable toggle on the admin users table. Rendered against the real
 * template so the disabled-row styling, the pill, and the confirm wiring are
 * exercised — not just the component method. Missing translations render as the
 * raw key, so assertions match on the key suffix (e.g. 'ADMIN.DISABLED').
 */
describe('AdminUsersComponent — disable/enable toggle (browser)', () => {
    const active: AdminUser = {
        idUser: 7,
        name: 'Ada',
        email: 'ada@x.io',
        colorAvatarBg: '#123456',
        isAgent: false,
        isAdmin: false,
        isActive: true
    };
    const disabled: AdminUser = { ...active, idUser: 8, name: 'Bob', isActive: false };

    let setActive$: ReturnType<typeof vi.fn>;
    let showError: ReturnType<typeof vi.fn>;

    beforeEach(async () => {
        setActive$ = vi.fn(() => of(void 0));
        showError = vi.fn();
        await TestBed.configureTestingModule({
            declarations: [AdminUsersComponent],
            imports: [
                UiModule,
                TranslateModule.forRoot(),
                TablerIconStub,
                AvatarStub,
                AdminTeamsStub,
                CreateUserDialogStub,
                EditUserDialogStub,
                AgentKeysDialogStub
            ],
            providers: [
                provideNoopAnimations(),
                {
                    provide: AdminUserApi,
                    useValue: {
                        load$: () => of([active, disabled]),
                        setActive$
                    }
                },
                { provide: ToastNotificationService, useValue: { showError } },
                // Acting admin is idUser 1 — neither list row, so both show the toggle.
                { provide: AuthStore, useValue: { user: () => ({ idUser: 1 }) } }
            ]
        }).compileComponents();
    });

    function render() {
        const fixture = TestBed.createComponent(AdminUsersComponent);
        fixture.detectChanges();
        return fixture;
    }

    function rows(fixture: ReturnType<typeof render>): HTMLElement[] {
        return Array.from(fixture.nativeElement.querySelectorAll('tbody tr'));
    }

    function actionButtons(row: HTMLElement): HTMLButtonElement[] {
        return Array.from(row.querySelectorAll('td:last-child ui-button button'));
    }

    function confirm(fixture: ReturnType<typeof render>): void {
        TestBed.inject(ApplicationRef).tick();
        const buttons = Array.from(
            document.querySelectorAll<HTMLButtonElement>('.ui-confirm-panel button')
        );
        expect(buttons.length, 'confirm popup must be open').toBeGreaterThan(0);
        // Template order: reject first, accept second.
        buttons[1].click();
        fixture.detectChanges();
    }

    it('marks the disabled row and shows the Disabled pill only for it', () => {
        const fixture = render();
        const [activeRow, disabledRow] = rows(fixture);

        expect(activeRow.classList.contains('user-disabled')).toBe(false);
        expect(disabledRow.classList.contains('user-disabled')).toBe(true);

        expect(activeRow.textContent).not.toContain('ADMIN.DISABLED');
        expect(disabledRow.textContent).toContain('ADMIN.DISABLED');
    });

    it('disables the promote button for a disabled human user only', () => {
        const fixture = render();
        const [activeRow, disabledRow] = rows(fixture);

        expect(actionButtons(activeRow)[0].disabled).toBe(false);
        expect(actionButtons(disabledRow)[0].disabled).toBe(true);
    });

    it('disabling an active user PATCHes isActive:false after confirm', () => {
        const fixture = render();
        // For a human user the actions are: promote, disable, edit, delete.
        actionButtons(rows(fixture)[0])[1].click();
        confirm(fixture);

        expect(setActive$).toHaveBeenCalledWith(7, false);
    });

    it('enabling a disabled user PATCHes isActive:true after confirm', () => {
        const fixture = render();
        actionButtons(rows(fixture)[1])[1].click();
        confirm(fixture);

        expect(setActive$).toHaveBeenCalledWith(8, true);
    });

    it("hides the disable button on the acting admin's own row", () => {
        // Own row (idUser 1) plus a normal row; only the normal row gets a toggle.
        TestBed.overrideProvider(AuthStore, {
            useValue: { user: () => ({ idUser: active.idUser }) }
        });
        TestBed.overrideProvider(AdminUserApi, {
            useValue: { load$: () => of([active, disabled]), setActive$ }
        });
        const fixture = render();
        const [ownRow, otherRow] = rows(fixture);

        // Own row: promote, edit, delete (no disable toggle).
        expect(actionButtons(ownRow).length).toBe(3);
        // Other row: promote, disable, edit, delete.
        expect(actionButtons(otherRow).length).toBe(4);
    });

    it('surfaces a failed toggle as an error toast', async () => {
        setActive$ = vi.fn(() => throwError(() => ({ status: 422 })));
        TestBed.overrideProvider(AdminUserApi, {
            useValue: { load$: () => of([active]), setActive$ }
        });
        const fixture = render();
        const comp = fixture.componentInstance as unknown as {
            onToggleActive(u: AdminUser): void;
        };
        comp.onToggleActive(active);
        await fixture.whenStable();

        expect(showError).toHaveBeenCalled();
    });
});
