import { describe, it, expect, beforeEach } from 'vitest';
import { By } from '@angular/platform-browser';
import { createTableFixture, makeIssue, QuickActionsStub } from './table-testbed.helper';
import { SplitDialogStub } from 'src/testing/stubs';
import { Issue } from '../../model/issue.model';

describe('IssueTableComponent split dialog (browser)', () => {
    let fixture: any;
    const issue = makeIssue({ idIssue: 12, idIssuePublic: 3, idProject: 10 });

    function quickActions(): QuickActionsStub {
        return fixture.debugElement.query(By.directive(QuickActionsStub))
            .componentInstance as QuickActionsStub;
    }

    function dialog(): SplitDialogStub | null {
        const el = fixture.debugElement.query(By.directive(SplitDialogStub));
        return el ? (el.componentInstance as SplitDialogStub) : null;
    }

    beforeEach(async () => {
        const result = await createTableFixture();
        fixture = result.fixture;
    });

    it('shows no dialog until quick actions request a split', () => {
        expect(dialog()).toBeNull();
    });

    it('opens the dialog with the requested issue when quick actions request a split', () => {
        quickActions().splitRequested.emit(issue);
        fixture.detectChanges();

        const stub = dialog();
        expect(stub).not.toBeNull();
        expect(stub!.issue()).toBe(issue);
        expect(stub!.idProject()).toBe(10);
    });

    it('closes the dialog when the split is cancelled', () => {
        quickActions().splitRequested.emit(issue);
        fixture.detectChanges();
        dialog()!.cancelled.emit();
        fixture.detectChanges();

        expect(dialog()).toBeNull();
    });

    it('closes the dialog when the split is accepted', () => {
        quickActions().splitRequested.emit(issue);
        fixture.detectChanges();
        dialog()!.accepted.emit([issue] as Issue[]);
        fixture.detectChanges();

        expect(dialog()).toBeNull();
    });
});
