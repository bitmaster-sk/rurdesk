import { CommonModule } from '@angular/common';
import { ComponentFixture, TestBed } from '@angular/core/testing';
import { ReactiveFormsModule } from '@angular/forms';
import { By } from '@angular/platform-browser';
import { TranslateModule, TranslateService } from '@ngx-translate/core';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { UiModule } from 'src/app/ui/ui.module';
import { UiTooltipDirective } from 'src/app/ui/directives/tooltip.directive';
import { TablerIconStub } from 'src/testing/stubs';
import en from 'src/assets/i18n/en.json';
import { SprintDialogComponent, SprintDialogSave } from './sprint-dialog.component';

interface DialogForm {
    form: { patchValue(value: Record<string, unknown>): void; invalid: boolean };
    onSubmit(): void;
}

describe('SprintDialogComponent', () => {
    let fixture: ComponentFixture<SprintDialogComponent>;

    const internals = (): DialogForm => fixture.componentInstance as unknown as DialogForm;

    beforeEach(async () => {
        await TestBed.configureTestingModule({
            declarations: [SprintDialogComponent],
            imports: [
                CommonModule,
                ReactiveFormsModule,
                TranslateModule.forRoot(),
                UiModule,
                TablerIconStub
            ]
        }).compileComponents();

        const translate = TestBed.inject(TranslateService);
        translate.setTranslation('en', en);
        translate.use('en');

        fixture = TestBed.createComponent(SprintDialogComponent);
        fixture.componentRef.setInput('sprint', null);
        fixture.componentRef.setInput('visible', true);
        fixture.detectChanges();
    });

    function submit(name: string, startAt: Date, endAt: Date): ReturnType<typeof vi.fn> {
        const saved = vi.fn();
        fixture.componentInstance.saved.subscribe((value: SprintDialogSave) => saved(value));
        internals().form.patchValue({ name, startAt, endAt });
        internals().onSubmit();
        fixture.detectChanges();
        return saved;
    }

    it('saves a window that ends after it starts, and closes', () => {
        const saved = submit('Cycle', new Date(2026, 7, 1), new Date(2026, 7, 15));

        expect(saved).toHaveBeenCalledWith({
            name: 'Cycle',
            startAt: '2026-08-01T00:00:00.000Z',
            endAt: '2026-08-15T00:00:00.000Z'
        });
        expect(fixture.componentInstance.visible()).toBe(false);
    });

    it('defaults to a 14-day window, including on the evening before a clock change', () => {
        vi.useFakeTimers();
        vi.setSystemTime(new Date(2026, 2, 28, 23, 30, 0));
        try {
            const saved = vi.fn();
            fixture.componentInstance.saved.subscribe((value: SprintDialogSave) => saved(value));
            fixture.componentRef.setInput('visible', false);
            fixture.detectChanges();
            fixture.componentRef.setInput('visible', true);
            fixture.detectChanges();

            internals().onSubmit();

            const { startAt, endAt } = saved.mock.calls[0][0] as SprintDialogSave;
            const days =
                (new Date(endAt).getTime() - new Date(startAt).getTime()) / (24 * 60 * 60 * 1000);
            expect(days).toBe(14);
        } finally {
            vi.useRealTimers();
        }
    });

    it('refuses to save a window that ends before it starts, and stays open', () => {
        const saved = submit('Backwards', new Date(2026, 7, 20), new Date(2026, 7, 10));

        expect(saved).not.toHaveBeenCalled();
        expect(fixture.componentInstance.visible()).toBe(true);
    });

    it('refuses to save a window that starts and ends on the same day', () => {
        const saved = submit('Zero', new Date(2026, 7, 20), new Date(2026, 7, 20));

        expect(saved).not.toHaveBeenCalled();
        expect(fixture.componentInstance.visible()).toBe(true);
    });

    it('says on screen why a refused window was not saved', () => {
        submit('Backwards', new Date(2026, 7, 20), new Date(2026, 7, 10));

        const hint = fixture.debugElement.query(By.css('[data-testid="sprint-hint"]'));
        expect(hint.nativeElement.textContent.trim()).toBe(
            'The end date must be after the start date'
        );
        expect(
            fixture.debugElement
                .queryAll(By.directive(UiTooltipDirective))
                .map(el => el.injector.get(UiTooltipDirective).uiTooltip())
        ).toContain('The end date must be after the start date');
    });

    it('says nothing once the form is valid', () => {
        internals().form.patchValue({
            name: 'Cycle',
            startAt: new Date(2026, 7, 1),
            endAt: new Date(2026, 7, 15)
        });
        fixture.detectChanges();

        expect(fixture.debugElement.query(By.css('[data-testid="sprint-hint"]'))).toBeNull();
    });
});
