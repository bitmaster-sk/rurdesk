import { ComponentFixture, TestBed } from '@angular/core/testing';
import { provideNoopAnimations } from '@angular/platform-browser/animations';
import { BehaviorSubject, map } from 'rxjs';
import { Tag } from 'src/app/tag/model/tag.model';
import { TagStore } from 'src/app/tag/store/tag.store';
import { UiModule } from 'src/app/ui/ui.module';
import { TagChipComponent } from '../tag-chip/tag-chip.component';
import { IssueTagsComponent } from './issue-tags.component';

const tag = (idTag: number, name: string): Tag => ({
    idTag,
    idProject: 1,
    name,
    color: '#16a34a',
    orderRank: idTag,
    issueCount: 0
});

const TAGS = [tag(1, 'gateway'), tag(2, 'next'), tag(3, 'wiki'), tag(4, 'design'), tag(5, 'ops')];

describe('IssueTagsComponent (browser)', () => {
    let fixture: ComponentFixture<IssueTagsComponent>;

    beforeEach(async () => {
        const tags$ = new BehaviorSubject(TAGS);
        await TestBed.configureTestingModule({
            imports: [UiModule],
            declarations: [IssueTagsComponent, TagChipComponent],
            providers: [
                provideNoopAnimations(),
                {
                    provide: TagStore,
                    useValue: {
                        tagsMap$: tags$.pipe(map(tags => new Map(tags.map(t => [t.idTag, t]))))
                    }
                }
            ]
        }).compileComponents();
        fixture = TestBed.createComponent(IssueTagsComponent);
        fixture.componentRef.setInput('idsTag', [1, 2, 3, 4, 5]);
        fixture.detectChanges();
    });

    afterEach(() => fixture.destroy());

    function chipNames(root: ParentNode): string[] {
        return Array.from(root.querySelectorAll('.tag-chip__name')).map(el =>
            el.textContent.trim()
        );
    }

    function more(): HTMLElement {
        return fixture.nativeElement.querySelector('[data-testid="issue-tags-more"]');
    }

    function hiddenList(): HTMLElement | null {
        return document.querySelector('[data-testid="issue-tags-hidden"]');
    }

    it('shows the first three tags and a count for the rest', () => {
        expect(chipNames(fixture.nativeElement)).toEqual(['gateway', 'next', 'wiki', '+2']);
    });

    it('lists the hidden tags as chips while the count is hovered', () => {
        more().dispatchEvent(new MouseEvent('mouseenter'));
        fixture.detectChanges();

        expect(chipNames(hiddenList()!)).toEqual(['design', 'ops']);

        more().dispatchEvent(new MouseEvent('mouseleave'));
        fixture.detectChanges();

        expect(hiddenList()).toBeNull();
    });

    it('lists the hidden tags while the count has keyboard focus', () => {
        more().focus();
        fixture.detectChanges();

        expect(chipNames(hiddenList()!)).toEqual(['design', 'ops']);
    });

    it('shows no count when every tag fits', () => {
        fixture.componentRef.setInput('idsTag', [1, 2]);
        fixture.detectChanges();

        expect(more()).toBeNull();
    });
});
