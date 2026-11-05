import { ComponentFixture, TestBed } from '@angular/core/testing';
import { TranslateModule, TranslateService } from '@ngx-translate/core';
import { TablerIconStub } from 'src/testing/stubs';
import { UiModule } from 'src/app/ui/ui.module';
import { IssueSelectionStore } from '../../store/issue-selection.store';
import { Issue } from '../../model/issue.model';
import { BulkEditButtonComponent } from './bulk-edit-button.component';

const issue = (idIssuePublic: number): Issue => ({
    idIssue: idIssuePublic,
    idIssuePublic,
    idProject: 1,
    idState: 1,
    idSeverity: null,
    idIssueType: null,
    title: '',
    description: '',
    tracked: 0
});

describe('BulkEditButtonComponent (TestBed)', () => {
    let fixture: ComponentFixture<BulkEditButtonComponent>;
    let selection: IssueSelectionStore;

    const button = (): HTMLButtonElement | null =>
        fixture.nativeElement.querySelector('[data-testid="bulk-edit-open"] button');

    beforeEach(async () => {
        selection = new IssueSelectionStore();
        await TestBed.configureTestingModule({
            declarations: [BulkEditButtonComponent],
            imports: [TranslateModule.forRoot(), UiModule, TablerIconStub],
            providers: [{ provide: IssueSelectionStore, useValue: selection }]
        }).compileComponents();
        const translate = TestBed.inject(TranslateService);
        translate.setTranslation('en', {
            ISSUE: { BULK_EDIT: { BUTTON: 'Bulk edit ({{count}})' } }
        });
        translate.use('en');
        fixture = TestBed.createComponent(BulkEditButtonComponent);
        fixture.detectChanges();
    });

    it('stays hidden while nothing is selected', () => {
        expect(button()).toBeNull();
    });

    it('shows one Bulk edit button with the selection count that opens the editor', () => {
        const edits: void[] = [];
        fixture.componentInstance.editRequested.subscribe(() => edits.push(undefined));
        selection.toggle(issue(1));
        selection.toggle(issue(2));
        fixture.detectChanges();

        expect(button()?.textContent).toContain('Bulk edit (2)');
        button()!.click();
        expect(edits).toHaveLength(1);
    });
});
