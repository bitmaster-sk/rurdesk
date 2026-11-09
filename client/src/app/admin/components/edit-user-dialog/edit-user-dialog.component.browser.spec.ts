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
