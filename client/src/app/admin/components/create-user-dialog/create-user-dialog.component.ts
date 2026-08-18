import { ChangeDetectionStrategy, Component, inject, model, output, signal } from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { FormBuilder, Validators } from '@angular/forms';
import { ToastNotificationService } from 'src/app/core/toast-notification.service';
import { Color } from 'src/app/shared/color/color';
import { AdminApi } from '../../api/admin.api.service';
import {
    AdminCreateUserReq,
    AdminCreateUserRes,
    UserCreatedEvent
} from '../../model/admin-user.model';

/**
 * CreateUserDialogComponent creates a human user or a bot. When "Is bot" is on, email and
 * password are hidden (the backend auto-generates a synthetic email and mints an API key);
 * the parent reveals the returned rawKey once. Project assignment is done separately via the
 * project members page.
 */
@Component({
    selector: 'app-create-user-dialog',
    templateUrl: './create-user-dialog.component.html',
    standalone: false,
    changeDetection: ChangeDetectionStrategy.OnPush
})
export class CreateUserDialogComponent {
    private readonly fb = inject(FormBuilder);
    private readonly adminApi = inject(AdminApi);
    private readonly sToast = inject(ToastNotificationService);

    public readonly visible = model<boolean>(false);
    public readonly created = output<UserCreatedEvent>();

    protected readonly isSaving = signal(false);

    protected readonly form = this.fb.group({
        name: ['', [Validators.required, Validators.maxLength(250)]],
        isBot: [false],
        isAdmin: [false],
        email: ['', [Validators.email, Validators.maxLength(250)]],
        password: ['', [Validators.minLength(5), Validators.maxLength(100)]],
        // Optional: when set on a bot, its gateway is created in the same step.
        gatewayUrl: ['', [Validators.maxLength(2000)]],
        colorAvatarBg: [Color.randomAvatarBg()]
    });

    public constructor() {
        // Re-apply the email/password/gateway validators whenever the bot flag
        // flips. Driven off the control's valueChanges (not a DOM event) so it
        // always runs AFTER the value is written to the model, which a native
        // checkbox's change event doesn't guarantee.
        this.form.controls.isBot.valueChanges
            .pipe(takeUntilDestroyed())
            .subscribe(() => this.onToggleBot());
    }

    protected onToggleBot(): void {
        const isBot = this.form.controls.isBot.value;
        const email = this.form.controls.email;
        const password = this.form.controls.password;
        const gatewayUrl = this.form.controls.gatewayUrl;
        if (isBot) {
            email.clearValidators();
            password.clearValidators();
            // A bot is only useful with a gateway — its URL is required so the
            // Tracker → Gateway token is created alongside the bot.
            gatewayUrl.setValidators([Validators.required, Validators.maxLength(2000)]);
        } else {
            email.setValidators([Validators.required, Validators.email, Validators.maxLength(250)]);
            password.setValidators([
                Validators.required,
                Validators.minLength(5),
                Validators.maxLength(100)
            ]);
            gatewayUrl.clearValidators();
        }
        email.updateValueAndValidity();
        password.updateValueAndValidity();
        gatewayUrl.updateValueAndValidity();
    }

    protected onSubmit(): void {
        if (this.form.invalid) {
            this.form.markAllAsTouched();
            return;
        }
        const value = this.form.getRawValue();
        const req: AdminCreateUserReq = value.isBot!
            ? { name: value.name!, isBot: true, colorAvatarBg: value.colorAvatarBg! }
            : {
                  name: value.name!,
                  isBot: false,
                  email: value.email!,
                  password: value.password!,
                  isAdmin: value.isAdmin!,
                  colorAvatarBg: value.colorAvatarBg!
              };

        const gatewayUrl = value.isBot ? (value.gatewayUrl ?? '').trim() : '';

        this.isSaving.set(true);
        this.adminApi.createUser$(req).subscribe({
            next: res => {
                if (gatewayUrl) {
                    this.createGatewayThenFinish(res, gatewayUrl);
                } else {
                    this.finish({ user: res, gatewayToken: null });
                }
            },
            error: () => this.isSaving.set(false)
        });
    }

    // One-shot: the bot exists, now register its gateway so the keys window can
    // reveal both tokens at once. A gateway failure is non-fatal — the bot is
    // already created, so we still open the keys window (token can be added there).
    private createGatewayThenFinish(user: AdminCreateUserRes, gatewayUrl: string): void {
        this.adminApi.createBotGateway$(user.idUser, { gatewayUrl }).subscribe({
            next: gw => this.finish({ user, gatewayToken: gw.trackerToGatewayToken }),
            error: () => {
                this.sToast.showError('API_KEY.GATEWAY_CREATE_FAILED');
                this.finish({ user, gatewayToken: null });
            }
        });
    }

    private finish(event: UserCreatedEvent): void {
        this.created.emit(event);
        this.isSaving.set(false);
        this.close();
    }

    protected onShuffleColor(): void {
        this.form.controls.colorAvatarBg.setValue(Color.randomAvatarBg());
    }

    protected onCancel(): void {
        this.close();
    }

    private close(): void {
        this.form.reset({ isBot: false, isAdmin: false, colorAvatarBg: Color.randomAvatarBg() });
        this.onToggleBot();
        this.visible.set(false);
    }
}
