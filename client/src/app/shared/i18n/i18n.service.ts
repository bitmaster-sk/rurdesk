import { Injectable, inject } from '@angular/core';
import { InterpolationParameters, LangChangeEvent, TranslateService } from '@ngx-translate/core';
import { Observable, map } from 'rxjs';

/**
 * The only way the app talks to ngx-translate — `TranslateService` is not injected
 * anywhere else, so the untyped `instant()` cannot leak back in.
 */
@Injectable({ providedIn: 'root' })
export class I18nService {
    private readonly translate = inject(TranslateService);

    public get langChange$(): Observable<LangChangeEvent> {
        return this.translate.onLangChange;
    }

    public get currentLang(): string | undefined {
        return this.translate.currentLang || undefined;
    }

    // ngx-translate declares `Translation = StrictTranslation | any`, which collapses to `any`;
    // a type alias cannot be augmented, so the narrowing cast has to live here and nowhere else.
    public instant(key: string, params?: InterpolationParameters): string {
        return this.translate.instant(key, params) as string;
    }

    public get$(key: string, params?: InterpolationParameters): Observable<string> {
        return this.translate.get(key, params).pipe(map((value): string => value as string));
    }

    public getAll$(
        keys: string[],
        params?: InterpolationParameters
    ): Observable<Record<string, string>> {
        return this.translate
            .get(keys, params)
            .pipe(map((value): Record<string, string> => value as Record<string, string>));
    }
}
