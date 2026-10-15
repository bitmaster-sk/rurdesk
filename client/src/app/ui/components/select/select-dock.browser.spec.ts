import { Component, signal } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { FormControl, ReactiveFormsModule } from '@angular/forms';
import { provideNoopAnimations } from '@angular/platform-browser/animations';
import { TranslateModule } from '@ngx-translate/core';
import { UiModule } from '../../ui.module';
import { UiSelectOptionPicked } from './option-picked.event';

interface Opt {
    label: string;
    value: string;
}

@Component({
    standalone: false,
    template: `
        <div [style.width.px]="hostWidth()">
            <ui-select
                [options]="options()"
                optionLabel="label"
                optionValue="value"
                placeholder="Pick one"
                [formControl]="ctrl"
                (optionPicked)="onPicked($event)"
                (opened)="openedCount = openedCount + 1"
                (valueChanged)="changeCount = changeCount + 1"
            >
                <ng-template #optionActions let-option let-openDock="openDock">
                    <button type="button" class="test-action" (click)="openDock('info')">
                        {{ option.label }} info
                    </button>
                </ng-template>
                <ng-template #dock let-option let-kind="kind" let-close="close">
                    <div class="test-dock">
                        <span class="test-dock__label">{{ option.label }} / {{ kind }}</span>
                        <button type="button" class="test-dock__close" (click)="close()">x</button>
                    </div>
                </ng-template>
            </ui-select>
        </div>
    `
})
class HostComponent {
    public readonly options = signal<Opt[]>([
        { label: 'Alpha', value: 'a' },
        { label: 'Beta', value: 'b' }
    ]);
    public readonly hostWidth = signal(900);
    public readonly ctrl = new FormControl<string | null>(null);
    public openedCount = 0;
    public changeCount = 0;
    public readonly picked: UiSelectOptionPicked<Opt>[] = [];
    public isPreventing = false;

    public onPicked(event: UiSelectOptionPicked<Opt>): void {
        this.picked.push(event);
        if (this.isPreventing) {
            event.preventDefault();
        }
    }
}

abstract class Dom {
    public static trigger(el: HTMLElement): HTMLElement {
        return el.querySelector('.ui-select-trigger') as HTMLElement;
    }

    public static panelOptions(): HTMLElement[] {
        return Array.from(document.querySelectorAll('.ui-select-panel__option'));
    }

    public static actions(): HTMLElement[] {
        return Array.from(document.querySelectorAll('.test-action'));
    }

    public static dock(): HTMLElement | null {
        return document.querySelector('.test-dock');
    }

    public static panel(): HTMLElement | null {
        return document.querySelector('.ui-select-panel');
    }
}

describe('UiSelectComponent docks (browser)', () => {
    beforeEach(async () => {
        await TestBed.configureTestingModule({
            declarations: [HostComponent],
            imports: [UiModule, ReactiveFormsModule, TranslateModule.forRoot()],
            providers: [provideNoopAnimations()]
        }).compileComponents();
    });

    function setup() {
        const fixture = TestBed.createComponent(HostComponent);
        fixture.detectChanges();
        Dom.trigger(fixture.nativeElement).click();
        fixture.detectChanges();
        return fixture;
    }

    it('emits opened when the panel opens, so consumers can lazy-load', () => {
        const fixture = setup();
        expect(fixture.componentInstance.openedCount).toBe(1);
    });

    it('renders the option actions template per option', () => {
        setup();
        expect(Dom.actions().map(a => a.textContent?.trim())).toEqual(['Alpha info', 'Beta info']);
    });

    it('clicking an action opens the dock without selecting the option or closing the panel', () => {
        const fixture = setup();

        Dom.actions()[1].click();
        fixture.detectChanges();

        expect(Dom.dock()?.textContent).toContain('Beta / info');
        expect(fixture.componentInstance.ctrl.value).toBeNull();
        expect(fixture.componentInstance.changeCount).toBe(0);
        expect(Dom.panelOptions().length).toBe(2);
    });

    it('the dock close callback hides the dock and leaves the panel open', () => {
        const fixture = setup();
        Dom.actions()[0].click();
        fixture.detectChanges();

        (document.querySelector('.test-dock__close') as HTMLElement).click();
        fixture.detectChanges();

        expect(Dom.dock()).toBeNull();
        expect(Dom.panelOptions().length).toBe(2);
    });

    it('Escape closes the dock first, and only the second Escape closes the panel', () => {
        const fixture = setup();
        Dom.actions()[0].click();
        fixture.detectChanges();

        const t = Dom.trigger(fixture.nativeElement);
        t.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }));
        fixture.detectChanges();
        expect(Dom.dock()).toBeNull();
        expect(Dom.panelOptions().length).toBe(2);

        t.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }));
        fixture.detectChanges();
        expect(Dom.panelOptions().length).toBe(0);
    });

    it('falls back to a stacked dock when there is no room to its right', () => {
        const fixture = TestBed.createComponent(HostComponent);
        // Push the trigger to the right edge: a side dock would render off-screen.
        fixture.componentInstance.hostWidth.set(document.documentElement.clientWidth - 40);
        fixture.detectChanges();
        Dom.trigger(fixture.nativeElement).click();
        fixture.detectChanges();

        Dom.actions()[0].click();
        fixture.detectChanges();

        expect(Dom.panel()?.classList.contains('ui-select-panel--dock-below')).toBe(true);
    });

    it('reports a pick on the docked option together with its dock', () => {
        const fixture = setup();
        Dom.actions()[1].click();
        fixture.detectChanges();

        Dom.panelOptions()[1].click();
        fixture.detectChanges();

        const [picked] = fixture.componentInstance.picked;
        expect(picked.value).toBe('b');
        expect(picked.isDockedOption).toBe(true);
        expect(picked.dock).toEqual({ option: { label: 'Beta', value: 'b' }, kind: 'info' });
        expect(fixture.componentInstance.ctrl.value).toBe('b');
    });

    it('a prevented pick keeps the value, the panel and the dock', () => {
        const fixture = setup();
        fixture.componentInstance.isPreventing = true;
        Dom.actions()[1].click();
        fixture.detectChanges();

        Dom.panelOptions()[1].click();
        fixture.detectChanges();

        expect(fixture.componentInstance.ctrl.value).toBeNull();
        expect(fixture.componentInstance.changeCount).toBe(0);
        expect(Dom.dock()).not.toBeNull();
    });

    it('picking another option than the docked one says so and closes the dock', () => {
        const fixture = setup();
        Dom.actions()[1].click();
        fixture.detectChanges();

        Dom.panelOptions()[0].click();
        fixture.detectChanges();

        expect(fixture.componentInstance.picked[0].isDockedOption).toBe(false);
        expect(fixture.componentInstance.ctrl.value).toBe('a');
        expect(Dom.dock()).toBeNull();
    });

    it('a pick with no dock open reports no dock', () => {
        const fixture = setup();

        Dom.panelOptions()[0].click();
        fixture.detectChanges();

        expect(fixture.componentInstance.picked[0].dock).toBeNull();
        expect(fixture.componentInstance.ctrl.value).toBe('a');
    });
});
