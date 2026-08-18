import {
    ChangeDetectionStrategy,
    Component,
    effect,
    inject,
    input,
    model,
    signal
} from '@angular/core';
import { FormControl, FormGroup, Validators } from '@angular/forms';
import { ToastNotificationService } from '../../../core/toast-notification.service';
import { AdminApi } from '../../api/admin.api.service';
import { AdminUser, BotApiKey, BotGateway } from '../../model/admin-user.model';

const DEFAULT_KEY_NAME = 'default';

/**
 * BotKeysDialogComponent manages one bot's two credentials. Both follow the same
 * single-credential model: register/create when none exists, then regenerate
 * (rotate in place) or delete. A freshly minted token is shown once inline — it
 * cannot be retrieved later.
 *
 * - Gateway → Tracker token (the bot's API key).
 * - Tracker → Gateway token (the bot's gateway webhook secret).
 */
@Component({
    selector: 'app-bot-keys-dialog',
    templateUrl: './bot-keys-dialog.component.html',
    styleUrls: ['./bot-keys-dialog.component.scss'],
    standalone: false,
    changeDetection: ChangeDetectionStrategy.OnPush
})
export class BotKeysDialogComponent {
    private readonly adminApi = inject(AdminApi);
    private readonly sToast = inject(ToastNotificationService);

    public readonly bot = input<AdminUser | null>(null);
    public readonly visible = model<boolean>(false);
    // Tokens freshly minted during one-shot bot creation, revealed once when the
    // dialog opens straight after create. Null when opened to manage an existing bot.
    public readonly presetRevealedKey = input<string | null>(null);
    public readonly presetRevealedGatewayToken = input<string | null>(null);

    protected readonly key = signal<BotApiKey | null>(null);
    protected readonly isKeyBusy = signal(false);
    protected readonly revealedKey = signal<string | null>(null);

    protected readonly gateway = signal<BotGateway | null>(null);
    protected readonly isGatewayBusy = signal(false);
    protected readonly revealedGatewayToken = signal<string | null>(null);

    protected readonly gatewayForm = new FormGroup({
        gatewayUrl: new FormControl('', {
            nonNullable: true,
            validators: [Validators.required]
        })
    });

    public constructor() {
        effect(() => {
            if (this.visible() && this.bot()) {
                this.loadKey();
                this.loadGateway();
                this.revealedKey.set(this.presetRevealedKey());
                this.revealedGatewayToken.set(this.presetRevealedGatewayToken());
            }
        });
    }

    private loadKey(): void {
        const bot = this.bot();
        if (!bot) {
            return;
        }
        this.adminApi.getBotKey$(bot.idUser).subscribe(key => this.key.set(key));
    }

    private loadGateway(): void {
        const bot = this.bot();
        if (!bot) {
            return;
        }
        this.adminApi.getBotGateway$(bot.idUser).subscribe(gw => this.gateway.set(gw));
    }

    protected onCreateKey(): void {
        const bot = this.bot();
        if (!bot) {
            return;
        }
        this.isKeyBusy.set(true);
        // The token name is not editable from the UI — mint with a fixed default.
        this.adminApi.createBotKey$(bot.idUser, DEFAULT_KEY_NAME).subscribe({
            next: res => {
                const { rawKey, ...key } = res;
                this.key.set(key);
                this.revealedKey.set(rawKey);
                this.isKeyBusy.set(false);
            },
            error: () => {
                this.isKeyBusy.set(false);
                this.sToast.showError('API_KEY.CREATE_FAILED');
            }
        });
    }

    protected onRegenerateKey(): void {
        const bot = this.bot();
        if (!bot) {
            return;
        }
        this.isKeyBusy.set(true);
        this.adminApi.regenerateBotKey$(bot.idUser).subscribe({
            next: res => {
                const { rawKey, ...key } = res;
                this.key.set(key);
                this.revealedKey.set(rawKey);
                this.isKeyBusy.set(false);
            },
            error: () => {
                this.isKeyBusy.set(false);
                this.sToast.showError('API_KEY.REGENERATE_FAILED');
            }
        });
    }

    protected onConfirmDeleteKey(): void {
        const bot = this.bot();
        if (!bot) {
            return;
        }
        this.adminApi.deleteBotKey$(bot.idUser).subscribe({
            next: () => {
                this.key.set(null);
                this.revealedKey.set(null);
            },
            error: () => this.sToast.showError('API_KEY.REVOKE_FAILED')
        });
    }

    protected onCopy(): void {
        const key = this.revealedKey();
        if (key) {
            this.copyToClipboard(key);
        }
    }

    protected onCreateGateway(): void {
        const bot = this.bot();
        if (!bot || this.gatewayForm.invalid) {
            this.gatewayForm.markAllAsTouched();
            return;
        }
        this.isGatewayBusy.set(true);
        this.adminApi
            .createBotGateway$(bot.idUser, {
                gatewayUrl: this.gatewayForm.controls.gatewayUrl.value.trim()
            })
            .subscribe({
                next: res => {
                    const { trackerToGatewayToken, ...gw } = res;
                    this.gateway.set(gw);
                    this.revealedGatewayToken.set(trackerToGatewayToken);
                    this.isGatewayBusy.set(false);
                },
                error: () => {
                    this.isGatewayBusy.set(false);
                    this.sToast.showError('API_KEY.GATEWAY_CREATE_FAILED');
                }
            });
    }

    protected onRegenerateGatewayToken(): void {
        const bot = this.bot();
        if (!bot) {
            return;
        }
        this.isGatewayBusy.set(true);
        this.adminApi.regenerateGatewayToken$(bot.idUser).subscribe({
            next: res => {
                this.revealedGatewayToken.set(res.trackerToGatewayToken);
                this.isGatewayBusy.set(false);
            },
            error: () => {
                this.isGatewayBusy.set(false);
                this.sToast.showError('API_KEY.GATEWAY_REGENERATE_FAILED');
            }
        });
    }

    protected onConfirmDeleteGateway(): void {
        const bot = this.bot();
        if (!bot) {
            return;
        }
        this.adminApi.deleteBotGateway$(bot.idUser).subscribe({
            next: () => {
                this.gateway.set(null);
                this.revealedGatewayToken.set(null);
            },
            error: () => this.sToast.showError('API_KEY.GATEWAY_DELETE_FAILED')
        });
    }

    protected onCopyGatewayToken(): void {
        const token = this.revealedGatewayToken();
        if (token) {
            this.copyToClipboard(token);
        }
    }

    private copyToClipboard(text: string): void {
        navigator.clipboard.writeText(text).catch(() => {
            this.sToast.showError('API_KEY.COPY_FAILED');
        });
    }

    protected onClose(): void {
        this.revealedKey.set(null);
        this.revealedGatewayToken.set(null);
        this.gatewayForm.reset();
        this.visible.set(false);
    }
}
