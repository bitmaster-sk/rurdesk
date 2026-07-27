import { Component } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { TranslateModule } from '@ngx-translate/core';
import { UiToastService } from '../../service/ui-toast.service';
import { UiToastModule } from '../../ui-toast.module';

@Component({
    standalone: false,
    template: `
        <ui-toast></ui-toast>
    `
})
class HostComponent {}

describe('UiToastComponent (browser)', () => {
    let service: UiToastService;

    function toasts(): HTMLElement[] {
        return Array.from(document.querySelectorAll('.ui-toast'));
    }

    beforeEach(async () => {
        await TestBed.configureTestingModule({
            declarations: [HostComponent],
            imports: [UiToastModule, TranslateModule.forRoot()]
        }).compileComponents();
        service = TestBed.inject(UiToastService);
    });

    function setup() {
        const fixture = TestBed.createComponent(HostComponent);
        fixture.detectChanges();
        return fixture;
    }

    // Long life so real timers never auto-dismiss mid-assert on slow CI.
    const LONG = 60000;

    it('renders a toast with its detail and severity class', () => {
        const fixture = setup();
        service.show({ severity: 'error', detail: 'Something broke', life: LONG });
        fixture.detectChanges();

        expect(toasts()).toHaveLength(1);
        expect(toasts()[0].classList.contains('ui-toast--error')).toBe(true);
        expect(toasts()[0].textContent).toContain('Something broke');
    });

    it('dismisses when the close button is clicked', () => {
        const fixture = setup();
        service.show({ severity: 'success', detail: 'Saved', life: LONG });
        fixture.detectChanges();

        const close = document.querySelector('.ui-toast__close') as HTMLButtonElement;
        close.click();
        fixture.detectChanges();

        // Assert on state (deterministic) — animate.leave may defer DOM removal.
        expect(service.toasts()).toHaveLength(0);
    });

    it('stacks multiple toasts', () => {
        const fixture = setup();
        service.show({ severity: 'info', detail: 'one', life: LONG });
        service.show({ severity: 'success', detail: 'two', life: LONG });
        fixture.detectChanges();

        expect(toasts()).toHaveLength(2);
    });
});
