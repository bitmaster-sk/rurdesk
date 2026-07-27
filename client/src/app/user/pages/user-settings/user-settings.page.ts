import {
    ChangeDetectionStrategy,
    Component,
    DestroyRef,
    OnInit,
    inject,
    signal
} from '@angular/core';
import { takeUntilDestroyed, toSignal } from '@angular/core/rxjs-interop';
import { FormBuilder, FormGroup, Validators } from '@angular/forms';
import { debounceTime, filter } from 'rxjs/operators';
import { UserService } from 'src/app/auth/user.service';
import { ToastNotificationService } from 'src/app/core/toast-notification.service';
import { Color } from 'src/app/shared/color/color';
import { UiSaveState } from 'src/app/ui/components/save-status/save-status-chip.component';

@Component({
    selector: 'app-user-settings',
    templateUrl: './user-settings.page.html',
    standalone: false,
    changeDetection: ChangeDetectionStrategy.OnPush
})
export class UserSettingsPage implements OnInit {
    private readonly sUser = inject(UserService);
    private readonly sToast = inject(ToastNotificationService);
    private readonly fb = inject(FormBuilder);
    private readonly destroyRef = inject(DestroyRef);

    public readonly currentUser = toSignal(this.sUser.user, {
        requireSync: true
    });

    // Per-field save chips so feedback appears next to the control the user
    // actually edited — a colour save must not light the chip over the name input.
    public readonly nameSaveStatus = signal<UiSaveState>(UiSaveState.Idle);
    public readonly colorSaveStatus = signal<UiSaveState>(UiSaveState.Idle);

    /** Live avatar background — reflects the colour control so the preview updates as you pick/shuffle. */
    public readonly avatarColor = signal<string>('');

    public profileForm!: FormGroup;
    public passwordForm!: FormGroup;

    public ngOnInit(): void {
        const user = this.currentUser();
        this.avatarColor.set(user?.colorAvatarBg ?? '');
        this.profileForm = this.fb.group({
            name: [
                user?.name,
                { validators: [Validators.required, Validators.maxLength(250)], updateOn: 'blur' }
            ],
            // Display only — the email is the login identifier, so changing it
            // needs its own flow (uniqueness, confirmation, session impact) and
            // updateUser deliberately has no email parameter. The input is
            // readonly rather than disabled so it stays selectable and copyable;
            // validators would be theatre on a value the user cannot edit.
            email: [user?.email],
            colorAvatarBg: [user?.colorAvatarBg]
        });
        this.passwordForm = this.fb.group({
            currentPassword: [
                '',
                [Validators.required, Validators.minLength(5), Validators.maxLength(100)]
            ],
            newPassword: [
                '',
                [Validators.required, Validators.minLength(5), Validators.maxLength(100)]
            ]
        });

        // Auto-save the name on blur — only when it's valid and actually changed.
        const nameControl = this.profileForm.get('name');
        nameControl.valueChanges
            .pipe(
                filter(() => nameControl.valid && nameControl.value !== this.currentUser()?.name),
                takeUntilDestroyed(this.destroyRef)
            )
            .subscribe(() => this.onSaveUser('name'));

        // The colour control drives the live preview immediately, and auto-saves
        // (debounced, since the native picker streams events while dragging).
        const colorControl = this.profileForm.get('colorAvatarBg');
        colorControl.valueChanges
            .pipe(takeUntilDestroyed(this.destroyRef))
            .subscribe((value: string) => this.avatarColor.set(value));
        colorControl.valueChanges
            .pipe(
                debounceTime(300),
                filter(() => colorControl.value !== this.currentUser()?.colorAvatarBg),
                takeUntilDestroyed(this.destroyRef)
            )
            .subscribe(() => this.onSaveUser('color'));
    }

    public onShuffleColor(): void {
        this.profileForm.get('colorAvatarBg').setValue(Color.randomAvatarBg());
    }

    public onSaveUser(source: 'name' | 'color' = 'name'): void {
        const status = source === 'color' ? this.colorSaveStatus : this.nameSaveStatus;
        status.set(UiSaveState.Saving);
        this.sUser
            .updateUser(this.profileForm.value.name, this.profileForm.value.colorAvatarBg)
            .subscribe({
                next: user => {
                    this.profileForm.patchValue(
                        { name: user.name, email: user.email, colorAvatarBg: user.colorAvatarBg },
                        { emitEvent: false }
                    );
                    this.avatarColor.set(user.colorAvatarBg);
                    status.set(UiSaveState.Saved);
                },
                error: () => {
                    status.set(UiSaveState.Error);
                    this.sToast.showError('USER.PROFILE_SAVE_FAILED');
                }
            });
    }

    public onChangePassword(): void {
        const { currentPassword, newPassword } = this.passwordForm.value;
        this.sUser.changePassword$(currentPassword, newPassword).subscribe({
            next: () => {
                this.passwordForm.reset();
                this.sToast.showSuccess('USER.PASSWORD_CHANGED');
            },
            error: () => this.sToast.showError('USER.PASSWORD_CHANGE_FAILED')
        });
    }
}
