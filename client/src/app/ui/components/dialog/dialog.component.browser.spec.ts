import { Component, OnDestroy, OnInit } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { By } from '@angular/platform-browser';
import { provideNoopAnimations } from '@angular/platform-browser/animations';
import { TranslateModule } from '@ngx-translate/core';
import { OVERLAY_DEFAULT_CONFIG } from '@angular/cdk/overlay';
import { UiModule } from '../../ui.module';
import { UiDialogComponent } from './dialog.component';

let initCount = 0;
let destroyCount = 0;

@Component({
    selector: 'test-dialog-child',
    standalone: false,
    template: `
        <span class="child">child</span>
    `
})
class ChildComponent implements OnInit, OnDestroy {
    public ngOnInit(): void {
        initCount++;
    }
    public ngOnDestroy(): void {
        destroyCount++;
    }
}

@Component({
    standalone: false,
    template: `
        <button #trigger (click)="open = true">open</button>
        <ui-dialog
            [(visible)]="open"
            [header]="header"
            [ariaLabel]="ariaLabel"
            [closable]="closable"
            [closeOnEscape]="closeOnEscape"
            [dismissable]="dismissable"
            [panelClass]="panelClass"
            (hide)="hideCount = hideCount + 1"
        >
            <test-dialog-child />
            @if (withFooter) {
                <ng-template #footer><button class="foot">ok</button></ng-template>
            }
        </ui-dialog>
    `
})
class HostComponent {
    public open = false;
    public header: string | undefined = 'My dialog';
    public ariaLabel: string | undefined;
    public closable = true;
    public closeOnEscape = true;
    public dismissable = false;
    public withFooter = false;
    public panelClass = '';
    public hideCount = 0;
}

describe('UiDialogComponent (browser)', () => {
    function panel(): HTMLElement | null {
        return document.querySelector('.ui-dialog');
    }
    function backdrop(): HTMLElement | null {
        return document.querySelector('.ui-dialog__backdrop');
    }
    function pressEscape(defaultPrevented = false): void {
        const event = new KeyboardEvent('keydown', {
            key: 'Escape',
            bubbles: true,
            cancelable: true
        });
        if (defaultPrevented) {
            event.preventDefault();
        }
        document.body.dispatchEvent(event);
    }

    beforeEach(async () => {
        initCount = 0;
        destroyCount = 0;
        await TestBed.configureTestingModule({
            declarations: [HostComponent, ChildComponent],
            imports: [UiModule, TranslateModule.forRoot()],
            providers: [provideNoopAnimations()]
        }).compileComponents();
    });

    function setup(configure?: (host: HostComponent) => void) {
        const fixture = TestBed.createComponent(HostComponent);
        configure?.(fixture.componentInstance);
        fixture.detectChanges();
        return fixture;
    }
    function dialogOf(fixture: ReturnType<typeof setup>): UiDialogComponent {
        return fixture.debugElement.query(By.directive(UiDialogComponent)).componentInstance;
    }

    it('is closed initially', () => {
        setup();
        expect(panel()).toBeNull();
    });

    it('attaches the overlay and projects content when visible becomes true', () => {
        const fixture = setup(h => (h.open = true));
        expect(panel()).not.toBeNull();
        expect(panel()!.querySelector('.child')?.textContent).toContain('child');
    });

    it('detaches the overlay when visible becomes false', () => {
        const fixture = setup(h => (h.open = true));
        dialogOf(fixture).visible.set(false);
        fixture.detectChanges();
        expect(panel()).toBeNull();
    });

    it('closes and emits hide on × click', () => {
        const fixture = setup(h => (h.open = true));
        (panel()!.querySelector('.ui-dialog__close') as HTMLElement).click();
        fixture.detectChanges();
        expect(panel()).toBeNull();
        expect(fixture.componentInstance.open).toBe(false);
        expect(fixture.componentInstance.hideCount).toBe(1);
    });

    it('hides the × button when not closable', () => {
        setup(h => {
            h.open = true;
            h.closable = false;
        });
        expect(panel()!.querySelector('.ui-dialog__close')).toBeNull();
    });

    it('renders the #footer template into the footer slot', () => {
        setup(h => {
            h.open = true;
            h.withFooter = true;
        });
        expect(panel()!.querySelector('.ui-dialog__footer .foot')).not.toBeNull();
    });

    it('closes on Escape when closable and closeOnEscape', () => {
        const fixture = setup(h => (h.open = true));
        pressEscape();
        fixture.detectChanges();
        expect(panel()).toBeNull();
        expect(fixture.componentInstance.hideCount).toBe(1);
    });

    it('does NOT close on Escape when not closable', () => {
        const fixture = setup(h => {
            h.open = true;
            h.closable = false;
        });
        pressEscape();
        fixture.detectChanges();
        expect(panel()).not.toBeNull();
    });

    it('does NOT close on Escape when closeOnEscape is false', () => {
        const fixture = setup(h => {
            h.open = true;
            h.closeOnEscape = false;
        });
        pressEscape();
        fixture.detectChanges();
        expect(panel()).not.toBeNull();
    });

    it('does NOT close on an already-handled (defaultPrevented) Escape', () => {
        const fixture = setup(h => (h.open = true));
        pressEscape(true);
        fixture.detectChanges();
        expect(panel()).not.toBeNull();
    });

    it('does NOT close on backdrop click by default (dismissable=false)', () => {
        const fixture = setup(h => (h.open = true));
        backdrop()!.click();
        fixture.detectChanges();
        expect(panel()).not.toBeNull();
    });

    it('closes on backdrop click when dismissable', () => {
        const fixture = setup(h => {
            h.open = true;
            h.dismissable = true;
        });
        backdrop()!.click();
        fixture.detectChanges();
        expect(panel()).toBeNull();
    });

    it('sets aria-labelledby to the title when a header is present', () => {
        setup(h => (h.open = true));
        const p = panel()!;
        const labelledby = p.getAttribute('aria-labelledby');
        expect(labelledby).toBeTruthy();
        expect(p.querySelector(`#${labelledby}`)?.textContent).toContain('My dialog');
        expect(p.getAttribute('aria-label')).toBeNull();
    });

    it('falls back to aria-label when there is no header', () => {
        setup(h => {
            h.open = true;
            h.header = undefined;
            h.ariaLabel = 'Split issue';
        });
        const p = panel()!;
        expect(p.getAttribute('aria-label')).toBe('Split issue');
        expect(p.getAttribute('aria-labelledby')).toBeNull();
    });

    it('applies a multi-class panelClass without throwing (space-separated)', () => {
        setup(h => {
            h.open = true;
            h.panelClass = 'split-dialog split-dialog--wide';
        });
        const p = panel()!;
        expect(p.classList.contains('split-dialog')).toBe(true);
        expect(p.classList.contains('split-dialog--wide')).toBe(true);
    });

    it('installs a focus trap while open (anchors present)', () => {
        setup(h => (h.open = true));
        expect(document.querySelector('.cdk-focus-trap-anchor')).not.toBeNull();
    });

    it('restores focus to the trigger on close', () => {
        const fixture = setup();
        const trigger = fixture.nativeElement.querySelector('button') as HTMLElement;
        trigger.focus();
        trigger.click(); // opens
        fixture.detectChanges();
        dialogOf(fixture).close();
        fixture.detectChanges();
        expect(document.activeElement).toBe(trigger);
    });

    it('constructs projected content eagerly and destroys it with the host', () => {
        const fixture = setup(); // closed
        expect(initCount).toBe(1); // eager: content built with the parent view, not on open
        fixture.destroy();
        expect(destroyCount).toBe(1);
    });

    it('does not leak the overlay when the host is destroyed while open', () => {
        const fixture = setup(h => (h.open = true));
        expect(panel()).not.toBeNull();
        fixture.destroy();
        expect(document.querySelector('.ui-dialog')).toBeNull();
        expect(document.querySelector('.ui-dialog__backdrop')).toBeNull();
    });

    it('keeps overlay and signal in sync on same-tick set(false);set(true)', () => {
        const fixture = setup(h => (h.open = true));
        const dialog = dialogOf(fixture);
        dialog.visible.set(false);
        dialog.visible.set(true);
        fixture.detectChanges();
        expect(panel()).not.toBeNull(); // guard keeps the single overlay
        expect(fixture.componentInstance.hideCount).toBe(0);
    });

    it('attaches nothing on same-tick set(true);set(false) from closed', () => {
        const fixture = setup(); // closed
        const dialog = dialogOf(fixture);
        dialog.visible.set(true);
        dialog.visible.set(false);
        fixture.detectChanges();
        expect(panel()).toBeNull();
        expect(fixture.componentInstance.hideCount).toBe(0);
    });
});

describe('UiDialogComponent (browser) — body-level popups stack over the dialog', () => {
    function openDialog() {
        const fixture = TestBed.createComponent(HostComponent);
        fixture.componentInstance.open = true;
        fixture.detectChanges();
        return fixture;
    }

    function bodyPopupOverDialog(): { probe: [number, number]; el: HTMLElement } {
        const dialogRect = document.querySelector('.ui-dialog')!.getBoundingClientRect();
        const el = document.createElement('div');
        el.className = 'body-level-popup';
        el.style.cssText = `position:absolute;z-index:99999;left:${
            dialogRect.left + window.scrollX
        }px;top:${dialogRect.top + window.scrollY}px;width:60px;height:60px;background:#fff;`;
        document.body.appendChild(el);
        return { probe: [dialogRect.left + 10, dialogRect.top + 10], el };
    }

    afterEach(() => {
        document.querySelectorAll('.body-level-popup').forEach(el => el.remove());
    });

    it('paints a body-level popup above the dialog when usePopover is off', async () => {
        TestBed.resetTestingModule();
        await TestBed.configureTestingModule({
            declarations: [HostComponent, ChildComponent],
            imports: [UiModule, TranslateModule.forRoot()],
            providers: [
                provideNoopAnimations(),
                { provide: OVERLAY_DEFAULT_CONFIG, useValue: { usePopover: false } }
            ]
        }).compileComponents();

        openDialog();
        expect(document.querySelector('.cdk-overlay-popover')).toBeNull();

        const { probe, el } = bodyPopupOverDialog();
        expect(document.elementsFromPoint(probe[0], probe[1])[0]).toBe(el);
    });

    it('cannot be escaped by z-index once the dialog is in the top layer', async () => {
        TestBed.resetTestingModule();
        await TestBed.configureTestingModule({
            declarations: [HostComponent, ChildComponent],
            imports: [UiModule, TranslateModule.forRoot()],
            providers: [
                provideNoopAnimations(),
                { provide: OVERLAY_DEFAULT_CONFIG, useValue: { usePopover: true } }
            ]
        }).compileComponents();

        openDialog();
        const wrapper = document.querySelector('.cdk-overlay-popover');
        expect(wrapper).not.toBeNull();
        expect(wrapper!.matches(':popover-open')).toBe(true);

        const { probe, el } = bodyPopupOverDialog();
        expect(document.elementsFromPoint(probe[0], probe[1])[0]).not.toBe(el);
    });
});
