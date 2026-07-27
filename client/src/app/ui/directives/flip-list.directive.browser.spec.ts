import { describe, it, expect, beforeEach } from 'vitest';
import { Component, signal } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { UiModule } from '../ui.module';

@Component({
    template: `
        <div uiFlipList>
            @for (item of items(); track item) {
                <div [attr.data-flip-id]="item" style="height: 40px">{{ item }}</div>
            }
        </div>
    `,
    standalone: false
})
class HostComponent {
    public readonly items = signal(['a', 'b', 'c']);
}

describe('UiFlipListDirective', () => {
    beforeEach(() =>
        TestBed.configureTestingModule({
            imports: [UiModule],
            declarations: [HostComponent]
        })
    );

    it('animates an item that changed vertical position', () => {
        const f = TestBed.createComponent(HostComponent);
        f.detectChanges();

        f.componentInstance.items.set(['c', 'a', 'b']);
        f.detectChanges();

        const moved = f.nativeElement.querySelector('[data-flip-id="c"]') as HTMLElement;
        expect(moved.getAnimations().length).toBe(1);
    });

    it('does not animate items whose position is unchanged or brand new', () => {
        const f = TestBed.createComponent(HostComponent);
        f.detectChanges();

        f.componentInstance.items.set(['a', 'b', 'c', 'd']);
        f.detectChanges();

        const stable = f.nativeElement.querySelector('[data-flip-id="a"]') as HTMLElement;
        const fresh = f.nativeElement.querySelector('[data-flip-id="d"]') as HTMLElement;
        expect(stable.getAnimations().length).toBe(0);
        expect(fresh.getAnimations().length).toBe(0);
    });
});
