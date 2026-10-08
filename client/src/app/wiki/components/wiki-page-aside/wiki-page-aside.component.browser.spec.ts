import { ComponentFixture, TestBed } from '@angular/core/testing';
import { By } from '@angular/platform-browser';
import { provideRouter } from '@angular/router';
import { RouterModule } from '@angular/router';
import { TranslateModule } from '@ngx-translate/core';
import { Subject, of } from 'rxjs';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { ToastNotificationService } from 'src/app/core/toast-notification.service';
import { TablerIconStub, UiButtonStub } from 'src/testing/stubs';
import { WikiApi } from '../../api/wiki.api.service';
import { WikiSpaceKind } from '../../constants/wiki-space-kind.enum';
import { WikiBacklinkList, WikiPageIssue, WikiPageIssueList } from '../../model/wiki-page.model';
import { WikiTreeStore } from '../../store/wiki-tree.store';
import { WikiPageAsideComponent } from './wiki-page-aside.component';

function issue(idIssue: number, overrides: Partial<WikiPageIssue> = {}): WikiPageIssue {
    return {
        idIssue,
        idIssuePublic: idIssue,
        idProject: 7,
        title: `Task ${idIssue}`,
        stateName: 'Open',
        isClosed: false,
        ...overrides
    };
}

function issues(from: number, count: number): WikiPageIssue[] {
    return Array.from({ length: count }, (_, index) => issue(from + index));
}

describe('WikiPageAsideComponent', () => {
    let fixture: ComponentFixture<WikiPageAsideComponent>;
    let moreIssues: Subject<WikiPageIssueList>;
    let loadPageIssues: ReturnType<typeof vi.fn>;
    let loadBacklinks: ReturnType<typeof vi.fn>;

    beforeEach(async () => {
        moreIssues = new Subject<WikiPageIssueList>();
        loadPageIssues = vi.fn().mockReturnValue(moreIssues);
        loadBacklinks = vi.fn().mockReturnValue(
            of({
                items: [{ idPage: 77, idSpace: 2, slug: 'late', title: 'Late page' }],
                total: 21
            })
        );
        await TestBed.configureTestingModule({
            declarations: [WikiPageAsideComponent],
            imports: [TranslateModule.forRoot(), RouterModule, TablerIconStub, UiButtonStub],
            providers: [
                provideRouter([]),
                {
                    provide: WikiApi,
                    useValue: { loadPageIssues$: loadPageIssues, loadBacklinks$: loadBacklinks }
                },
                { provide: WikiTreeStore, useValue: { kindOfSpace: () => WikiSpaceKind.Project } },
                { provide: ToastNotificationService, useValue: { showError: vi.fn() } }
            ]
        }).compileComponents();
        fixture = TestBed.createComponent(WikiPageAsideComponent);
    });

    function render(issueList: WikiPageIssueList, backlinks: WikiBacklinkList): HTMLElement {
        fixture.componentRef.setInput('idProject', 3);
        fixture.componentRef.setInput('idPage', 42);
        fixture.componentRef.setInput('issues', issueList);
        fixture.componentRef.setInput('backlinks', backlinks);
        fixture.detectChanges();
        return fixture.nativeElement as HTMLElement;
    }

    function byTestId(el: HTMLElement, testId: string): HTMLElement[] {
        return Array.from(el.querySelectorAll<HTMLElement>(`[data-testid="${testId}"]`));
    }

    function moreButton(): UiButtonStub {
        return fixture.debugElement.query(By.css('[data-testid="wiki-page-issues-more"]'))
            .componentInstance as UiButtonStub;
    }

    it('links each task into its own project and shows closed ones with their state', () => {
        const el = render(
            {
                items: [
                    issue(1, { idProject: 9, idIssuePublic: 4 }),
                    issue(2, { isClosed: true, stateName: 'Done' })
                ],
                total: 2
            },
            { items: [], total: 0 }
        );

        const rows = byTestId(el, 'wiki-page-issue');
        expect(rows.map(row => row.getAttribute('href'))).toEqual([
            '/project/9/issue/4',
            '/project/7/issue/2'
        ]);
        expect(rows[1].textContent).toContain('Done');
        expect(rows[1].classList).toContain('wiki-aside__row--closed');
        expect(byTestId(el, 'wiki-page-issues-count')).toEqual([]);
    });

    it('loads the next tasks with a spinner and stops offering more at the end', () => {
        const el = render({ items: issues(1, 20), total: 25 }, { items: [], total: 0 });
        expect(byTestId(el, 'wiki-page-issues-count')[0].textContent?.trim()).toBe('20 / 25');

        byTestId(el, 'wiki-page-issues-more')[0].click();
        fixture.detectChanges();
        expect(loadPageIssues).toHaveBeenCalledWith(42, 20);
        expect(moreButton().loading()).toBe(true);

        byTestId(el, 'wiki-page-issues-more')[0].click();
        expect(loadPageIssues).toHaveBeenCalledTimes(1);

        moreIssues.next({ items: issues(20, 6), total: 25 });
        moreIssues.complete();
        fixture.detectChanges();

        expect(byTestId(el, 'wiki-page-issue')).toHaveLength(25);
        expect(byTestId(el, 'wiki-page-issues-count')[0].textContent?.trim()).toBe('25 / 25');
        expect(byTestId(el, 'wiki-page-issues-more')).toEqual([]);
    });

    it('drops a late page when the reader has already moved to another wiki page', () => {
        const el = render({ items: issues(1, 20), total: 25 }, { items: [], total: 0 });
        byTestId(el, 'wiki-page-issues-more')[0].click();

        fixture.componentRef.setInput('idPage', 43);
        fixture.componentRef.setInput('issues', { items: issues(100, 2), total: 2 });
        fixture.detectChanges();
        moreIssues.next({ items: issues(20, 5), total: 25 });
        fixture.detectChanges();

        expect(byTestId(el, 'wiki-page-issue')).toHaveLength(2);
    });

    it('loads more pages that link here in the project the reader is in', () => {
        const backlinks = Array.from({ length: 20 }, (_, index) => ({
            idPage: index + 1,
            idSpace: 2,
            slug: `page-${index + 1}`,
            title: `Page ${index + 1}`
        }));
        const el = render({ items: [], total: 0 }, { items: backlinks, total: 21 });

        byTestId(el, 'wiki-page-backlinks-more')[0].click();
        fixture.detectChanges();

        expect(loadBacklinks).toHaveBeenCalledWith(3, 42, 20);
        const rows = byTestId(el, 'wiki-page-backlink');
        expect(rows).toHaveLength(21);
        expect(rows[20].getAttribute('href')).toBe('/project/3/wiki/project/late');
        expect(byTestId(el, 'wiki-page-backlinks-more')).toEqual([]);
    });
});
