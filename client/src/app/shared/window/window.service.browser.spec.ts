import { describe, beforeEach, expect, it } from 'vitest';
import { Component, ErrorHandler } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { TranslateModule } from '@ngx-translate/core';
import { WindowModule } from './window.module';
import { WindowService } from './window.service';

@Component({ selector: 'app-window-test-content', template: 'content', standalone: false })
class WindowTestContentComponent {}

class RecordingErrorHandler implements ErrorHandler {
    public readonly errors: unknown[] = [];

    public handleError(error: unknown): void {
        this.errors.push(error);
    }
}

describe('WindowService', () => {
    let errorHandler: RecordingErrorHandler;

    beforeEach(() => {
        errorHandler = new RecordingErrorHandler();
        TestBed.configureTestingModule({
            declarations: [WindowTestContentComponent],
            imports: [WindowModule, TranslateModule.forRoot()],
            providers: [WindowService, { provide: ErrorHandler, useValue: errorHandler }]
        });
    });

    function openWindow(): void {
        TestBed.inject(WindowService)
            .open(WindowTestContentComponent, { header: 'Test' } as WindowConfig)
            .onClose.subscribe();
        TestBed.inject(WindowService); // ensure DI graph is realised before CD
        TestBed.flushEffects();
    }

    function closeButton(): HTMLElement {
        return document.querySelector('.ui-dialog__close') as HTMLElement;
    }

    it('closes via the × button without corrupting the view tree', async () => {
        openWindow();
        expect(closeButton()).not.toBeNull();

        closeButton().click();
        TestBed.flushEffects();
        await Promise.resolve();

        expect(errorHandler.errors).toEqual([]);
        expect(document.querySelector('.ui-dialog__pane')).toBeNull();
    });
});
