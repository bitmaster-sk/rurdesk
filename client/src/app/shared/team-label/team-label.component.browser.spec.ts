import { TestBed } from '@angular/core/testing';
import { TeamLabelComponent } from './team-label.component';

describe('TeamLabelComponent (browser)', () => {
    function render(team: { name: string; color: string }): HTMLElement {
        TestBed.configureTestingModule({ declarations: [TeamLabelComponent] });
        const fixture = TestBed.createComponent(TeamLabelComponent);
        fixture.componentRef.setInput('team', team);
        fixture.detectChanges();
        return fixture.nativeElement as HTMLElement;
    }

    function dot(element: HTMLElement): HTMLElement {
        return element.querySelector('[data-testid="team-label-dot"]') as HTMLElement;
    }

    it('shows the team name with a dot in the team color', () => {
        const element = render({ name: 'Developers', color: '#ff0000' });

        expect(element.textContent?.trim()).toBe('Developers');
        expect(dot(element).style.backgroundColor).toBe('rgb(255, 0, 0)');
    });

    it('falls back to the neutral color when the team has none', () => {
        const element = render({ name: 'Legacy', color: '' });

        expect(dot(element).style.backgroundColor).toBe('var(--ui-color-unknown)');
    });
});
