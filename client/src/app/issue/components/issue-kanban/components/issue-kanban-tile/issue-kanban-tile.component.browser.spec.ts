import { ComponentFixture, TestBed } from '@angular/core/testing';
import { Component, input } from '@angular/core';
import { TranslateModule } from '@ngx-translate/core';
import { CommonModule } from '@angular/common';
import { RouterModule } from '@angular/router';
import { By } from '@angular/platform-browser';
import { AvatarStub, TablerIconStub } from 'src/testing/stubs';
import { KanbanTileComponent } from './issue-kanban-tile.component';
import { KanbanTile } from '../../entity/kanban-tile.entity';

@Component({ selector: 'app-severity-circle', template: '', standalone: true })
class SeverityCircleStub {
    public readonly color = input<string | undefined>(undefined);
}

@Component({ selector: 'app-state-badge', template: '', standalone: true })
class StateBadgeStub {
    public readonly state = input<unknown>(undefined);
    public readonly size = input<string>('');
}

@Component({ selector: 'app-issue-type-badge', template: '', standalone: true })
class IssueTypeBadgeStub {
    public readonly issueType = input<unknown>(undefined);
    public readonly size = input<string>('');
}

function tile(over: Partial<KanbanTile>): KanbanTile {
    return {
        idIssue: 1,
        idIssuePublic: 1,
        idProject: 1,
        idState: 1,
        idSeverity: null,
        title: 't',
        description: '',
        tracked: 0,
        ...over
    } as unknown as KanbanTile;
}

describe('KanbanTileComponent points badge', () => {
    let fixture: ComponentFixture<KanbanTileComponent>;

    beforeEach(async () => {
        await TestBed.configureTestingModule({
            declarations: [KanbanTileComponent],
            imports: [
                TranslateModule.forRoot(),
                CommonModule,
                RouterModule.forRoot([]),
                AvatarStub,
                TablerIconStub,
                SeverityCircleStub,
                StateBadgeStub,
                IssueTypeBadgeStub
            ]
        }).compileComponents();
        fixture = TestBed.createComponent(KanbanTileComponent);
    });

    it('shows the points badge when the issue has points', () => {
        fixture.componentRef.setInput('tile', tile({ points: 8 }));
        fixture.detectChanges();
        const badge = fixture.debugElement.query(By.css('[data-testid="tile-points"]'));
        expect(badge).toBeTruthy();
        expect(badge.nativeElement.textContent).toContain('8');
    });

    it('hides the badge when points is null', () => {
        fixture.componentRef.setInput('tile', tile({ points: null }));
        fixture.detectChanges();
        expect(fixture.debugElement.query(By.css('[data-testid="tile-points"]'))).toBeNull();
    });

    it('shows the severity when the issue has one', () => {
        fixture.componentRef.setInput(
            'tile',
            tile({
                severity: {
                    idSeverity: 1,
                    idProject: 1,
                    title: 'High',
                    color: '#f00',
                    protected: false,
                    orderRank: 1
                }
            })
        );
        fixture.detectChanges();

        const severity = fixture.debugElement.query(By.css('.tile-card--severity'));
        expect(severity.nativeElement.textContent).toContain('High');
    });

    it('omits the severity entirely when the issue has none', () => {
        fixture.componentRef.setInput('tile', tile({ severity: undefined }));
        fixture.detectChanges();

        expect(fixture.debugElement.query(By.css('.tile-card--severity'))).toBeNull();
    });
});
