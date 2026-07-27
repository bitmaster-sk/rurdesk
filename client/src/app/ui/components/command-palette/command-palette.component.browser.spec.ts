import { describe, it, expect, beforeEach } from 'vitest';
import { TestBed } from '@angular/core/testing';
import { TranslateModule } from '@ngx-translate/core';
import { UiModule } from '../../ui.module';
import { UiCommandPaletteComponent } from './command-palette.component';
import { CommandGroup, RankedCommand } from '../../../core/command/command.model';

const item = (id: string, title: string): RankedCommand => ({
    id,
    title,
    group: 'G',
    icon: 'command',
    modes: ['all'],
    run: () => {},
    score: 0,
    highlight: [{ text: title, hit: false }]
});
const groups: CommandGroup[] = [{ heading: 'G', items: [item('a', 'Alpha'), item('b', 'Bravo')] }];

describe('UiCommandPaletteComponent', () => {
    beforeEach(() =>
        TestBed.configureTestingModule({ imports: [UiModule, TranslateModule.forRoot()] })
    );

    const mount = (g: CommandGroup[] = groups) => {
        const f = TestBed.createComponent(UiCommandPaletteComponent);
        f.componentRef.setInput('groups', g);
        f.detectChanges();
        return f;
    };
    const press = (f: any, key: string, init: KeyboardEventInit = {}) => {
        f.nativeElement
            .querySelector('input')
            .dispatchEvent(new KeyboardEvent('keydown', { key, bubbles: true, ...init }));
        f.detectChanges();
    };

    it('renders every item', () =>
        expect(mount().nativeElement.querySelectorAll('[data-item]').length).toBe(2));

    it('selects first by default and moves with ArrowDown', () => {
        const f = mount();
        press(f, 'ArrowDown');
        expect(f.nativeElement.querySelector('[data-item].sel').getAttribute('data-item')).toBe(
            'b'
        );
    });

    it('emits execute for the selected item on Enter', () => {
        const f = mount();
        let got: RankedCommand | null = null;
        f.componentInstance.execute.subscribe((c: RankedCommand) => (got = c));
        press(f, 'Enter');
        expect(got!.id).toBe('a');
    });

    it('emits executePersist on Cmd+Enter', () => {
        const f = mount();
        let got: RankedCommand | null = null;
        f.componentInstance.executePersist.subscribe((c: RankedCommand) => (got = c));
        press(f, 'Enter', { metaKey: true });
        expect(got!.id).toBe('a');
    });

    it('emits complete on Tab', () => {
        const f = mount();
        let got: RankedCommand | null = null;
        f.componentInstance.complete.subscribe((c: RankedCommand) => (got = c));
        press(f, 'Tab');
        expect(got!.id).toBe('a');
    });

    it('Escape clears a non-empty query, then closes when empty', () => {
        const f = mount();
        let cleared: string | null = null;
        let closed = false;
        f.componentInstance.queryChange.subscribe((v: string) => (cleared = v));
        f.componentInstance.closed.subscribe(() => (closed = true));
        f.componentRef.setInput('query', '> x');
        f.detectChanges();
        press(f, 'Escape');
        expect(cleared).toBe(''); // sentinel null → fails if queryChange never emits
        f.componentRef.setInput('query', '');
        f.detectChanges();
        press(f, 'Escape');
        expect(closed).toBe(true);
    });
});
