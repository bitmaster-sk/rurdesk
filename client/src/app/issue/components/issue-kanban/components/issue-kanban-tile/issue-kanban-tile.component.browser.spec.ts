import { ComponentFixture, TestBed } from '@angular/core/testing';
import { Component, input } from '@angular/core';
import { TranslateModule } from '@ngx-translate/core';
import { CommonModule } from '@angular/common';
import { RouterModule } from '@angular/router';
import { By } from '@angular/platform-browser';
import { AvatarStub, TablerIconStub } from 'src/testing/stubs';
import { KanbanTileComponent } from './issue-kanban-tile.component';
import { KanbanTile } from '../../entity/kanban-tile.entity';
import { IssueSelectionStore } from '../../../../store/issue-selection.store';
import { TileSelectionClick } from '../../entity/kanban-selection-click.entity';

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
});

describe('KanbanTileComponent selection', () => {
    let fixture: ComponentFixture<KanbanTileComponent>;
    let selection: IssueSelectionStore;
    let clicks: TileSelectionClick[];

    beforeEach(async () => {
        selection = new IssueSelectionStore();
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
            ],
            providers: [{ provide: IssueSelectionStore, useValue: selection }]
        }).compileComponents();
        fixture = TestBed.createComponent(KanbanTileComponent);
        fixture.componentRef.setInput('tile', tile({ idIssuePublic: 7 }));
        clicks = [];
        fixture.componentInstance.selectionClicked.subscribe(click => clicks.push(click));
        fixture.detectChanges();
    });

    const card = (): HTMLElement => fixture.nativeElement.querySelector('.tile-card');

    const clickCard = (init: MouseEventInit): void => {
        card()
            .querySelector('.tile-card--id')!
            .dispatchEvent(new MouseEvent('click', { bubbles: true, ...init }));
    };

    it('reports Cmd+click as a toggle and Shift+click as a range', () => {
        clickCard({ metaKey: true });
        clickCard({ shiftKey: true });

        expect(clicks.map(click => click.mode)).toEqual(['toggle', 'range']);
        expect(clicks[0].issue.idIssuePublic).toBe(7);
    });

    it('ignores a plain click', () => {
        clickCard({});
        expect(clicks).toEqual([]);
    });

    it('marks the card as selected when the task is in the selection', () => {
        selection.toggle(tile({ idIssuePublic: 7 }));
        fixture.detectChanges();

        expect(card().classList).toContain('tile-card--selected');
        expect(fixture.nativeElement.getAttribute('aria-selected')).toBe('true');
    });
});
