import { describe, beforeEach, expect, it } from 'vitest';
import { TestBed } from '@angular/core/testing';
import { TranslateModule } from '@ngx-translate/core';
import { IssueState } from 'src/app/state/model/issue-state.model';
import { DeleteMigrationDialogComponent } from './delete-migration-dialog.component';

const targetState: IssueState = {
    idState: 2,
    idProject: 1,
    name: 'In progress',
    start: false,
    final: false,
    protected: false,
    orderRank: 1
};

describe('DeleteMigrationDialogComponent', () => {
    beforeEach(() => {
        TestBed.configureTestingModule({
            declarations: [DeleteMigrationDialogComponent],
            imports: [TranslateModule.forRoot()]
        }).overrideComponent(DeleteMigrationDialogComponent, {
            set: { template: '' }
        });
    });

    function create() {
        const fixture = TestBed.createComponent(DeleteMigrationDialogComponent);
        fixture.componentRef.setInput('entityLabel', 'State');
        fixture.componentRef.setInput('usageItems', ['3 tasks still use this state']);
        fixture.componentRef.setInput('stateOptions', [targetState]);
        fixture.componentRef.setInput('hasUsage', true);
        fixture.componentRef.setInput('visible', true);
        fixture.detectChanges();
        return fixture;
    }

    it('defaults to migrate mode when options exist', () => {
        const fixture = create();
        expect(fixture.componentInstance.mode()).toBe('migrate');
    });

    it('emits migrateTo id on confirm in migrate mode', () => {
        const fixture = create();
        const emitted: Array<{ migrateTo: number | null }> = [];
        fixture.componentInstance.confirmed.subscribe(v => emitted.push(v));
        fixture.componentInstance.selectedId.set(2);
        fixture.componentInstance.onConfirm();
        expect(emitted).toEqual([{ migrateTo: 2 }]);
    });

    it('emits null on confirm in unassign mode', () => {
        const fixture = create();
        const emitted: Array<{ migrateTo: number | null }> = [];
        fixture.componentInstance.confirmed.subscribe(v => emitted.push(v));
        fixture.componentInstance.mode.set('unassign');
        fixture.componentInstance.onConfirm();
        expect(emitted).toEqual([{ migrateTo: null }]);
    });

    it('blocks confirm in migrate mode until a target is picked', () => {
        const fixture = create();
        expect(fixture.componentInstance.isConfirmDisabled()).toBe(true);
        fixture.componentInstance.selectedId.set(2);
        expect(fixture.componentInstance.isConfirmDisabled()).toBe(false);
    });

    it('falls back to unassign mode when there is no migration target', () => {
        const fixture = TestBed.createComponent(DeleteMigrationDialogComponent);
        fixture.componentRef.setInput('entityLabel', 'State');
        fixture.componentRef.setInput('usageItems', []);
        fixture.componentRef.setInput('stateOptions', []);
        fixture.componentRef.setInput('hasUsage', true);
        fixture.componentRef.setInput('visible', true);
        fixture.detectChanges();
        expect(fixture.componentInstance.mode()).toBe('unassign');
    });

    it('zero usage → plain confirm: unassign mode, confirm enabled immediately', () => {
        const fixture = TestBed.createComponent(DeleteMigrationDialogComponent);
        fixture.componentRef.setInput('entityLabel', 'State');
        fixture.componentRef.setInput('usageItems', []);
        fixture.componentRef.setInput('stateOptions', [targetState]);
        fixture.componentRef.setInput('hasUsage', false);
        fixture.componentRef.setInput('visible', true);
        fixture.detectChanges();
        expect(fixture.componentInstance.mode()).toBe('unassign');
        expect(fixture.componentInstance.isConfirmDisabled()).toBe(false);
    });

    it('does not close itself on confirm (host closes after the delete settles)', () => {
        const fixture = create();
        fixture.componentInstance.selectedId.set(2);
        fixture.componentInstance.onConfirm();
        expect(fixture.componentInstance.visible()).toBe(true);
    });
});
