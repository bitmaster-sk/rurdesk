import { AuthApi } from '../api/auth.api.service';
import { SessionService } from '../service/session.service';
import { AuthTokenStore } from '../store/auth-token.store';
import { Component, OnInit, inject, signal } from '@angular/core';
import { FormBuilder, FormGroup, Validators } from '@angular/forms';

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

    public form: FormGroup;
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
        const credentials = this.form.value as { email: string; password: string };
        this.authApi.login$(credentials.email, credentials.password).subscribe({
            next: token => this.session.start(token),
            error: () => this.hasFailed.set(true)
        });
    }

    private buildForm(): FormGroup {
        return this.fb.group({
            email: this.fb.control(null, [Validators.email, Validators.maxLength(250)]),
            password: this.fb.control(null, [Validators.minLength(5), Validators.maxLength(100)])
        });
    }
}
