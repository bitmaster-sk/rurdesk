import { ChangeDetectionStrategy, Component, OnInit, inject, signal } from '@angular/core';
import { HttpErrorResponse } from '@angular/common/http';
import {
    AbstractControl,
    FormBuilder,
    FormControl,
    FormGroup,
    ValidationErrors,
    Validators
} from '@angular/forms';
import { Router } from '@angular/router';
import { switchMap } from 'rxjs/operators';
import { AuthApi } from '../api/auth.api.service';
import { SessionService } from '../service/session.service';

interface CredentialsForm {
    password: FormControl<string | null>;
    password2: FormControl<string | null>;
}

interface RegisterForm {
    name: FormControl<string | null>;
    email: FormControl<string | null>;
    credentials: FormGroup<CredentialsForm>;
}

@Component({
    selector: 'app-register',
    templateUrl: './register.component.html',
    styleUrls: ['./register.component.scss'],
    changeDetection: ChangeDetectionStrategy.OnPush,
    standalone: false
})
export class RegisterComponent implements OnInit {
    private readonly fb = inject(FormBuilder);
    private readonly router = inject(Router);
    private readonly authApi = inject(AuthApi);
    private readonly session = inject(SessionService);

    public readonly form: FormGroup<RegisterForm> = this.buildForm();
    public readonly errorKey = signal<string | null>(null);

    public ngOnInit(): void {
        // Clear a previous failure as soon as the user edits the form.
        this.form.valueChanges.subscribe(() => this.errorKey.set(null));
    }

    public onRegister(): void {
        this.errorKey.set(null);
        if (this.form.invalid) {
            this.errorKey.set(
                this.form.controls.credentials.hasError('differentPasswords')
                    ? 'REGISTER.PASSWORD.MISMATCH'
                    : 'REGISTER.INVALID'
            );
            return;
        }

        const values = this.form.getRawValue();
        this.authApi
            .register$({
                name: values.name ?? '',
                email: values.email ?? '',
                password: values.credentials.password ?? ''
            })
            .pipe(
                switchMap(() =>
                    this.authApi.login$(
                        values.email ?? '',
                        values.credentials.password ?? '',
                        false
                    )
                )
            )
            .subscribe({
                next: token => this.session.start(token),
                error: err => this.onRegisterError(err)
            });
    }

    private onRegisterError(err: unknown): void {
        if (err instanceof HttpErrorResponse && err.url?.includes('/login')) {
            // The account was created but the follow-up auto-login failed —
            // let the user sign in manually instead of failing silently.
            void this.router.navigate(['/login']);
            return;
        }
        if (err instanceof HttpErrorResponse && err.status === 403) {
            // Registration is a one-time bootstrap; after the first user it is closed.
            this.errorKey.set('REGISTER.CLOSED');
            return;
        }
        this.errorKey.set('REGISTER.FAILED');
    }

    private buildForm(): FormGroup<RegisterForm> {
        return this.fb.group<RegisterForm>({
            name: this.fb.control<string | null>(null, [
                Validators.required,
                Validators.maxLength(250)
            ]),
            email: this.fb.control<string | null>(null, [
                Validators.required,
                Validators.email,
                Validators.maxLength(250)
            ]),
            credentials: this.fb.group<CredentialsForm>(
                {
                    password: this.fb.control<string | null>(null, [
                        Validators.required,
                        Validators.minLength(5),
                        Validators.maxLength(100)
                    ]),
                    password2: this.fb.control<string | null>(null, [
                        Validators.required,
                        Validators.minLength(5),
                        Validators.maxLength(100)
                    ])
                },
                { validators: [RegisterComponent.confirmPasswordCheck] }
            )
        });
    }

    private static isCredentialsGroup(
        control: AbstractControl
    ): control is FormGroup<CredentialsForm> {
        return (
            control instanceof FormGroup &&
            'password' in control.controls &&
            'password2' in control.controls
        );
    }

    private static confirmPasswordCheck(group: AbstractControl): ValidationErrors | null {
        if (!RegisterComponent.isCredentialsGroup(group)) {
            return null;
        }
        const password = group.controls.password.value ?? '';
        const password2 = group.controls.password2.value ?? '';

        return password === password2 ? null : { differentPasswords: true };
    }
}
