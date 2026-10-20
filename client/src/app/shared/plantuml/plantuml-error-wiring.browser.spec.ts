import { HTTP_INTERCEPTORS, provideHttpClient, withInterceptorsFromDi } from '@angular/common/http';
import { HttpTestingController, provideHttpClientTesting } from '@angular/common/http/testing';
import { TestBed } from '@angular/core/testing';
import { TranslateModule } from '@ngx-translate/core';
import { of } from 'rxjs';
import { describe, expect, it, vi } from 'vitest';
import { ErrorInterceptor } from 'src/app/core/error.interceptor';
import { ToastNotificationService } from 'src/app/core/toast-notification.service';
import { I18nService } from 'src/app/shared/i18n/i18n.service';
import { SettingsStore } from 'src/app/core/settings/settings.store';
import { PlantUmlApi } from './plantuml.api.service';
import { PlantUmlDiagramComponent } from './plantuml-diagram.component';

describe('PlantUml render errors through ErrorInterceptor (browser)', () => {
    const showError = vi.fn();
    const i18n = { get$: (key: string) => of(key) } as unknown as I18nService;

    async function setup() {
        await TestBed.configureTestingModule({
            declarations: [PlantUmlDiagramComponent],
            imports: [TranslateModule.forRoot()],
            providers: [
                provideHttpClient(withInterceptorsFromDi()),
                provideHttpClientTesting(),
                { provide: HTTP_INTERCEPTORS, useClass: ErrorInterceptor, multi: true },
                PlantUmlApi,
                { provide: I18nService, useValue: i18n },
                { provide: ToastNotificationService, useValue: { showError } },
                { provide: SettingsStore, useValue: { isPlantUmlEnabled: () => true } }
            ]
        }).compileComponents();
        const fixture = TestBed.createComponent(PlantUmlDiagramComponent);
        fixture.componentRef.setInput('source', 'Alice -> Bob: hi');
        fixture.autoDetectChanges();
        return fixture;
    }

    it('maps a 422 to the syntax error with the body message, without a toast', async () => {
        const fixture = await setup();

        const ctrl = TestBed.inject(HttpTestingController);
        const req = ctrl.expectOne('/api/private/plantuml/svg');
        req.flush(
            {
                code: 'PLANTUML_SYNTAX',
                message: 'syntax fail',
                translateKey: 'error.plantuml_syntax'
            },
            { status: 422, statusText: 'Unprocessable Entity' }
        );

        await vi.waitFor(() => {
            const error = fixture.nativeElement.querySelector('[data-testid="plantuml-error"]');
            expect(error.textContent).toContain('syntax fail');
        });
        expect(showError).not.toHaveBeenCalled();
        ctrl.verify();
    });

    it('maps a 502 to the unavailable error, without a toast', async () => {
        const fixture = await setup();

        const ctrl = TestBed.inject(HttpTestingController);
        const req = ctrl.expectOne('/api/private/plantuml/svg');
        req.flush(
            {
                code: 'PLANTUML_UNAVAILABLE',
                message: 'down',
                translateKey: 'error.plantuml_unavailable'
            },
            { status: 502, statusText: 'Bad Gateway' }
        );

        await vi.waitFor(() => {
            const error = fixture.nativeElement.querySelector('[data-testid="plantuml-error"]');
            expect(error.textContent).toContain('SHARED.PLANTUML.UNAVAILABLE');
        });
        expect(showError).not.toHaveBeenCalled();
        ctrl.verify();
    });
});
