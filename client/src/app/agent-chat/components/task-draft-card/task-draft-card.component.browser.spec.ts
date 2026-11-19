import { CUSTOM_ELEMENTS_SCHEMA } from '@angular/core';
import { ComponentFixture, TestBed } from '@angular/core/testing';
import { TranslateModule } from '@ngx-translate/core';
import { page } from '@vitest/browser/context';
import { TablerIconStub, UiButtonStub } from 'src/testing/stubs';
import { TaskDraftCardComponent } from './task-draft-card.component';

describe('TaskDraftCardComponent (browser)', () => {
    let fixture: ComponentFixture<TaskDraftCardComponent>;

    beforeEach(async () => {
        await page.viewport(1280, 800);
        await TestBed.configureTestingModule({
            declarations: [TaskDraftCardComponent],
            imports: [TranslateModule.forRoot(), TablerIconStub, UiButtonStub],
            schemas: [CUSTOM_ELEMENTS_SCHEMA]
        }).compileComponents();
    });

    // The component specs load no global stylesheet: give the tokens the card uses.
    function withTokens(): void {
        const style = document.createElement('style');
        style.textContent =
            ':root { --ui-space: 0.5rem; --ui-space-s: 0.25rem; --ui-space-l: 1rem; --ui-color-border: #e6dbd1; }';
        document.head.appendChild(style);
        onTestFinished(() => style.remove());
    }

    function render(): HTMLElement {
        fixture = TestBed.createComponent(TaskDraftCardComponent);
        fixture.componentRef.setInput(
            'body',
            '# Finish the formatter\n\n## Goal\n\nValidate input.'
        );
        fixture.detectChanges();
        return fixture.nativeElement as HTMLElement;
    }

    function expectFramed(host: HTMLElement): void {
        const card = host.querySelector<HTMLElement>('[data-testid="task-draft-card"]')!;
        const title = host.querySelector<HTMLElement>('[data-testid="task-draft-card-title"]')!;
        const style = getComputedStyle(card);

        expect(parseFloat(style.borderTopWidth)).toBeGreaterThanOrEqual(1);
        expect(parseFloat(style.borderLeftWidth)).toBeGreaterThanOrEqual(1);
        expect(title.getBoundingClientRect().left).toBeGreaterThan(
            card.getBoundingClientRect().left + 4
        );
    }

    it('sets the draft apart from the answer with a frame', () => {
        withTokens();

        expectFramed(render());
    });

    describe('at phone width', () => {
        beforeEach(async () => {
            await page.viewport(390, 844);
        });

        afterEach(async () => {
            await page.viewport(1280, 800);
        });

        it('keeps the frame so the draft does not read as part of the answer', () => {
            withTokens();

            expectFramed(render());
        });
    });
});
