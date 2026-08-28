import { Directive, ElementRef, inject } from '@angular/core';

/**
 * Makes a non-interactive element behave like a button for keyboard users:
 * puts it in the tab order, exposes `role="button"`, and turns Enter/Space
 * into the same `click` the mouse would fire — so the element's existing
 * `(click)` binding needs no change.
 *
 * Only for containers that cannot BE a `<button>` — because they already hold
 * other controls (nested buttons are invalid HTML) or double as a drop target.
 * Anywhere a native `<button>`/`<a>` fits, use that instead.
 */
@Directive({
    selector: '[uiActivatable]',
    standalone: false,
    host: {
        'role': 'button',
        'tabindex': '0',
        '(keydown)': 'onKeydown($event)'
    }
})
export class UiActivatableDirective {
    private readonly el = inject<ElementRef<HTMLElement>>(ElementRef);

    protected onKeydown(event: KeyboardEvent): void {
        if (event.key !== 'Enter' && event.key !== ' ') {
            return;
        }
        // Keys pressed on a nested control belong to that control — without this
        // guard Space on an inner button would also activate the container.
        if (event.target !== this.el.nativeElement) {
            return;
        }
        event.preventDefault();
        this.el.nativeElement.click();
    }
}
