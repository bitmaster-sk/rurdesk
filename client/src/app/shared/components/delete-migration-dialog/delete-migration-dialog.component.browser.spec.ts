import { describe, beforeEach, expect, it } from 'vitest';
import { TestBed } from '@angular/core/testing';
import { FormsModule } from '@angular/forms';
import { TranslateModule } from '@ngx-translate/core';
import { TablerIconStub } from 'src/testing/stubs';
import { UiModule } from 'src/app/ui/ui.module';
import { DeleteMigrationOption, DeleteMigrationUsageItem } from './delete-migration-option.model';
import { DeleteMigrationDialogComponent } from './delete-migration-dialog.component';

const usageItems: DeleteMigrationUsageItem[] = [
    { key: 'STATE.DELETE_USAGE.MANY', params: { count: 3 } }
];

const targetOption: DeleteMigrationOption = {
    id: 2,
    label: 'In progress',
    color: 'var(--ui-color-state-in-progress)'
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
        fixture.componentRef.setInput('usageItems', usageItems);
        fixture.componentRef.setInput('options', [targetOption]);
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
        const emitted: { migrateTo: number | null }[] = [];
        fixture.componentInstance.confirmed.subscribe(v => emitted.push(v));
        fixture.componentInstance.selectedId.set(2);
        fixture.componentInstance.onConfirm();
        expect(emitted).toEqual([{ migrateTo: 2 }]);
    });

    it('emits null on confirm in unassign mode', () => {
        const fixture = create();
        const emitted: { migrateTo: number | null }[] = [];
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
        fixture.componentRef.setInput('options', []);
        fixture.componentRef.setInput('hasUsage', true);
        fixture.componentRef.setInput('visible', true);
        fixture.detectChanges();
        expect(fixture.componentInstance.mode()).toBe('unassign');
    });

    it('zero usage → plain confirm: unassign mode, confirm enabled immediately', () => {
        const fixture = TestBed.createComponent(DeleteMigrationDialogComponent);
        fixture.componentRef.setInput('entityLabel', 'State');
        fixture.componentRef.setInput('usageItems', []);
        fixture.componentRef.setInput('options', [targetOption]);
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

describe('DeleteMigrationDialogComponent — rendered choices', () => {
    beforeEach(() => {
        TestBed.configureTestingModule({
            declarations: [DeleteMigrationDialogComponent],
            imports: [FormsModule, UiModule, TranslateModule.forRoot(), TablerIconStub]
        });
    });

    function render(options: DeleteMigrationOption[]) {
        const fixture = TestBed.createComponent(DeleteMigrationDialogComponent);
        fixture.componentRef.setInput('entityLabel', 'Custom field');
        fixture.componentRef.setInput('usageItems', usageItems);
        fixture.componentRef.setInput('options', options);
        fixture.componentRef.setInput('hasUsage', true);
        fixture.componentRef.setInput('visible', true);
        fixture.detectChanges();
        return fixture;
    }

    it('offers both choices when the values can be moved somewhere', () => {
        render([targetOption]);

        expect(
            document.querySelector('[data-testid="delete-migration-mode-migrate"]')
        ).not.toBeNull();
        expect(
            document.querySelector('[data-testid="delete-migration-mode-unassign"]')
        ).not.toBeNull();
    });

    it('offers no choice at all when there is nowhere to move the values', () => {
        render([]);

        expect(document.querySelector('[data-testid="delete-migration-mode-migrate"]')).toBeNull();
        expect(document.querySelector('[data-testid="delete-migration-mode-unassign"]')).toBeNull();
    });
});
