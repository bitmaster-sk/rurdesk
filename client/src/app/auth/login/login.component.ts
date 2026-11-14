import { AuthApi } from '../api/auth.api.service';
import { PASSWORD_ONLY } from '../constants/sso.constants';
import { AuthMethods } from '../model/auth-methods.model';
import { SsoConverter } from '../converter/sso.converter';
import { SessionService } from '../service/session.service';
import { AuthTokenStore } from '../store/auth-token.store';
import {
    ChangeDetectionStrategy,
    Component,
    DestroyRef,
    OnInit,
    computed,
    inject,
    signal
} from '@angular/core';
import { takeUntilDestroyed, toSignal } from '@angular/core/rxjs-interop';
import { FormBuilder, FormControl, FormGroup, Validators } from '@angular/forms';
import { ActivatedRoute } from '@angular/router';
import { of } from 'rxjs';
import { catchError, map } from 'rxjs/operators';

interface LoginForm {
    email: FormControl<string | null>;
    password: FormControl<string | null>;
    hasExtendedSessionLifetime: FormControl<boolean | null>;
}

@Component({
    selector: 'app-login',
    templateUrl: './login.component.html',
    styleUrls: ['./login.component.scss'],
    standalone: false,
    changeDetection: ChangeDetectionStrategy.OnPush
})
export class LoginComponent implements OnInit {
    private fb = inject(FormBuilder);
    private authApi = inject(AuthApi);
    private tokenStore = inject(AuthTokenStore);
    private session = inject(SessionService);

    private readonly route = inject(ActivatedRoute);

    private destroyRef = inject(DestroyRef);
    public readonly form: FormGroup<LoginForm> = this.buildForm();
    public readonly hasFailed = signal(false);
    public readonly passwordVisible = signal(false);
    private readonly isPasswordChosen = signal(false);

    public readonly methods = toSignal<AuthMethods | null>(
        this.authApi.loadMethods$().pipe(catchError(() => of(PASSWORD_ONLY))),
        { initialValue: null }
    );

    public readonly ssoErrorKey = toSignal(
        this.route.queryParamMap.pipe(
            map(params => params.get('ssoError')),
            map(code => (code ? SsoConverter.toErrorKey(code) : null))
        ),
        { initialValue: null }
    );

    private readonly isExtended = toSignal(
        this.form.controls.hasExtendedSessionLifetime.valueChanges.pipe(
            map(value => value ?? false)
        ),
        { initialValue: false }
    );

    public readonly ssoStartUrl = computed(() =>
        SsoConverter.toStartUrl(!!this.methods()?.sso?.canStaySignedIn && this.isExtended())
    );

    public readonly isPasswordShown = computed(
        () => !this.methods()?.sso || this.isPasswordChosen()
    );

    public ngOnInit(): void {
        if (this.tokenStore.hasToken()) {
            this.session.end();
        }
        // Clear a previous failure as soon as the user edits their credentials.
        this.form.valueChanges
            .pipe(takeUntilDestroyed(this.destroyRef))
            .subscribe(() => this.hasFailed.set(false));
    }

    public showPassword(isShown: boolean): void {
        this.isPasswordChosen.set(isShown);
        this.hasFailed.set(false);
    }

    public togglePasswordVisibility(): void {
        this.passwordVisible.update(value => !value);
    }

    public onLogin(): void {
        if (this.form.invalid) {
            return;
        }
        this.hasFailed.set(false);
        const { email, password, hasExtendedSessionLifetime } = this.form.value;
        this.authApi
            .login$(email ?? '', password ?? '', hasExtendedSessionLifetime ?? false)
            .subscribe({
                next: token => this.session.start(token),
                error: () => this.hasFailed.set(true)
            });
    }

    private buildForm(): FormGroup<LoginForm> {
        return this.fb.group<LoginForm>({
            email: this.fb.control<string | null>(null, [
                Validators.required,
                Validators.email,
                Validators.maxLength(250)
            ]),
            password: this.fb.control<string | null>(null, [
                Validators.required,
                Validators.minLength(5),
                Validators.maxLength(100)
            ]),
            hasExtendedSessionLifetime: this.fb.control<boolean | null>(false)
        });
    }
}
