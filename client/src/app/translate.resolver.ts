import { Injectable, inject } from '@angular/core';
import { Resolve } from '@angular/router';
import { Observable, map } from 'rxjs';
import { I18nService } from './shared/i18n/i18n.service';

@Injectable({ providedIn: 'root' })
export class TranslateResolver implements Resolve<void> {
    private readonly i18n = inject(I18nService);

    public resolve(): Observable<void> {
        return this.i18n.get$('LOGIN').pipe(map((): void => undefined));
    }
}
