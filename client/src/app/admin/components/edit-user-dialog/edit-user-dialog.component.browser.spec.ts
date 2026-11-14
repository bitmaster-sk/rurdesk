import { AgentEffort } from 'src/app/agent/model/agent-effort.enum';
import { AgentStage } from 'src/app/agent/model/agent-stage.enum';
import { SettingsStore } from 'src/app/core/settings/settings.store';
import { TestBed } from '@angular/core/testing';
import { ReactiveFormsModule } from '@angular/forms';
import { of } from 'rxjs';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { ToastNotificationService } from 'src/app/core/toast-notification.service';
import { AdminSsoApi } from '../../api/admin-sso.api.service';
import { AdminUserApi } from '../../api/admin-user.api.service';
import { AdminUser } from '../../model/admin-user.model';
import { AgentGatewayApi } from '../../api/agent-gateway.api.service';
import { AgentGateway } from '../../model/agent-gateway.model';
import { EditUserDialogComponent } from './edit-user-dialog.component';

interface EditUserDialogInternals {
    form: EditUserDialogComponent['form'];
    onSubmit(): void;
    autoInitials(): string;
}

const GATEWAY: AgentGateway = {
    idGateway: 1,
    idUserAgent: 8,
    gatewayUrl: 'http://gw:9090',
    createdAt: '2026-10-01T00:00:00Z',
    models: ['openai/kimi'],
    modelDefault: 'openai/kimi',
    modelsReportedAt: '2026-10-17T00:00:00Z',
    maxTurnsLimit: null,
    stageDefaults: {
        [AgentStage.Brainstorming]: { maxTurns: 120 },
        [AgentStage.Design]: { effort: AgentEffort.High }
    }
};

describe('EditUserDialogComponent — gateway turn limits (browser)', () => {
    let updateGateway: ReturnType<typeof vi.fn>;

    beforeEach(async () => {
        updateGateway = vi.fn().mockReturnValue(of(GATEWAY));

        await TestBed.configureTestingModule({
            declarations: [EditUserDialogComponent],
            imports: [ReactiveFormsModule],
            providers: [
                { provide: AdminUserApi, useValue: { update$: () => of(undefined) } },
                {
                    provide: AgentGatewayApi,
                    useValue: { load$: () => of(GATEWAY), update$: updateGateway }
                },
                { provide: AdminSsoApi, useValue: { loadIdentities$: () => of([]) } },
                { provide: ToastNotificationService, useValue: { showError: vi.fn() } },
                { provide: SettingsStore, useValue: { agentStageDefaults: () => ({}) } }
            ]
        })
            .overrideComponent(EditUserDialogComponent, { set: { template: '' } })
            .compileComponents();
    });

    function open(): EditUserDialogInternals {
        const fixture = TestBed.createComponent(EditUserDialogComponent);
        fixture.componentRef.setInput('user', {
            idUser: 8,
            name: 'modelbot',
            email: '',
            colorAvatarBg: '#123456',
            isAgent: true,
            isAdmin: false,
            isActive: true
        });
        fixture.componentRef.setInput('visible', true);
        fixture.detectChanges();
        return fixture.componentInstance as unknown as EditUserDialogInternals;
    }

    function stage(dialog: EditUserDialogInternals, name: AgentStage) {
        return dialog.form.controls.stageDefaults.controls[name].controls;
    }

    it('loads the stage defaults of the agent into the form', () => {
        const dialog = open();

        expect(stage(dialog, AgentStage.Brainstorming).maxTurns.value).toBe(120);
        expect(stage(dialog, AgentStage.Design).effort.value).toBe(AgentEffort.High);
        expect(stage(dialog, AgentStage.ReleaseNotes).maxTurns.value).toBeNull();
    });

    it('saves a changed stage together with the unchanged ones and URL', () => {
        const dialog = open();

        stage(dialog, AgentStage.ReleaseNotes).maxTurns.setValue(60);
        stage(dialog, AgentStage.Implementation).effort.setValue(AgentEffort.Low);
        dialog.onSubmit();

        expect(updateGateway).toHaveBeenCalledWith(8, {
            gatewayUrl: 'http://gw:9090',
            stageDefaults: {
                [AgentStage.Brainstorming]: { maxTurns: 120, effort: null },
                [AgentStage.Design]: { maxTurns: null, effort: AgentEffort.High },
                [AgentStage.Implementation]: { maxTurns: null, effort: AgentEffort.Low },
                [AgentStage.ReleaseNotes]: { maxTurns: 60, effort: null }
            }
        });
    });

    it('a cleared stage is left out so the app settings apply again', () => {
        const dialog = open();

        stage(dialog, AgentStage.Brainstorming).maxTurns.setValue(null);
        dialog.onSubmit();

        expect(updateGateway).toHaveBeenCalledWith(8, {
            gatewayUrl: 'http://gw:9090',
            stageDefaults: { [AgentStage.Design]: { maxTurns: null, effort: AgentEffort.High } }
        });
    });

    it('a changed URL keeps the stage defaults', () => {
        const dialog = open();

        dialog.form.controls.gatewayUrl.setValue('http://other:9090');
        dialog.onSubmit();

        expect(updateGateway).toHaveBeenCalledWith(
            8,
            expect.objectContaining({
                gatewayUrl: 'http://other:9090',
                stageDefaults: {
                    [AgentStage.Brainstorming]: { maxTurns: 120, effort: null },
                    [AgentStage.Design]: { maxTurns: null, effort: AgentEffort.High }
                }
            })
        );
    });

    it('leaves the gateway alone when nothing about it changed', () => {
        open().onSubmit();

        expect(updateGateway).not.toHaveBeenCalled();
    });
});

describe('EditUserDialogComponent — avatar initials (browser)', () => {
    let updateUser: ReturnType<typeof vi.fn>;

    beforeEach(async () => {
        updateUser = vi.fn().mockReturnValue(of(undefined));

        await TestBed.configureTestingModule({
            declarations: [EditUserDialogComponent],
            imports: [ReactiveFormsModule],
            providers: [
                { provide: AdminUserApi, useValue: { update$: updateUser } },
                {
                    provide: AgentGatewayApi,
                    useValue: { load$: () => of(null), update$: () => of({}) }
                },
                { provide: AdminSsoApi, useValue: { loadIdentities$: () => of([]) } },
                { provide: ToastNotificationService, useValue: { showError: vi.fn() } },
                { provide: SettingsStore, useValue: { agentStageDefaults: () => ({}) } }
            ]
        })
            .overrideComponent(EditUserDialogComponent, { set: { template: '' } })
            .compileComponents();
    });

    function open(): EditUserDialogInternals {
        const fixture = TestBed.createComponent(EditUserDialogComponent);
        fixture.componentRef.setInput('user', {
            idUser: 8,
            name: 'modelbot',
            email: '',
            colorAvatarBg: '#123456',
            avatarInitials: 'MB',
            isAgent: true,
            isAdmin: false,
            isActive: true
        });
        fixture.componentRef.setInput('visible', true);
        fixture.detectChanges();
        return fixture.componentInstance as unknown as EditUserDialogInternals;
    }

    it('loads the stored initials into the form', () => {
        expect(open().form.controls.avatarInitials.value).toBe('MB');
    });

    it('sends the edited initials in the update request', () => {
        const dialog = open();

        dialog.form.controls.avatarInitials.setValue('xy');
        dialog.onSubmit();

        expect(updateUser).toHaveBeenCalledWith(
            8,
            expect.objectContaining({ name: 'modelbot', avatarInitials: 'xy' })
        );
    });

    it('sends an empty string when initials are cleared (back to auto)', () => {
        const dialog = open();

        dialog.form.controls.avatarInitials.setValue('');
        dialog.onSubmit();

        expect(updateUser).toHaveBeenCalledWith(8, expect.objectContaining({ avatarInitials: '' }));
    });

    it('updates the placeholder with the auto initials as the name is edited', () => {
        const dialog = open();

        expect(dialog.autoInitials()).toBe('MO');

        dialog.form.controls.name.setValue('Ada Lovelace');

        expect(dialog.autoInitials()).toBe('AL');
    });
});

describe('EditUserDialogComponent — admin sets a password (browser)', () => {
    let update$: ReturnType<typeof vi.fn>;
    let setPassword$: ReturnType<typeof vi.fn>;

    beforeEach(async () => {
        update$ = vi.fn().mockReturnValue(of(undefined));
        setPassword$ = vi.fn().mockReturnValue(of(undefined));

        await TestBed.configureTestingModule({
            declarations: [EditUserDialogComponent],
            imports: [ReactiveFormsModule],
            providers: [
                { provide: AdminUserApi, useValue: { update$, setPassword$ } },
                { provide: AgentGatewayApi, useValue: { load$: () => of(null), update$: vi.fn() } },
                { provide: AdminSsoApi, useValue: { loadIdentities$: () => of([]) } },
                { provide: ToastNotificationService, useValue: { showError: vi.fn() } },
                { provide: SettingsStore, useValue: { agentStageDefaults: () => ({}) } }
            ]
        })
            .overrideComponent(EditUserDialogComponent, { set: { template: '' } })
            .compileComponents();
    });

    function openHuman(): EditUserDialogInternals {
        const fixture = TestBed.createComponent(EditUserDialogComponent);
        fixture.componentRef.setInput('user', {
            idUser: 5,
            name: 'human',
            email: 'human@test.sk',
            colorAvatarBg: '#123456',
            isAgent: false,
            isAdmin: false,
            isActive: true,
            isBreakGlass: false,
            isPasswordless: false,
            lastLoginAt: null,
            hasSsoIdentity: false,
            isScimManaged: false
        });
        fixture.componentRef.setInput('visible', true);
        fixture.detectChanges();
        return fixture.componentInstance as unknown as EditUserDialogInternals;
    }

    it('keeps the current password when the field is left empty', () => {
        const dialog = openHuman();

        dialog.onSubmit();

        expect(update$).toHaveBeenCalledTimes(1);
        expect(setPassword$).not.toHaveBeenCalled();
    });

    it('sets the entered password next to the profile update', () => {
        const dialog = openHuman();

        dialog.form.controls.newPassword.setValue('brand-new');
        dialog.onSubmit();

        expect(setPassword$).toHaveBeenCalledWith(5, 'brand-new');
        expect(update$).toHaveBeenCalledTimes(1);
    });

    it('rejects a password shorter than the backend accepts', () => {
        const dialog = openHuman();

        dialog.form.controls.newPassword.setValue('abc');
        dialog.onSubmit();

        expect(setPassword$).not.toHaveBeenCalled();
        expect(update$).not.toHaveBeenCalled();
    });
});

describe('EditUserDialogComponent — SSO identity (browser)', () => {
    const identity = {
        idUserIdentity: 1,
        issuer: 'https://idp.acme.test',
        subject: 'sub-1',
        createAt: '2026-11-10T10:00:00Z'
    };
    const person = (changes: Partial<AdminUser>): AdminUser => ({
        idUser: 5,
        name: 'Jana',
        email: 'jana@acme.test',
        colorAvatarBg: '#123456',
        isAgent: false,
        isAdmin: false,
        isActive: true,
        isBreakGlass: false,
        isPasswordless: true,
        lastLoginAt: null,
        hasSsoIdentity: true,
        isScimManaged: false,
        ...changes
    });
    let ssoApi: {
        loadIdentities$: ReturnType<typeof vi.fn>;
        unlinkIdentity$: ReturnType<typeof vi.fn>;
        moveIdentity$: ReturnType<typeof vi.fn>;
    };

    interface SsoInternals {
        identities(): unknown[];
        moveTargets(): AdminUser[];
        moveTarget: { set(value: number | null): void };
        onMoveIdentity(): void;
        onUnlinkIdentity(): void;
    }

    beforeEach(async () => {
        ssoApi = {
            loadIdentities$: vi.fn().mockReturnValue(of([identity])),
            unlinkIdentity$: vi.fn().mockReturnValue(of([])),
            moveIdentity$: vi.fn().mockReturnValue(of([]))
        };
        await TestBed.configureTestingModule({
            declarations: [EditUserDialogComponent],
            imports: [ReactiveFormsModule],
            providers: [
                { provide: AdminUserApi, useValue: { update$: () => of(undefined) } },
                { provide: AgentGatewayApi, useValue: { load$: () => of(null), update$: vi.fn() } },
                { provide: AdminSsoApi, useValue: ssoApi },
                { provide: ToastNotificationService, useValue: { showError: vi.fn() } },
                { provide: SettingsStore, useValue: { agentStageDefaults: () => ({}) } }
            ]
        })
            .overrideComponent(EditUserDialogComponent, { set: { template: '' } })
            .compileComponents();
    });

    function open(user: AdminUser) {
        const fixture = TestBed.createComponent(EditUserDialogComponent);
        fixture.componentRef.setInput('user', user);
        fixture.componentRef.setInput('users', [
            user,
            person({ idUser: 6, name: 'Target' }),
            person({ idUser: 7, isAgent: true }),
            person({ idUser: 8, isBreakGlass: true }),
            person({ idUser: 9, isActive: false })
        ]);
        fixture.componentRef.setInput('visible', true);
        fixture.detectChanges();
        const updated = vi.fn();
        fixture.componentInstance.updated.subscribe(updated);
        return {
            fixture,
            dialog: fixture.componentInstance as unknown as SsoInternals,
            updated
        };
    }

    it('loads the identity of a linked account only', () => {
        const linked = open(person({}));
        expect(ssoApi.loadIdentities$).toHaveBeenCalledWith(5);
        expect(linked.dialog.identities()).toEqual([identity]);
    });

    it('skips the identity request for an account without SSO', () => {
        const { dialog } = open(person({ hasSsoIdentity: false }));

        expect(ssoApi.loadIdentities$).not.toHaveBeenCalled();
        expect(dialog.identities()).toEqual([]);
    });

    it('offers only other active person accounts as move targets', () => {
        const { dialog } = open(person({}));

        expect(dialog.moveTargets().map(user => user.idUser)).toEqual([6]);
    });

    it('moves the identity and closes the dialog', () => {
        const { fixture, dialog, updated } = open(person({}));

        dialog.moveTarget.set(6);
        dialog.onMoveIdentity();

        expect(ssoApi.moveIdentity$).toHaveBeenCalledWith(5, { idTargetUser: 6 });
        expect(updated).toHaveBeenCalled();
        expect(fixture.componentInstance.visible()).toBe(false);
    });

    it('unlinks the identity', () => {
        const { dialog, updated } = open(person({}));

        dialog.onUnlinkIdentity();

        expect(ssoApi.unlinkIdentity$).toHaveBeenCalledWith(5);
        expect(dialog.identities()).toEqual([]);
        expect(updated).toHaveBeenCalled();
    });
});

describe('EditUserDialogComponent — break-glass (browser)', () => {
    let update$: ReturnType<typeof vi.fn>;
    const admin: AdminUser = {
        idUser: 5,
        name: 'vault',
        email: 'vault@test.sk',
        colorAvatarBg: '#123456',
        isAgent: false,
        isAdmin: true,
        isActive: true,
        isBreakGlass: false,
        isPasswordless: false,
        lastLoginAt: null,
        hasSsoIdentity: false,
        isScimManaged: false
    };

    beforeEach(async () => {
        update$ = vi.fn().mockReturnValue(of(undefined));
        await TestBed.configureTestingModule({
            declarations: [EditUserDialogComponent],
            imports: [ReactiveFormsModule],
            providers: [
                { provide: AdminUserApi, useValue: { update$, setPassword$: vi.fn() } },
                { provide: AgentGatewayApi, useValue: { load$: () => of(null), update$: vi.fn() } },
                { provide: AdminSsoApi, useValue: { loadIdentities$: () => of([]) } },
                { provide: ToastNotificationService, useValue: { showError: vi.fn() } },
                { provide: SettingsStore, useValue: { agentStageDefaults: () => ({}) } }
            ]
        })
            .overrideComponent(EditUserDialogComponent, { set: { template: '' } })
            .compileComponents();
    });

    function open(user: AdminUser) {
        const fixture = TestBed.createComponent(EditUserDialogComponent);
        fixture.componentRef.setInput('user', user);
        fixture.componentRef.setInput('visible', true);
        fixture.detectChanges();
        return fixture.componentInstance as unknown as EditUserDialogInternals & {
            canBeBreakGlass(): boolean;
        };
    }

    it('offers the flag only to an active admin with a password and no SSO identity', () => {
        expect(open(admin).canBeBreakGlass()).toBe(true);
        expect(open({ ...admin, isAdmin: false }).canBeBreakGlass()).toBe(false);
        expect(open({ ...admin, isPasswordless: true }).canBeBreakGlass()).toBe(false);
        expect(open({ ...admin, hasSsoIdentity: true }).canBeBreakGlass()).toBe(false);
        expect(open({ ...admin, isActive: false }).canBeBreakGlass()).toBe(false);
    });

    it('sends the flag only when it changed', () => {
        const unchanged = open(admin);
        unchanged.onSubmit();
        expect(update$.mock.calls[0][1]).not.toHaveProperty('isBreakGlass');

        const marked = open(admin);
        marked.form.controls.isBreakGlass.setValue(true);
        marked.onSubmit();
        expect(update$.mock.calls[1][1]).toMatchObject({ isBreakGlass: true });
    });
});
