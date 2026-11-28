import { Component } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { TranslateModule } from '@ngx-translate/core';
import { UiToastService } from '../../service/ui-toast.service';
import { UiToastModule } from '../../ui-toast.module';
// The toast placement lives in the global styles.
import 'src/styles.scss';

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

    it('shows at the bottom centre, clear of the page header and its actions', async () => {
        const fixture = setup();
        service.show({ severity: 'success', detail: 'Page created', life: LONG });
        fixture.detectChanges();

        await vi.waitFor(() => {
            const toast = toasts()[0].getBoundingClientRect();
            expect(toast.top).toBeGreaterThan(window.innerHeight / 2);
            expect(toast.bottom).toBeLessThanOrEqual(window.innerHeight);
            expect(Math.abs((toast.left + toast.right) / 2 - window.innerWidth / 2)).toBeLessThan(
                1
            );
        });
    });
});
