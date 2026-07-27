import { Component } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { UiModule } from '../../ui.module';
import { UiMessageSeverity } from './message.component';

@Component({
    standalone: false,
    template: `
        <ui-message [severity]="severity"><span class="body">Heads up</span></ui-message>
    `
})
class HostComponent {
    public severity: UiMessageSeverity = 'info';
}

describe('UiMessageComponent (browser)', () => {
    function el(fixture: { nativeElement: HTMLElement }): HTMLElement {
        return fixture.nativeElement.querySelector('ui-message') as HTMLElement;
    }

    beforeEach(async () => {
        await TestBed.configureTestingModule({
            declarations: [HostComponent],
            imports: [UiModule]
        }).compileComponents();
    });

    function setup(severity?: UiMessageSeverity) {
        const fixture = TestBed.createComponent(HostComponent);
        if (severity) {
            fixture.componentInstance.severity = severity;
        }
        fixture.detectChanges();
        return fixture;
    }

    it('projects its content', () => {
        const fixture = setup();
        expect(el(fixture).querySelector('.body')?.textContent).toBe('Heads up');
    });

    it('renders a decorative severity icon', () => {
        const fixture = setup();
        const icon = el(fixture).querySelector('.ui-message__icon');
        expect(icon).not.toBeNull();
        expect(icon?.getAttribute('aria-hidden')).toBe('true');
    });

    it('defaults to info: status role + info class', () => {
        const fixture = setup();
        expect(el(fixture).getAttribute('role')).toBe('status');
        expect(el(fixture).classList).toContain('ui-message--info');
    });

    it('warn stays polite (status) with the warn class', () => {
        const fixture = setup('warn');
        expect(el(fixture).getAttribute('role')).toBe('status');
        expect(el(fixture).classList).toContain('ui-message--warn');
    });

    it('danger is assertive (alert)', () => {
        const fixture = setup('danger');
        expect(el(fixture).getAttribute('role')).toBe('alert');
        expect(el(fixture).classList).toContain('ui-message--danger');
    });

    it('success renders the success class', () => {
        const fixture = setup('success');
        expect(el(fixture).classList).toContain('ui-message--success');
        expect(el(fixture).getAttribute('role')).toBe('status');
    });
});
