import { AgentEffort } from 'src/app/agent/model/agent-effort.enum';
import { AgentStage } from 'src/app/agent/model/agent-stage.enum';
import { SettingsStore } from 'src/app/core/settings/settings.store';
import { TestBed } from '@angular/core/testing';
import { ReactiveFormsModule } from '@angular/forms';
import { of } from 'rxjs';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { ToastNotificationService } from 'src/app/core/toast-notification.service';
import { AdminUserApi } from '../../api/admin-user.api.service';
import { AgentGatewayApi } from '../../api/agent-gateway.api.service';
import { CreateUserDialogComponent } from './create-user-dialog.component';

interface CreateUserDialogInternals {
    form: CreateUserDialogComponent['form'];
    onSubmit(): void;
    autoInitials(): string;
}

describe('CreateUserDialogComponent — agent turn limits (browser)', () => {
    let insertGateway: ReturnType<typeof vi.fn>;

    beforeEach(async () => {
        insertGateway = vi.fn().mockReturnValue(of({ trackerToGatewayToken: 'token' }));

        await TestBed.configureTestingModule({
            declarations: [CreateUserDialogComponent],
            imports: [ReactiveFormsModule],
            providers: [
                {
                    provide: AdminUserApi,
                    useValue: { insert$: () => of({ idUser: 8, name: 'modelbot', isAgent: true }) }
                },
                { provide: AgentGatewayApi, useValue: { insert$: insertGateway } },
                { provide: ToastNotificationService, useValue: { showError: vi.fn() } },
                { provide: SettingsStore, useValue: { agentStageDefaults: () => ({}) } }
            ]
        })
            .overrideComponent(CreateUserDialogComponent, { set: { template: '' } })
            .compileComponents();
    });

    function openAgentForm(): CreateUserDialogInternals {
        const fixture = TestBed.createComponent(CreateUserDialogComponent);
        fixture.componentRef.setInput('visible', true);
        fixture.detectChanges();
        const dialog = fixture.componentInstance as unknown as CreateUserDialogInternals;
        dialog.form.patchValue({ name: 'modelbot', isAgent: true, gatewayUrl: 'http://gw:9090' });
        return dialog;
    }

    it('creates the gateway with the stage defaults the admin entered', () => {
        const dialog = openAgentForm();

        dialog.form.patchValue({
            stageDefaults: {
                [AgentStage.Brainstorming]: { maxTurns: 90 },
                [AgentStage.ReleaseNotes]: { maxTurns: 60, effort: AgentEffort.Low }
            }
        });
        dialog.onSubmit();

        expect(insertGateway).toHaveBeenCalledWith(8, {
            gatewayUrl: 'http://gw:9090',
            stageDefaults: {
                [AgentStage.Brainstorming]: { maxTurns: 90, effort: null },
                [AgentStage.ReleaseNotes]: { maxTurns: 60, effort: AgentEffort.Low }
            }
        });
    });

    it('leaves empty stages to the app settings', () => {
        openAgentForm().onSubmit();

        expect(insertGateway).toHaveBeenCalledWith(8, {
            gatewayUrl: 'http://gw:9090',
            stageDefaults: {}
        });
    });

    it('refuses a limit above the ceiling', () => {
        const dialog = openAgentForm();

        dialog.form.patchValue({ stageDefaults: { [AgentStage.Design]: { maxTurns: 10001 } } });
        dialog.onSubmit();

        expect(insertGateway).not.toHaveBeenCalled();
    });
});

describe('CreateUserDialogComponent — avatar initials (browser)', () => {
    let insertUser: ReturnType<typeof vi.fn>;

    beforeEach(async () => {
        insertUser = vi
            .fn()
            .mockReturnValue(
                of({ idUser: 9, name: 'human', isAgent: false, colorAvatarBg: '#123456' })
            );

        await TestBed.configureTestingModule({
            declarations: [CreateUserDialogComponent],
            imports: [ReactiveFormsModule],
            providers: [
                { provide: AdminUserApi, useValue: { insert$: insertUser } },
                { provide: AgentGatewayApi, useValue: { insert$: () => of({}) } },
                { provide: ToastNotificationService, useValue: { showError: vi.fn() } },
                { provide: SettingsStore, useValue: { agentStageDefaults: () => ({}) } }
            ]
        })
            .overrideComponent(CreateUserDialogComponent, { set: { template: '' } })
            .compileComponents();
    });

    function openHumanForm(): CreateUserDialogInternals {
        const fixture = TestBed.createComponent(CreateUserDialogComponent);
        fixture.componentRef.setInput('visible', true);
        fixture.detectChanges();
        const dialog = fixture.componentInstance as unknown as CreateUserDialogInternals;
        dialog.form.patchValue({
            name: 'human',
            isAgent: false,
            email: 'human@test.sk',
            password: 'secret1'
        });
        return dialog;
    }

    it('sends the entered initials in the create request', () => {
        const dialog = openHumanForm();

        dialog.form.controls.avatarInitials.setValue('ab');
        dialog.onSubmit();

        expect(insertUser).toHaveBeenCalledWith(
            expect.objectContaining({ name: 'human', avatarInitials: 'ab' })
        );
    });

    it('sends an empty string when initials are left blank (auto-computed)', () => {
        openHumanForm().onSubmit();

        expect(insertUser).toHaveBeenCalledWith(expect.objectContaining({ avatarInitials: '' }));
    });

    it('refuses initials longer than two characters', () => {
        const dialog = openHumanForm();

        dialog.form.controls.avatarInitials.setValue('abc');
        dialog.onSubmit();

        expect(insertUser).not.toHaveBeenCalled();
    });

    it('updates the placeholder with the auto initials as the name is typed', () => {
        const dialog = openHumanForm();

        expect(dialog.autoInitials()).toBe('HU');

        dialog.form.controls.name.setValue('Ada Lovelace');

        expect(dialog.autoInitials()).toBe('AL');
    });
});
