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
    maxTurnsPlan: 120,
    maxTurnsImplement: null
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
                { provide: ToastNotificationService, useValue: { showError: vi.fn() } }
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
            isAdmin: false
        });
        fixture.componentRef.setInput('visible', true);
        fixture.detectChanges();
        return fixture.componentInstance as unknown as EditUserDialogInternals;
    }

    it('loads the agent turn limits into the form', () => {
        const dialog = open();

        expect(dialog.form.controls.maxTurnsPlan.value).toBe(120);
        expect(dialog.form.controls.maxTurnsImplement.value).toBeNull();
    });

    it('saves a changed turn limit together with the unchanged URL', () => {
        const dialog = open();

        dialog.form.controls.maxTurnsImplement.setValue(700);
        dialog.onSubmit();

        expect(updateGateway).toHaveBeenCalledWith(8, {
            gatewayUrl: 'http://gw:9090',
            maxTurnsPlan: 120,
            maxTurnsImplement: 700
        });
    });

    it('a cleared limit is sent as null so the app setting applies again', () => {
        const dialog = open();

        dialog.form.controls.maxTurnsPlan.setValue(null);
        dialog.onSubmit();

        expect(updateGateway).toHaveBeenCalledWith(8, {
            gatewayUrl: 'http://gw:9090',
            maxTurnsPlan: null,
            maxTurnsImplement: null
        });
    });

    it('a changed URL keeps the turn limits', () => {
        const dialog = open();

        dialog.form.controls.gatewayUrl.setValue('http://other:9090');
        dialog.onSubmit();

        expect(updateGateway).toHaveBeenCalledWith(8, {
            gatewayUrl: 'http://other:9090',
            maxTurnsPlan: 120,
            maxTurnsImplement: null
        });
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
                { provide: ToastNotificationService, useValue: { showError: vi.fn() } }
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
            isAdmin: false
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
});
