import { ChangeDetectionStrategy, Component, inject } from '@angular/core';
import { Router } from '@angular/router';
import { TranslateModule } from '@ngx-translate/core';
import { UiModule } from '../../ui/ui.module';

/**
 * Global 404 page. Standalone so it can be mounted on a top-level, auth-independent
 * route (reachable logged in or out) rather than only inside the authenticated shell.
 */
@Component({
    selector: 'app-not-found',
    standalone: true,
    imports: [TranslateModule, UiModule],
    templateUrl: './not-found.page.html',
    styleUrls: ['./not-found.page.scss'],
    changeDetection: ChangeDetectionStrategy.OnPush
})
export class NotFoundPage {
    private readonly router = inject(Router);

    protected onHome(): void {
        this.router.navigateByUrl('/');
    }
}
