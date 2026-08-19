import { TestBed } from '@angular/core/testing';
import { ReactiveFormsModule } from '@angular/forms';
import { of } from 'rxjs';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { ClipboardService } from 'src/app/core/clipboard.service';
import { ToastNotificationService } from 'src/app/core/toast-notification.service';
import { AdminApi } from '../../api/admin.api.service';
import { BotKeysDialogComponent } from './bot-keys-dialog.component';

interface BotKeysDialogInternals {
    onCopy(): void;
    onCopyGatewayToken(): void;
}

describe('BotKeysDialogComponent — token copy feedback (browser)', () => {
    let copy: ReturnType<typeof vi.fn>;
    let showSuccess: ReturnType<typeof vi.fn>;
    let showError: ReturnType<typeof vi.fn>;

    beforeEach(async () => {
        copy = vi.fn().mockResolvedValue(true);
        showSuccess = vi.fn();
        showError = vi.fn();

        await TestBed.configureTestingModule({
            declarations: [BotKeysDialogComponent],
            imports: [ReactiveFormsModule],
            providers: [
                {
                    provide: AdminApi,
                    useValue: {
                        getBotKey$: () => of(null),
                        getBotGateway$: () => of(null)
                    }
                },
                { provide: ToastNotificationService, useValue: { showSuccess, showError } },
                { provide: ClipboardService, useValue: { copy } }
            ]
        })
            .overrideComponent(BotKeysDialogComponent, { set: { template: '' } })
            .compileComponents();
    });

    function open(revealedKey: string | null, revealedGatewayToken: string | null) {
        const fixture = TestBed.createComponent(BotKeysDialogComponent);
        fixture.componentRef.setInput('bot', { idUser: 1, name: 'bot' });
        fixture.componentRef.setInput('presetRevealedKey', revealedKey);
        fixture.componentRef.setInput('presetRevealedGatewayToken', revealedGatewayToken);
        fixture.componentRef.setInput('visible', true);
        fixture.detectChanges();
        return fixture.componentInstance as unknown as BotKeysDialogInternals;
    }

    it('copies the revealed API key and confirms success', async () => {
        open('rd_secret_key', null).onCopy();
        await Promise.resolve();

        expect(copy).toHaveBeenCalledWith('rd_secret_key');
        expect(showSuccess).toHaveBeenCalledWith('API_KEY.COPIED');
        expect(showError).not.toHaveBeenCalled();
    });

    it('copies the revealed gateway token', async () => {
        open(null, 'rd_gateway_token').onCopyGatewayToken();
        await Promise.resolve();

        expect(copy).toHaveBeenCalledWith('rd_gateway_token');
        expect(showSuccess).toHaveBeenCalledWith('API_KEY.COPIED');
    });

    it('tells the user to copy manually when the clipboard is unavailable', async () => {
        copy.mockResolvedValue(false);

        open('rd_secret_key', null).onCopy();
        await Promise.resolve();
        await Promise.resolve();

        expect(showError).toHaveBeenCalledWith('API_KEY.COPY_FAILED');
        expect(showSuccess).not.toHaveBeenCalled();
    });

    it('does not touch the clipboard when no token is revealed', () => {
        open(null, null).onCopy();

        expect(copy).not.toHaveBeenCalled();
    });
});
