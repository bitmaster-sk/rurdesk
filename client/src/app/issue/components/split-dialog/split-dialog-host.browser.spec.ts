import { describe, it, expect, beforeEach } from 'vitest';
import { ComponentFixture, TestBed } from '@angular/core/testing';
import { By } from '@angular/platform-browser';

import { SplitDialogHostComponent } from './split-dialog-host.component';
import { Fixtures } from 'src/testing/fixtures';
import { SplitDialogStub } from 'src/testing/stubs';

describe('SplitDialogHostComponent (browser)', () => {
    let fixture: ComponentFixture<SplitDialogHostComponent>;
    let host: SplitDialogHostComponent;

    const issue = Fixtures.issue({ idIssue: 3, idIssuePublic: 9, idProject: 4 });
    const otherIssue = Fixtures.issue({ idIssue: 7, idIssuePublic: 11, idProject: 4 });

    function dialogStub(): SplitDialogStub | null {
        const el = fixture.debugElement.query(By.directive(SplitDialogStub));
        return el ? (el.componentInstance as SplitDialogStub) : null;
    }

    beforeEach(async () => {
        await TestBed.configureTestingModule({
            declarations: [SplitDialogHostComponent],
            imports: [SplitDialogStub]
        }).compileComponents();

        fixture = TestBed.createComponent(SplitDialogHostComponent);
        host = fixture.componentInstance;
        fixture.detectChanges();
    });

    it('renders no dialog until one is opened', () => {
        expect(dialogStub()).toBeNull();
    });

    it('opens the dialog for the requested issue with its project id', () => {
        host.open(issue);
        fixture.detectChanges();

        const stub = dialogStub();
        expect(stub).not.toBeNull();
        expect(stub!.issue()).toBe(issue);
        expect(stub!.idProject()).toBe(4);
    });

    it('replaces the open issue when a new split is requested', () => {
        host.open(issue);
        host.open(otherIssue);
        fixture.detectChanges();

        const stub = dialogStub();
        expect(stub!.issue()).toBe(otherIssue);
        expect(stub!.idProject()).toBe(4);
    });

    it('closes the dialog after accepted', () => {
        host.open(issue);
        fixture.detectChanges();
        dialogStub()!.accepted.emit([]);
        fixture.detectChanges();

        expect(dialogStub()).toBeNull();
    });

    it('closes the dialog after cancelled', () => {
        host.open(issue);
        fixture.detectChanges();
        dialogStub()!.cancelled.emit();
        fixture.detectChanges();

        expect(dialogStub()).toBeNull();
    });

    it('opens again after closing', () => {
        host.open(issue);
        fixture.detectChanges();
        dialogStub()!.cancelled.emit();
        fixture.detectChanges();
        host.open(otherIssue);
        fixture.detectChanges();

        expect(dialogStub()!.issue()).toBe(otherIssue);
    });
});
