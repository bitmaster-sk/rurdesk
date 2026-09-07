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
import { ClipboardService } from '../../../core/clipboard.service';
import { ToastNotificationService } from '../../../core/toast-notification.service';
import { AgentApiKeyApi } from '../../api/agent-api-key.api.service';
import { AgentGatewayApi } from '../../api/agent-gateway.api.service';
import { AdminUser } from '../../model/admin-user.model';
import { AgentApiKey } from '../../model/agent-api-key.model';
import { AgentGateway } from '../../model/agent-gateway.model';

const DEFAULT_KEY_NAME = 'default';

/**
 * AgentKeysDialogComponent manages one agent's two credentials. Both follow the same
 * single-credential model: register/create when none exists, then regenerate
 * (rotate in place) or delete. A freshly minted token is shown once inline — it
 * cannot be retrieved later.
 *
 * - Gateway → Tracker token (the agent's API key).
 * - Tracker → Gateway token (the agent's gateway webhook secret).
 */
@Component({
    selector: 'app-agent-keys-dialog',
    templateUrl: './agent-keys-dialog.component.html',
    styleUrls: ['./agent-keys-dialog.component.scss'],
    standalone: false,
    changeDetection: ChangeDetectionStrategy.OnPush
})
export class AgentKeysDialogComponent {
    private readonly agentKeyApi = inject(AgentApiKeyApi);
    private readonly agentGatewayApi = inject(AgentGatewayApi);
    private readonly sToast = inject(ToastNotificationService);
    private readonly clipboard = inject(ClipboardService);

    public readonly agent = input<AdminUser | null>(null);
    public readonly visible = model<boolean>(false);
    // Tokens freshly minted during one-shot agent creation, revealed once when the
    // dialog opens straight after create. Null when opened to manage an existing agent.
    public readonly presetRevealedKey = input<string | null>(null);
    public readonly presetRevealedGatewayToken = input<string | null>(null);

    protected readonly key = signal<AgentApiKey | null>(null);
    protected readonly isKeyBusy = signal(false);
    protected readonly revealedKey = signal<string | null>(null);

    protected readonly gateway = signal<AgentGateway | null>(null);
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
            if (this.visible() && this.agent()) {
                this.loadKey();
                this.loadGateway();
                this.revealedKey.set(this.presetRevealedKey());
                this.revealedGatewayToken.set(this.presetRevealedGatewayToken());
            }
        });
    }

    private loadKey(): void {
        const agent = this.agent();
        if (!agent) {
            return;
        }
        this.agentKeyApi.load$(agent.idUser).subscribe(key => this.key.set(key));
    }

    private loadGateway(): void {
        const agent = this.agent();
        if (!agent) {
            return;
        }
        this.agentGatewayApi.load$(agent.idUser).subscribe(gw => this.gateway.set(gw));
    }

    protected onCreateKey(): void {
        const agent = this.agent();
        if (!agent) {
            return;
        }
        this.isKeyBusy.set(true);
        // The token name is not editable from the UI — mint with a fixed default.
        this.agentKeyApi.insert$(agent.idUser, DEFAULT_KEY_NAME).subscribe({
            next: res => {
                const { rawKey, ...key } = res;
                this.key.set(key);
                this.revealedKey.set(rawKey);
                this.isKeyBusy.set(false);
            },
            error: () => {
                this.isKeyBusy.set(false);
                this.sToast.showError('AGENT_API_KEY.CREATE_FAILED');
            }
        });
    }

    protected onRegenerateKey(): void {
        const agent = this.agent();
        if (!agent) {
            return;
        }
        this.isKeyBusy.set(true);
        this.agentKeyApi.regenerate$(agent.idUser).subscribe({
            next: res => {
                const { rawKey, ...key } = res;
                this.key.set(key);
                this.revealedKey.set(rawKey);
                this.isKeyBusy.set(false);
            },
            error: () => {
                this.isKeyBusy.set(false);
                this.sToast.showError('AGENT_API_KEY.REGENERATE_FAILED');
            }
        });
    }

    protected onConfirmDeleteKey(): void {
        const agent = this.agent();
        if (!agent) {
            return;
        }
        this.agentKeyApi.revoke$(agent.idUser).subscribe({
            next: () => {
                this.key.set(null);
                this.revealedKey.set(null);
            },
            error: () => this.sToast.showError('AGENT_API_KEY.REVOKE_FAILED')
        });
    }

    protected onCopy(): void {
        const key = this.revealedKey();
        if (key) {
            this.copyToClipboard(key);
        }
    }

    protected onCreateGateway(): void {
        const agent = this.agent();
        if (!agent || this.gatewayForm.invalid) {
            this.gatewayForm.markAllAsTouched();
            return;
        }
        this.isGatewayBusy.set(true);
        this.agentGatewayApi
            .insert$(agent.idUser, {
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
                    this.sToast.showError('AGENT_API_KEY.GATEWAY_CREATE_FAILED');
                }
            });
    }

    protected onRegenerateGatewayToken(): void {
        const agent = this.agent();
        if (!agent) {
            return;
        }
        this.isGatewayBusy.set(true);
        this.agentGatewayApi.regenerateToken$(agent.idUser).subscribe({
            next: res => {
                this.revealedGatewayToken.set(res.trackerToGatewayToken);
                this.isGatewayBusy.set(false);
            },
            error: () => {
                this.isGatewayBusy.set(false);
                this.sToast.showError('AGENT_API_KEY.GATEWAY_REGENERATE_FAILED');
            }
        });
    }

    protected onConfirmDeleteGateway(): void {
        const agent = this.agent();
        if (!agent) {
            return;
        }
        this.agentGatewayApi.delete$(agent.idUser).subscribe({
            next: () => {
                this.gateway.set(null);
                this.revealedGatewayToken.set(null);
            },
            error: () => this.sToast.showError('AGENT_API_KEY.GATEWAY_DELETE_FAILED')
        });
    }

    protected onCopyGatewayToken(): void {
        const token = this.revealedGatewayToken();
        if (token) {
            this.copyToClipboard(token);
        }
    }

    private copyToClipboard(text: string): void {
        void this.clipboard.copy(text).then(isCopied => {
            if (isCopied) {
                this.sToast.showSuccess('AGENT_API_KEY.COPIED');
            } else {
                this.sToast.showError('AGENT_API_KEY.COPY_FAILED');
            }
        });
    }

    protected onClose(): void {
        this.revealedKey.set(null);
        this.revealedGatewayToken.set(null);
        this.gatewayForm.reset();
        this.visible.set(false);
    }
}
