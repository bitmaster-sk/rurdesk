import {
    ChangeDetectionStrategy,
    Component,
    computed,
    effect,
    inject,
    input,
    model,
    output,
    signal
} from '@angular/core';
import { FormBuilder, Validators } from '@angular/forms';
import { forkJoin, map } from 'rxjs';
import { ToastNotificationService } from 'src/app/core/toast-notification.service';
import { Color } from 'src/app/shared/color/color';
import { AdminApi } from '../../api/admin.api.service';
import { AdminUpdateUserReq, AdminUser, BotGateway } from '../../model/admin-user.model';

/**
 * EditUserDialogComponent edits an existing user — same shape as create but the
 * bot flag is fixed (shown read-only). A bot edits only its name; a human also
 * edits email and the admin flag. Password is never changed here.
 */
@Component({
    selector: 'app-edit-user-dialog',
    templateUrl: './edit-user-dialog.component.html',
    standalone: false,
    changeDetection: ChangeDetectionStrategy.OnPush
})
export class EditUserDialogComponent {
    private readonly fb = inject(FormBuilder);
    private readonly adminApi = inject(AdminApi);
    private readonly sToast = inject(ToastNotificationService);

    public readonly user = input<AdminUser | null>(null);
    public readonly visible = model<boolean>(false);
    public readonly updated = output<void>();

    protected readonly isSaving = signal(false);
    protected readonly isBot = computed(() => this.user()?.isBot ?? false);
    // The bot's existing gateway, loaded on open. null = bot has none yet
    // (URL is then managed in the keys window, not here).
    protected readonly gateway = signal<BotGateway | null>(null);

    protected readonly form = this.fb.group({
        name: ['', [Validators.required, Validators.maxLength(250)]],
        // Read-only — the bot flag can't change after creation.
        isBot: [{ value: false, disabled: true }],
        email: ['', [Validators.email, Validators.maxLength(250)]],
        isAdmin: [false],
        gatewayUrl: ['', [Validators.maxLength(2000)]],
        colorAvatarBg: ['']
    });

    constructor() {
        // Re-seed the form whenever a different user is opened for editing.
        effect(() => {
            const user = this.user();
            if (!this.visible() || !user) {
                return;
            }
            this.form.reset({
                name: user.name,
                isBot: user.isBot,
                email: user.email,
                isAdmin: user.isAdmin,
                gatewayUrl: '',
                colorAvatarBg: user.colorAvatarBg
            });
            const email = this.form.controls.email;
            const gatewayUrl = this.form.controls.gatewayUrl;
            if (user.isBot) {
                email.clearValidators();
                // Gateway URL is editable only once the gateway exists; required then.
                this.gateway.set(null);
                this.adminApi.getBotGateway$(user.idUser).subscribe(gw => {
                    this.gateway.set(gw);
                    if (gw) {
                        gatewayUrl.setValue(gw.gatewayUrl);
                        gatewayUrl.setValidators([Validators.required, Validators.maxLength(2000)]);
                    } else {
                        gatewayUrl.clearValidators();
                    }
                    gatewayUrl.updateValueAndValidity();
                });
            } else {
                email.setValidators([
                    Validators.required,
                    Validators.email,
                    Validators.maxLength(250)
                ]);
                gatewayUrl.clearValidators();
            }
            email.updateValueAndValidity();
            gatewayUrl.updateValueAndValidity();
        });
    }

    protected onSubmit(): void {
        const user = this.user();
        if (!user || this.form.invalid) {
            this.form.markAllAsTouched();
            return;
        }
        const value = this.form.getRawValue();
        const req: AdminUpdateUserReq = user.isBot
            ? { name: value.name!.trim(), colorAvatarBg: value.colorAvatarBg! }
            : {
                  name: value.name!.trim(),
                  email: value.email!.trim(),
                  isAdmin: value.isAdmin!,
                  colorAvatarBg: value.colorAvatarBg!
              };

        // For a bot with an existing gateway, save a changed URL alongside the name.
        const gw = this.gateway();
        const newUrl = value.gatewayUrl!.trim();
        const calls: ReturnType<AdminApi['updateUser$']>[] = [
            this.adminApi.updateUser$(user.idUser, req)
        ];
        if (user.isBot && gw && newUrl && newUrl !== gw.gatewayUrl) {
            calls.push(
                this.adminApi
                    .updateBotGatewayUrl$(user.idUser, { gatewayUrl: newUrl })
                    .pipe(map(() => void 0))
            );
        }

        this.isSaving.set(true);
        forkJoin(calls).subscribe({
            next: () => {
                this.updated.emit();
                this.isSaving.set(false);
                this.visible.set(false);
            },
            error: () => {
                this.isSaving.set(false);
                this.sToast.showError('ADMIN.ACTION_FAILED');
            }
        });
    }

    protected onShuffleColor(): void {
        this.form.controls.colorAvatarBg.setValue(Color.randomAvatarBg());
    }

    protected onCancel(): void {
        this.visible.set(false);
    }
}
