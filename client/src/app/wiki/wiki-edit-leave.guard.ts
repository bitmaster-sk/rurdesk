import { CanDeactivateFn } from '@angular/router';
import { Observable } from 'rxjs';
import { WikiEditPage } from './pages/wiki-edit/wiki-edit.page';

export abstract class WikiEditLeaveGuard {
    public static readonly canDeactivate: CanDeactivateFn<WikiEditPage> = (
        page: WikiEditPage
    ): Observable<boolean> => page.confirmLeave$();
}
