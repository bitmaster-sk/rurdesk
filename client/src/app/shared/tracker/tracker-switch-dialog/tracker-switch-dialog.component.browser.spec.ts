import { Component, signal } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { TranslateModule } from '@ngx-translate/core';
import { subSeconds } from 'date-fns';
import { SharedModule } from 'src/app/shared/shared.module';
import { Tracker } from '../model/tracker.model';

const RUNNING: Tracker = {
    idTracker: 7,
    idUser: 1,
    idIssue: 42,
    startAt: subSeconds(new Date(), 65),
    pausedAt: null,
    pausedSeconds: 0,
    duration: {},
    idProject: 3,
    idIssuePublic: 12,
    issueTitle: 'Fix the broken login',
    projectName: 'Acme'
};

@Component({
    standalone: false,
    template: `
        <app-tracker-switch-dialog
            [(visible)]="visible"
            [tracker]="tracker"
            [idIssuePublic]="18"
            issueTitle="Write the docs"
            projectName="Acme"
            (confirmed)="switched.set(true)"
        ></app-tracker-switch-dialog>
    `
})
class HostComponent {
    public visible = true;
    public tracker = RUNNING;
    public switched = signal(false);
}

describe('TrackerSwitchDialogComponent (browser)', () => {
    beforeEach(async () => {
        await TestBed.configureTestingModule({
            declarations: [HostComponent],
            imports: [SharedModule, TranslateModule.forRoot()]
        }).compileComponents();
    });

    function setup() {
        const fixture = TestBed.createComponent(HostComponent);
        fixture.detectChanges();
        return fixture;
    }

    function panel(): HTMLElement {
        return document.querySelector('[data-testid="tracker-switch-dialog"]') as HTMLElement;
    }

    it('names both tasks with their reference and project', () => {
        setup();
        const refs = Array.from(panel().querySelectorAll('.tsd-ref')).map(el => el.textContent);
        const titles = Array.from(panel().querySelectorAll('.tsd-title')).map(el =>
            el.textContent?.trim()
        );

        expect(refs).toEqual(['#12', '#18']);
        expect(titles).toEqual(['Fix the broken login', 'Write the docs']);
        expect(panel().textContent).toContain('Acme');
    });

    it('shows how much time the running timer would submit', () => {
        setup();
        expect(
            document.querySelector('[data-testid="tracker-switch-elapsed"]')?.textContent?.trim()
        ).toBe('00:01:05');
    });

    it('switches and closes on confirm', () => {
        const fixture = setup();
        (
            document.querySelector('[data-testid="tracker-switch-confirm"] button') as HTMLElement
        ).click();
        fixture.detectChanges();

        expect(fixture.componentInstance.switched()).toBe(true);
        expect(panel()).toBeNull();
    });

    it('closes without switching on cancel', () => {
        const fixture = setup();
        (
            document.querySelector('[data-testid="tracker-switch-cancel"] button') as HTMLElement
        ).click();
        fixture.detectChanges();

        expect(fixture.componentInstance.switched()).toBe(false);
        expect(panel()).toBeNull();
    });
});
