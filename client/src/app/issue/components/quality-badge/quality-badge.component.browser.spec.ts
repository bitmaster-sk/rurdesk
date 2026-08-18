import { ComponentFixture, TestBed } from '@angular/core/testing';
import { QualityBadgeComponent } from './quality-badge.component';
import { By } from '@angular/platform-browser';
import { Component } from '@angular/core';

@Component({
    template: '<app-quality-badge [score]="score"></app-quality-badge>',
    standalone: false
})
class TestHostComponent {
    public score: number | null = null;
}

describe('QualityBadgeComponent', () => {
    let fixture: ComponentFixture<TestHostComponent>;
    let host: TestHostComponent;

    beforeEach(async () => {
        await TestBed.configureTestingModule({
            declarations: [QualityBadgeComponent, TestHostComponent]
        }).compileComponents();

        fixture = TestBed.createComponent(TestHostComponent);
        host = fixture.componentInstance;
    });

    it('renders nothing when score is null', () => {
        host.score = null;
        fixture.detectChanges();

        const badge = fixture.debugElement.query(By.css('.quality-badge'));
        expect(badge).toBeNull();
    });

    it('renders red badge for score 30 (poor)', () => {
        host.score = 30;
        fixture.detectChanges();

        const badge = fixture.debugElement.query(By.css('.quality-badge--poor'));
        expect(badge).toBeTruthy();
    });

    it('renders yellow badge for score 60 (acceptable)', () => {
        host.score = 60;
        fixture.detectChanges();

        const badge = fixture.debugElement.query(By.css('.quality-badge--acceptable'));
        expect(badge).toBeTruthy();
    });

    it('renders green badge for score 80 (good)', () => {
        host.score = 80;
        fixture.detectChanges();

        const badge = fixture.debugElement.query(By.css('.quality-badge--good'));
        expect(badge).toBeTruthy();
    });

    it('renders red badge for score 40 (boundary of poor)', () => {
        host.score = 40;
        fixture.detectChanges();

        const badge = fixture.debugElement.query(By.css('.quality-badge--poor'));
        expect(badge).toBeTruthy();
    });

    it('renders yellow badge for score 41 (boundary of acceptable)', () => {
        host.score = 41;
        fixture.detectChanges();

        const badge = fixture.debugElement.query(By.css('.quality-badge--acceptable'));
        expect(badge).toBeTruthy();
    });

    it('renders green badge for score 71 (boundary of good)', () => {
        host.score = 71;
        fixture.detectChanges();

        const badge = fixture.debugElement.query(By.css('.quality-badge--good'));
        expect(badge).toBeTruthy();
    });
});
