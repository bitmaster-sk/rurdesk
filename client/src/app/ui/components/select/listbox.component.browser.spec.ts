import { Component, signal } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { provideNoopAnimations } from '@angular/platform-browser/animations';
import { TranslateModule } from '@ngx-translate/core';
import { UiModule } from '../../ui.module';

interface Opt {
    name: string;
    id: number;
}

@Component({
    standalone: false,
    template: `
        <ui-listbox
            [options]="options()"
            optionLabel="name"
            [filter]="filter()"
            (valueChanged)="picked = picked.concat([$event.value])"
        />
    `
})
class HostComponent {
    public readonly options = signal<Opt[]>([
        { name: 'Ann', id: 1 },
        { name: 'Bob', id: 2 },
        { name: 'Cara', id: 3 }
    ]);
    public readonly filter = signal(false);
    public picked: Opt[] = [];
}

describe('UiListboxComponent (browser)', () => {
    function options(el: HTMLElement): HTMLElement[] {
        return Array.from(el.querySelectorAll('.ui-select-panel__option'));
    }

    beforeEach(async () => {
        await TestBed.configureTestingModule({
            declarations: [HostComponent],
            imports: [UiModule, TranslateModule.forRoot()],
            providers: [provideNoopAnimations()]
        }).compileComponents();
    });

    function setup() {
        const fixture = TestBed.createComponent(HostComponent);
        fixture.detectChanges();
        return fixture;
    }

    it('renders all options inline (always open, no trigger)', () => {
        const fixture = setup();
        expect(options(fixture.nativeElement).map(o => o.textContent?.trim())).toEqual([
            'Ann',
            'Bob',
            'Cara'
        ]);
    });

    it('emits the option on every click (action mode, no toggle-to-null)', () => {
        const fixture = setup();
        options(fixture.nativeElement)[1].click();
        fixture.detectChanges();
        options(fixture.nativeElement)[1].click();
        fixture.detectChanges();
        expect(fixture.componentInstance.picked.map(o => o.name)).toEqual(['Bob', 'Bob']);
    });

    it('filters the visible options', () => {
        const fixture = setup();
        fixture.componentInstance.filter.set(true);
        fixture.detectChanges();
        const input = fixture.nativeElement.querySelector(
            '.ui-select-panel__filter'
        ) as HTMLInputElement;
        input.value = 'ar';
        input.dispatchEvent(new Event('input'));
        fixture.detectChanges();
        expect(options(fixture.nativeElement).map(o => o.textContent?.trim())).toEqual(['Cara']);
    });
});
