import { ComponentFixture, TestBed } from '@angular/core/testing';
import { Component, input, output } from '@angular/core';
import { IssueKanbanSwimlaneComponent } from './issue-kanban-swimlane.component';
import { DragDropModule } from '@angular/cdk/drag-drop';
import { TranslateModule } from '@ngx-translate/core';
import { CommonModule } from '@angular/common';
import { AvatarStub, TablerIconStub, UiButtonStub, UiOdometerStub } from 'src/testing/stubs';
import { SwimlaneRow } from '../../entity/swimlane-row.entity';
import { IssueState } from 'src/app/state/model/issue-state.model';
import { By } from '@angular/platform-browser';

@Component({ selector: 'app-issue-kanban-tile', template: '', standalone: true })
class KanbanTileStub {
    public readonly tile = input<unknown>(undefined);
    public readonly contextMenuRequested = output<unknown>();
}

const stateA: IssueState = {
    idState: 1,
    name: 'Todo',
    idProject: 1,
    start: true,
    final: false,
    protected: false,
    orderRank: 0
};
const stateB: IssueState = {
    idState: 2,
    name: 'Done',
    idProject: 1,
    start: false,
    final: true,
    protected: false,
    orderRank: 1
};

const userAlice = { idUser: 1, name: 'Alice', colorAvatarBg: '#aaa', email: 'a@a.com' } as any;

const rows: SwimlaneRow[] = [
    {
        user: undefined,
        cells: [
            { state: stateA, user: undefined, tiles: [] },
            { state: stateB, user: undefined, tiles: [] }
        ]
    },
    {
        user: userAlice,
        cells: [
            { state: stateA, user: userAlice, tiles: [] },
            { state: stateB, user: userAlice, tiles: [] }
        ]
    }
];

describe('IssueKanbanSwimlaneComponent', () => {
    let component: IssueKanbanSwimlaneComponent;
    let fixture: ComponentFixture<IssueKanbanSwimlaneComponent>;

    beforeEach(async () => {
        await TestBed.configureTestingModule({
            declarations: [IssueKanbanSwimlaneComponent],
            imports: [
                DragDropModule,
                TranslateModule.forRoot(),
                CommonModule,
                AvatarStub,
                TablerIconStub,
                UiButtonStub,
                UiOdometerStub,
                KanbanTileStub
            ]
        }).compileComponents();
    });

    beforeEach(() => {
        fixture = TestBed.createComponent(IssueKanbanSwimlaneComponent);
        component = fixture.componentInstance;
        fixture.componentRef.setInput('rows', rows);
        fixture.componentRef.setInput('states', [stateA, stateB]);
        fixture.detectChanges();
    });

    it('should create', () => {
        expect(component).toBeTruthy();
    });

    it('renders correct number of rows (users + unassigned)', () => {
        const rowEls = fixture.debugElement.queryAll(By.css('.swimlane-row'));
        expect(rowEls.length).toBe(2);
    });

    it('renders correct number of cells per row (one per state)', () => {
        const cellEls = fixture.debugElement.queryAll(By.css('.swimlane-cell'));
        expect(cellEls.length).toBe(4); // 2 rows × 2 states
    });

    it('sorts the unassigned row last', () => {
        const userCells = fixture.debugElement.queryAll(By.css('.swimlane-user-cell'));
        // userCells[0] is the header blank cell; data rows follow with assigned
        // users first (localeCompare) and the unassigned row pushed to the end.
        // With no translations loaded, the label renders as the raw i18n key.
        const lastRowText = userCells[userCells.length - 1].nativeElement.textContent;
        expect(lastRowText).toContain('KANBAN.UNASSIGNED');
    });

    it('emits cardDrop when a drop event fires on a cell', () => {
        const emitted: any[] = [];
        component.cardDrop.subscribe(e => emitted.push(e));
        const cells = fixture.debugElement.queryAll(By.css('.swimlane-cell'));
        cells[0].triggerEventHandler('cdkDropListDropped', { previousIndex: 0, currentIndex: 0 });
        expect(emitted.length).toBe(1);
    });
});
