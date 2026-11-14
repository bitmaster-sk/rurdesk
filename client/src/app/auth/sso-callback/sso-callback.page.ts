import {
    ChangeDetectionStrategy,
    Component,
    DestroyRef,
    OnInit,
    inject,
    signal
} from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { ActivatedRoute } from '@angular/router';
import { AuthApi } from '../api/auth.api.service';
import { SsoConverter } from '../converter/sso.converter';
import { SessionService } from '../service/session.service';

@Component({
    selector: 'app-sso-callback',
    templateUrl: './sso-callback.page.html',
    standalone: false,
    changeDetection: ChangeDetectionStrategy.OnPush
})
export class SsoCallbackPage implements OnInit {
    private readonly route = inject(ActivatedRoute);
    private readonly authApi = inject(AuthApi);
    private readonly session = inject(SessionService);
    private readonly destroyRef = inject(DestroyRef);

    protected readonly hasFailed = signal(false);

    public ngOnInit(): void {
        const code = SsoConverter.toCode(this.route.snapshot.fragment);
        if (!code) {
            this.hasFailed.set(true);
            return;
        }
        this.authApi
            .exchangeSso$(code)
            .pipe(takeUntilDestroyed(this.destroyRef))
            .subscribe({
                next: res => this.session.start(res.token, res.returnUrl),
                error: () => this.hasFailed.set(true)
            });
    }
}
