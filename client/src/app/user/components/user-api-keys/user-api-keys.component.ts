import {
    ChangeDetectionStrategy,
    Component,
    OnInit,
    computed,
    inject,
    signal
} from '@angular/core';
import { FormControl, FormGroup } from '@angular/forms';
import { endOfDay } from 'date-fns';
import { ClipboardService } from '../../../core/clipboard.service';
import { SettingsStore } from '../../../core/settings/settings.store';
import { ToastNotificationService } from '../../../core/toast-notification.service';
import { LoadState, LoadStateUtil } from '../../../shared/model/load-state.model';
import { FutureDateValidator } from '../../../shared/validators/future-date.validator';
import { NotBlankValidator } from '../../../shared/validators/not-blank.validator';
import { UserApiKeyApi } from '../../api/user-api-key.api.service';
import { UserApiKey } from '../../model/user-api-key.model';

interface KeyFormControls {
    name: FormControl<string>;
    expiresAt: FormControl<Date | null>;
}

interface RevealedKey {
    idApiKey: number;
    rawKey: string;
}

@Component({
    selector: 'app-user-api-keys',
    templateUrl: './user-api-keys.component.html',
    standalone: false,
    changeDetection: ChangeDetectionStrategy.OnPush
})
export class UserApiKeysComponent implements OnInit {
    private readonly api = inject(UserApiKeyApi);
    private readonly settings = inject(SettingsStore);
    private readonly clipboard = inject(ClipboardService);
    private readonly sToast = inject(ToastNotificationService);

    protected readonly state = signal<LoadState<UserApiKey[]>>(LoadStateUtil.idle());
    protected readonly hasLoadFailed = signal(false);
    protected readonly isCreateBusy = signal(false);
    protected readonly idRegeneratingApiKey = signal<number | null>(null);
    // Shown once, right after create or regenerate — the raw value is never
    // retrievable again.
    protected readonly revealed = signal<RevealedKey | null>(null);

    protected readonly keys = computed(() => this.state().data ?? []);
    protected readonly isLoading = computed(() => this.state().isLoading);
    protected readonly limit = this.settings.userApiKeyLimit;
    protected readonly isAtLimit = computed(() => this.keys().length >= this.limit());

    protected readonly form = new FormGroup<KeyFormControls>({
        name: new FormControl('', {
            nonNullable: true,
            validators: [NotBlankValidator.validate]
        }),
        expiresAt: new FormControl<Date | null>(null, {
            validators: [FutureDateValidator.validate]
        })
    });

    public ngOnInit(): void {
        this.load();
    }

    private load(): void {
        this.state.update(current => LoadStateUtil.loading(current.data));
        this.api.load$().subscribe({
            next: keys => {
                this.hasLoadFailed.set(false);
                this.state.set(LoadStateUtil.loaded(keys));
            },
            error: () => {
                this.hasLoadFailed.set(true);
                this.state.set(LoadStateUtil.loaded(this.state().data));
            }
        });
    }

    protected onCreate(): void {
        if (this.form.invalid || this.isAtLimit()) {
            this.form.markAllAsTouched();
            return;
        }
        const { name, expiresAt } = this.form.getRawValue();
        this.isCreateBusy.set(true);
        this.api
            .insert$({
                name: name.trim(),
                // End of day, so a key picked for "31 Dec" does not expire at
                // 23:00 the day before once local midnight is converted to UTC.
                expiresAt: expiresAt ? endOfDay(expiresAt).toISOString() : null
            })
            .subscribe({
                next: created => {
                    const { rawKey, ...key } = created;
                    this.state.update(current =>
                        LoadStateUtil.loaded([key, ...(current.data ?? [])])
                    );
                    this.revealed.set({ idApiKey: key.idApiKey, rawKey });
                    this.form.reset({ name: '', expiresAt: null });
                    this.isCreateBusy.set(false);
                },
                error: () => {
                    this.isCreateBusy.set(false);
                    // The server is the authority on the limit; resync so a stale
                    // count cannot keep the form enabled or disabled wrongly.
                    this.load();
                }
            });
    }

    protected onRegenerate(idApiKey: number): void {
        this.idRegeneratingApiKey.set(idApiKey);
        this.api.regenerate$(idApiKey).subscribe({
            next: created => {
                const { rawKey, ...key } = created;
                this.state.update(current =>
                    LoadStateUtil.loaded(
                        (current.data ?? []).map(k => (k.idApiKey === idApiKey ? key : k))
                    )
                );
                this.revealed.set({ idApiKey, rawKey });
                this.idRegeneratingApiKey.set(null);
            },
            error: () => this.idRegeneratingApiKey.set(null)
        });
    }

    protected onRevoke(idApiKey: number): void {
        this.api.revoke$(idApiKey).subscribe({
            next: () => {
                this.state.update(current =>
                    LoadStateUtil.loaded((current.data ?? []).filter(k => k.idApiKey !== idApiKey))
                );
                if (this.revealed()?.idApiKey === idApiKey) {
                    this.revealed.set(null);
                }
            },
            // Swallowed on purpose: ErrorInterceptor already toasts the server's
            // message, and the row must stay because the server still has it.
            error: () => {}
        });
    }

    protected onCopy(): void {
        const revealed = this.revealed();
        if (!revealed) {
            return;
        }
        void this.clipboard.copy(revealed.rawKey).then(isCopied => {
            if (isCopied) {
                this.sToast.showSuccess('USER_API_KEY.COPIED');
            } else {
                this.sToast.showError('USER_API_KEY.COPY_FAILED');
            }
        });
    }

    protected onDismissRevealed(): void {
        this.revealed.set(null);
    }

    protected onRetryLoad(): void {
        this.load();
    }
}
