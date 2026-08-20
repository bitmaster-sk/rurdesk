import { BrowserModule } from '@angular/platform-browser';
import { NgModule } from '@angular/core';
import { AppRoutingModule } from './app-routing.module';
import { AppComponent } from './app.component';
import './app.chart';
import { AuthInterceptor } from './auth/auth.interceptor';
import { ErrorInterceptor } from './core/error.interceptor';
import { HTTP_INTERCEPTORS, provideHttpClient, withInterceptorsFromDi } from '@angular/common/http';
import { AuthModule } from './auth/auth.module';
import { CoreModule } from './core/core.module';
import { provideAnimationsAsync } from '@angular/platform-browser/animations/async';
import { provideTranslateService } from '@ngx-translate/core';
import { provideTranslateHttpLoader } from '@ngx-translate/http-loader';
import { MarkdownModule } from 'ngx-markdown';
import { MARKDOWN_MARKED_OPTIONS } from './shared/markdown/marked-options';
import { OVERLAY_DEFAULT_CONFIG } from '@angular/cdk/overlay';
import { UiToastModule } from './ui/ui-toast.module';

@NgModule({
    declarations: [AppComponent],
    imports: [
        BrowserModule,
        CoreModule,
        AppRoutingModule,
        AuthModule,
        MarkdownModule.forRoot({ markedOptions: MARKDOWN_MARKED_OPTIONS }),
        UiToastModule
    ],
    providers: [
        { provide: HTTP_INTERCEPTORS, useClass: AuthInterceptor, multi: true },
        { provide: HTTP_INTERCEPTORS, useClass: ErrorInterceptor, multi: true },
        { provide: OVERLAY_DEFAULT_CONFIG, useValue: { usePopover: false } },
        provideTranslateService({
            defaultLanguage: 'en',
            useDefaultLang: true,
            loader: provideTranslateHttpLoader({
                prefix: '../assets/i18n/',
                suffix: '.json',
                enforceLoading: true,
                // Load the public i18n JSON via HttpBackend, bypassing the DI interceptors.
                // The loader needs no auth header and must not trigger the 401-redirect /
                // error-toast logic; it also breaks the construction cycle ErrorInterceptor
                // → TranslateService → (loader) HttpClient → ErrorInterceptor that otherwise
                // stops translations from loading once withInterceptorsFromDi() is active.
                useHttpBackend: true
            })
        }),
        // withInterceptorsFromDi() is REQUIRED for the HTTP_INTERCEPTORS class
        // interceptors above (Auth/Error) to run — without it Angular silently
        // ignores them, so 401s never clear the token or redirect to /login.
        provideHttpClient(withInterceptorsFromDi()),
        provideAnimationsAsync()
    ],
    bootstrap: [AppComponent]
})
export class AppModule {}
