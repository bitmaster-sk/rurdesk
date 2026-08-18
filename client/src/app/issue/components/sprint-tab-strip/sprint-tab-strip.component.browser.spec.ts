import { ComponentFixture, TestBed } from '@angular/core/testing';
import { CdkDropList, DragDropModule } from '@angular/cdk/drag-drop';
import { TablerIconStub, UiButtonStub } from 'src/testing/stubs';
import { TranslateModule } from '@ngx-translate/core';
import { CommonModule } from '@angular/common';
import { By } from '@angular/platform-browser';
import { SprintTabStripComponent, SprintTab } from './sprint-tab-strip.component';

// The add-sprint control is a <ui-button>, not a tab — the backlog tab shares its
// .sprint-tabs__pinned class, so scope by element to tell the two apart.
const ADD_BUTTON = 'ui-button.sprint-tabs__pinned';

const tabs: SprintTab[] = [
    {
        idSprint: null,
        label: 'Backlog',
        isCurrent: false,
        isClosed: false,
        listId: 'sprint-tab-backlog'
    },
    { idSprint: 12, label: 'Sprint 12', isCurrent: true, isClosed: false, listId: 'sprint-tab-12' },
    {
        idSprint: 13,
        label: 'Sprint 13',
        isCurrent: false,
        isClosed: false,
        listId: 'sprint-tab-13'
    },
    { idSprint: 9, label: 'Sprint 9', isCurrent: false, isClosed: true, listId: 'sprint-tab-9' }
];

describe('SprintTabStripComponent', () => {
    let component: SprintTabStripComponent;
    let fixture: ComponentFixture<SprintTabStripComponent>;

    beforeEach(async () => {
        await TestBed.configureTestingModule({
            declarations: [SprintTabStripComponent],
            imports: [
                DragDropModule,
                TranslateModule.forRoot(),
                CommonModule,
                TablerIconStub,
                UiButtonStub
            ]
        }).compileComponents();
        fixture = TestBed.createComponent(SprintTabStripComponent);
        component = fixture.componentInstance;
        fixture.componentRef.setInput('tabs', tabs);
        fixture.componentRef.setInput('selectedIdSprint', 12);
        fixture.detectChanges();
    });

    it('renders a tab per sprint plus Backlog and an add button', () => {
        const tabEls = fixture.debugElement.queryAll(By.css('.sprint-tab'));
        expect(tabEls.length).toBe(4);
        const addBtn = fixture.debugElement.query(By.css(ADD_BUTTON));
        expect(addBtn).toBeTruthy();
    });

    it('marks the current sprint and the selected tab', () => {
        expect(fixture.debugElement.query(By.css('.sprint-tab--dot'))).toBeTruthy();
        const active = fixture.debugElement.query(By.css('.sprint-tab--active'));
        expect(active.nativeElement.textContent).toContain('Sprint 12');
    });

    it('emits selected with the sprint id on tab click', () => {
        let picked: number | null | undefined;
        component.selected.subscribe(v => (picked = v));
        const tabEls = fixture.debugElement.queryAll(By.css('.sprint-tab'));
        tabEls[2].nativeElement.click(); // Sprint 13
        expect(picked).toBe(13);
    });

    it('emits createRequested when the add button is clicked', () => {
        let created = 0;
        component.createRequested.subscribe(() => created++);
        fixture.debugElement.query(By.css(ADD_BUTTON)).nativeElement.click();
        expect(created).toBe(1);
    });

    it('marks a closed sprint tab and hides its edit affordance', () => {
        const closedTab = fixture.debugElement.query(By.css('.sprint-tab--closed'));
        expect(closedTab).toBeTruthy();
        expect(closedTab.nativeElement.textContent).toContain('Sprint 9');
        expect(closedTab.query(By.css('.sprint-tab__edit'))).toBeNull();
        const openTab = fixture.debugElement
            .queryAll(By.css('.sprint-tab:not(.sprint-tabs__pinned)'))
            .find(el => el.nativeElement.textContent.includes('Sprint 12'));
        expect(openTab!.query(By.css('.sprint-tab__edit'))).toBeTruthy();
    });

    it('disables the drop list on a closed sprint tab', () => {
        const closedTab = fixture.debugElement.query(By.css('.sprint-tab--closed'));
        expect(closedTab.injector.get(CdkDropList).disabled).toBe(true);
        const openTab = fixture.debugElement
            .queryAll(By.css('.sprint-tab'))
            .find(el => el.nativeElement.textContent.includes('Sprint 12'));
        expect(openTab!.injector.get(CdkDropList).disabled).toBe(false);
    });

    it('still emits selected when a closed sprint tab is clicked', () => {
        let picked: number | null | undefined;
        component.selected.subscribe(v => (picked = v));
        fixture.debugElement.query(By.css('.sprint-tab--closed')).nativeElement.click();
        expect(picked).toBe(9);
    });

    it('emits taskDropped with the tab sprint id on a drop', () => {
        const emitted: { idSprint: number | null }[] = [];
        component.taskDropped.subscribe(e => emitted.push(e));
        const tabEls = fixture.debugElement.queryAll(By.css('.sprint-tab'));
        tabEls[1].triggerEventHandler('cdkDropListDropped', { previousIndex: 0, currentIndex: 0 });
        expect(emitted.length).toBe(1);
        expect(emitted[0].idSprint).toBe(12);
    });
});

describe('SprintTabStripComponent — revealing the current cycle', () => {
    let fixture: ComponentFixture<SprintTabStripComponent>;

    function manyTabs(currentIndex: number): SprintTab[] {
        return Array.from({ length: 30 }, (_, i) => ({
            idSprint: i + 1,
            label: `Sprint ${i + 1}`,
            isCurrent: i === currentIndex,
            isClosed: false,
            listId: `sprint-tab-${i + 1}`
        }));
    }

    function scroller(): HTMLElement {
        return fixture.debugElement.query(By.css('.sprint-tabs__scroll')).nativeElement;
    }

    async function render(tabs: SprintTab[]): Promise<void> {
        await TestBed.configureTestingModule({
            declarations: [SprintTabStripComponent],
            imports: [
                DragDropModule,
                TranslateModule.forRoot(),
                CommonModule,
                TablerIconStub,
                UiButtonStub
            ]
        }).compileComponents();
        fixture = TestBed.createComponent(SprintTabStripComponent);
        fixture.nativeElement.style.width = '400px';
        fixture.componentRef.setInput('tabs', tabs);
        fixture.componentRef.setInput('selectedIdSprint', null);
        fixture.detectChanges();
        await new Promise(resolve => setTimeout(resolve));
    }

    it('scrolls a far-right current cycle into view on first render', async () => {
        await render(manyTabs(27));
        expect(scroller().scrollLeft).toBeGreaterThan(0);

        const tab = fixture.debugElement.query(By.css('.sprint-tab--current')).nativeElement;
        const view = scroller().getBoundingClientRect();
        const rect = tab.getBoundingClientRect();
        expect(rect.left).toBeGreaterThanOrEqual(view.left - 1);
        expect(rect.right).toBeLessThanOrEqual(view.right + 1);
    });

    it('leaves the scroll alone when the current cycle is already visible', async () => {
        await render(manyTabs(0));
        expect(scroller().scrollLeft).toBe(0);
    });

    it('does not fight a later manual scroll', async () => {
        await render(manyTabs(27));
        const el = scroller();
        el.scrollLeft = 0;

        fixture.componentRef.setInput('tabs', manyTabs(27).slice(0, 29));
        fixture.detectChanges();
        await new Promise(resolve => setTimeout(resolve));

        expect(el.scrollLeft).toBe(0);
    });
});
