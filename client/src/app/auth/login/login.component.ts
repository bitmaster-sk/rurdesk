import { AuthApi } from '../api/auth.api.service';
import { SessionService } from '../service/session.service';
import { AuthTokenStore } from '../store/auth-token.store';
import { Component, OnInit, inject, signal } from '@angular/core';
import { FormBuilder, FormControl, FormGroup, Validators } from '@angular/forms';

interface LoginForm {
    email: FormControl<string>;
    password: FormControl<string>;
}

@Component({
    selector: 'app-login',
    templateUrl: './login.component.html',
    styleUrls: ['./login.component.scss'],
    standalone: false
})
export class LoginComponent implements OnInit {
    private fb = inject(FormBuilder);
    private authApi = inject(AuthApi);
    private tokenStore = inject(AuthTokenStore);
    private session = inject(SessionService);

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
        this.form.valueChanges.subscribe(() => this.hasFailed.set(false));
    }

    public onLogin(): void {
        this.hasFailed.set(false);
        const credentials = this.form.getRawValue();
        this.authApi.login$(credentials.email, credentials.password).subscribe({
            next: token => this.session.start(token),
            error: () => this.hasFailed.set(true)
        });
    }

    private buildForm(): FormGroup<LoginForm> {
        return this.fb.group({
            email: this.fb.nonNullable.control('', [
                Validators.email,
                Validators.maxLength(250)
            ]),
            password: this.fb.nonNullable.control('', [
                Validators.minLength(5),
                Validators.maxLength(100)
            ])
        });
    }
}
