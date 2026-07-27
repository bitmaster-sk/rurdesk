import { TestBed } from '@angular/core/testing';
import { TranslateModule } from '@ngx-translate/core';
import { UiModule } from '../../../ui/ui.module';
import { TablerIconStub } from 'src/testing/stubs';
import { MockupCardComponent } from './mockup-card.component';

describe('MockupCardComponent (browser)', () => {
    beforeEach(async () => {
        await TestBed.configureTestingModule({
            imports: [UiModule, TranslateModule.forRoot(), TablerIconStub],
            declarations: [MockupCardComponent]
        }).compileComponents();
    });

    function create(html: string, title?: string) {
        const fixture = TestBed.createComponent(MockupCardComponent);
        fixture.componentRef.setInput('html', html);
        if (title !== undefined) {
            fixture.componentRef.setInput('title', title);
        }
        fixture.detectChanges();
        return fixture;
    }

    it('renders the card label and the title when provided', () => {
        const fixture = create('<h1>Hi</h1>', 'Login screen');
        const card = fixture.nativeElement.querySelector('.mockup-card') as HTMLElement;

        expect(card).toBeTruthy();
        expect(card.textContent).toContain('MOCKUP.LABEL');
        expect(card.textContent).toContain('Login screen');
    });

    it('opens the dialog with the sandboxed iframe on card click', async () => {
        const fixture = create('<button id="go">Go</button>', 'Demo');
        const card = fixture.nativeElement.querySelector('.mockup-card') as HTMLElement;

        expect(document.querySelector('iframe.mockup-frame')).toBeNull();

        card.click();
        fixture.detectChanges();
        await fixture.whenStable();

        const iframe = document.querySelector('iframe.mockup-frame') as HTMLIFrameElement;
        expect(iframe).toBeTruthy();

        const sandbox = iframe.getAttribute('sandbox') ?? '';
        expect(sandbox).toContain('allow-scripts');
        expect(sandbox).not.toContain('allow-same-origin');

        const srcdoc = iframe.getAttribute('srcdoc') ?? '';
        expect(srcdoc).toContain('<button id="go">Go</button>');
        expect(srcdoc).toContain('Content-Security-Policy');
    });

    it('shows the approve button only when approvable and emits on click', () => {
        const fixture = create('<p>x</p>');
        expect(fixture.nativeElement.querySelector('.mockup-card__approve')).toBeFalsy();

        fixture.componentRef.setInput('approvable', true);
        fixture.detectChanges();

        let emitted = 0;
        fixture.componentInstance.useAndApprove.subscribe(() => emitted++);

        const btn = fixture.nativeElement.querySelector('.mockup-card__approve') as HTMLElement;
        expect(btn).toBeTruthy();
        btn.click();
        expect(emitted).toBe(1);
    });

    it('keeps the approve action legible on touch: a visible label plus an accessible name', () => {
        const fixture = create('<p>x</p>');
        fixture.componentRef.setInput('approvable', true);
        fixture.detectChanges();

        // Visible label — the action is not icon-only, so touch users (no hover
        // tooltip) still see what the button does.
        const label = fixture.nativeElement.querySelector(
            '.mockup-card__approve .ui-button__label'
        ) as HTMLElement;
        expect(label).toBeTruthy();
        expect(label.textContent?.trim()).toBe('AGENT.ACTIONS.USE_SHORT');

        // Accessible name carries the full, unambiguous action for screen readers.
        const nativeBtn = fixture.nativeElement.querySelector(
            '.mockup-card__approve .ui-button'
        ) as HTMLElement;
        expect(nativeBtn.getAttribute('aria-label')).toBe('AGENT.ACTIONS.USE_AND_APPROVE');
    });

    it('renders the selected badge and hides the action when selected', () => {
        const fixture = create('<p>x</p>');
        fixture.componentRef.setInput('approvable', true);
        fixture.componentRef.setInput('selected', true);
        fixture.detectChanges();

        expect(fixture.nativeElement.querySelector('.mockup-card--selected')).toBeTruthy();
        expect(fixture.nativeElement.querySelector('.mockup-card__badge')).toBeTruthy();
        expect(fixture.nativeElement.querySelector('.mockup-card__approve')).toBeFalsy();
    });

    it('greys out and hides the action when rejected', () => {
        const fixture = create('<p>x</p>');
        fixture.componentRef.setInput('approvable', true);
        fixture.componentRef.setInput('rejected', true);
        fixture.detectChanges();

        expect(fixture.nativeElement.querySelector('.mockup-card--rejected')).toBeTruthy();
        expect(fixture.nativeElement.querySelector('.mockup-card__approve')).toBeFalsy();
    });
});
