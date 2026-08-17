import { Injectable, inject } from '@angular/core';
import { Resolve } from '@angular/router';
import { TranslateService } from '@ngx-translate/core';
import { Observable } from 'rxjs';

@Injectable({ providedIn: 'root' })
export class TranslateResolver implements Resolve<void> {
    private readonly i18n = inject(TranslateService);

    public resolve(): Observable<void> | Promise<void> | void {
        return this.i18n.get('LOGIN');
    }
}
