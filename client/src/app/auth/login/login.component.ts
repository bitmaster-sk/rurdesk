import { AuthApi } from '../api/auth.api.service';
import { SessionService } from '../service/session.service';
import { AuthTokenStore } from '../store/auth-token.store';
import {
    ChangeDetectionStrategy,
    Component,
    DestroyRef,
    OnInit,
    inject,
    signal
} from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { FormBuilder, FormControl, FormGroup, Validators } from '@angular/forms';

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

    private destroyRef = inject(DestroyRef);
    public form: FormGroup<LoginForm>;
    public readonly hasFailed = signal(false);

    public constructor() {
        this.form = this.buildForm();
    }

    public ngOnInit(): void {
        if (this.tokenStore.hasToken()) {
            this.session.end();
        }
        // Clear a previous failure as soon as the user edits their credentials.
        this.form.valueChanges
            .pipe(takeUntilDestroyed(this.destroyRef))
            .subscribe(() => this.hasFailed.set(false));
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
