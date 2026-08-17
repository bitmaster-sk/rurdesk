import { UserService } from './../user.service';
import { Component, OnInit, inject, signal } from '@angular/core';
import { FormBuilder, FormGroup, Validators } from '@angular/forms';
import { Router } from '@angular/router';
import { SettingsStore } from '../../core/settings/settings.store';

@Component({
    selector: 'app-login',
    templateUrl: './login.component.html',
    styleUrls: ['./login.component.scss'],
    standalone: false
})
export class LoginComponent implements OnInit {
    private fb = inject(FormBuilder);
    private router = inject(Router);
    private sUser = inject(UserService);
    private settingsStore = inject(SettingsStore);

    public form: FormGroup;
    public readonly hasFailed = signal(false);

    public constructor() {
        this.form = this.buildForm();
    }

    public ngOnInit(): void {
        if (this.sUser.hasAuthLocal()) {
            this.logout();
        }
        // Clear a previous failure as soon as the user edits their credentials.
        this.form.valueChanges.subscribe(() => this.hasFailed.set(false));
    }

    public onLogin(): void {
        this.hasFailed.set(false);
        const credentials = this.form.value as { email: string; password: string };
        this.sUser.login(credentials.email, credentials.password).subscribe({
            next: token => this.onLoginSuccess(token),
            error: () => this.hasFailed.set(true)
        });
    }

    private onLoginSuccess(token: string): void {
        this.sUser.saveAuthLocal(token);
        // App bootstrap skips the auth-gated settings load for anonymous visitors;
        // now that we have a token, load them (navigation does not re-bootstrap).
        this.settingsStore.load();
        this.router.navigate(['/']);
    }

    private logout(): void {
        this.sUser.logout().subscribe({
            next: () => this.onLogoutSuccess(),
            error: () => this.onLogoutSuccess()
        });
    }

    private onLogoutSuccess(): void {
        this.sUser.deleteAuthLocal();
    }

    private buildForm(): FormGroup {
        return this.fb.group({
            email: this.fb.control(null, [Validators.email, Validators.maxLength(250)]),
            password: this.fb.control(null, [Validators.minLength(5), Validators.maxLength(100)])
        });
    }
}
