import { TestBed } from '@angular/core/testing';
import { AvatarComponent } from './avatar.component';

describe('AvatarComponent (browser)', () => {
    beforeEach(async () => {
        await TestBed.configureTestingModule({
            declarations: [AvatarComponent]
        }).compileComponents();
    });

    it('re-renders initials when the name input changes', () => {
        const fixture = TestBed.createComponent(AvatarComponent);
        fixture.componentRef.setInput('name', 'Ada Lovelace');
        fixture.detectChanges();

        expect(fixture.nativeElement.textContent.trim()).toBe('AL');

        fixture.componentRef.setInput('name', 'Grace Hopper');
        fixture.detectChanges();

        expect(fixture.nativeElement.textContent.trim()).toBe('GH');
    });

    it('re-renders text color when the bgColor input changes', () => {
        const fixture = TestBed.createComponent(AvatarComponent);
        fixture.componentRef.setInput('bgColor', '#000000');
        fixture.detectChanges();

        expect(fixture.nativeElement.querySelector('.initials-avatar')?.style.color).toBe(
            'rgb(255, 255, 255)'
        );
    });

    it('explicit initials win over the name-based computation', () => {
        const fixture = TestBed.createComponent(AvatarComponent);
        fixture.componentRef.setInput('name', 'Ada Lovelace');
        fixture.componentRef.setInput('initials', 'xy');
        fixture.detectChanges();

        expect(fixture.nativeElement.textContent.trim()).toBe('XY');
    });

    it('falls back to the name-based initials when initials are empty', () => {
        const fixture = TestBed.createComponent(AvatarComponent);
        fixture.componentRef.setInput('name', 'Ada Lovelace');
        fixture.componentRef.setInput('initials', '');
        fixture.detectChanges();

        expect(fixture.nativeElement.textContent.trim()).toBe('AL');
    });
});
