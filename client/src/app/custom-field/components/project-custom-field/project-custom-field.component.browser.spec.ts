import { Component, input, model, output } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { DragDropModule } from '@angular/cdk/drag-drop';
import { TranslateModule, TranslateService } from '@ngx-translate/core';
import { of, throwError } from 'rxjs';
import { TablerIconStub, UiButtonStub } from 'src/testing/stubs';
import { ProjectCustomFieldComponent } from './project-custom-field.component';
import { CustomFieldApi } from '../../api/custom-field.api.service';
import { CustomFieldStore } from '../../store/custom-field.store';
import { CustomField } from '../../model/custom-field.model';
import { WindowService } from '../../../shared/window/window.service';
import { CustomFieldType } from 'src/app/custom-field/constants/custom-field-type.enum';

@Component({ selector: 'ui-tag', template: '', standalone: true })
class UiTagStub {
    public readonly value = input<string>();
    public readonly severity = input<string>();
}

@Component({ selector: 'app-delete-migration-dialog', template: '', standalone: true })
class DeleteMigrationDialogStub {
    public readonly visible = model(false);
    public readonly entityLabel = input<string>();
    public readonly usageItems = input<unknown[]>([]);
    public readonly options = input<unknown[]>([]);
    public readonly hasUsage = input(false);
    public readonly isLoading = input(false);
    public readonly confirmed = output<unknown>();
}

describe('ProjectCustomFieldComponent (browser)', () => {
    const field = (id: number, rank: number, archivedAt: string | null = null): CustomField => ({
        idCustomField: id,
        idProject: 10,
        key: `k${id}`,
        name: `F${id}`,
        fieldType: CustomFieldType.Text,
        isRequired: false,
        requiredSince: null,
        defaultValue: null,
        orderRank: rank,
        archivedAt,
        options: []
    });

    const option = (idOption: number, label: string, orderRank: number) => ({
        idOption,
        idCustomField: 1,
        label,
        orderRank
    });

    let customFieldApi: {
        load$: any;
        update$: any;
        delete$: any;
        usage$: any;
        archive$: any;
    };
    let customFieldStore: { load: any; customFieldsByProject$: any };

    function setup(usage = { issues: 0, optionUsage: {} }, fields = [field(1, 1), field(2, 2)]) {
        customFieldApi = {
            load$: vi.fn().mockReturnValue(of(fields)),
            update$: vi.fn().mockReturnValue(of(fields[0])),
            delete$: vi.fn().mockReturnValue(of(undefined)),
            usage$: vi.fn().mockReturnValue(of(usage)),
            archive$: vi.fn().mockReturnValue(of(fields[0]))
        };
        customFieldStore = {
            load: vi.fn(),
            customFieldsByProject$: vi.fn().mockReturnValue(of(fields))
        };

        TestBed.configureTestingModule({
            declarations: [ProjectCustomFieldComponent],
            imports: [TranslateModule.forRoot()],
            providers: [
                { provide: CustomFieldApi, useValue: customFieldApi },
                { provide: CustomFieldStore, useValue: customFieldStore }
            ]
        });
        TestBed.overrideComponent(ProjectCustomFieldComponent, {
            set: { template: '', providers: [{ provide: WindowService, useValue: {} }] }
        });
        const fixture = TestBed.createComponent(ProjectCustomFieldComponent);
        fixture.componentRef.setInput('project', { idProject: 10, name: 'P' });
        fixture.detectChanges();
        return fixture;
    }

    function render(fields: CustomField[]) {
        TestBed.configureTestingModule({
            declarations: [ProjectCustomFieldComponent],
            imports: [
                DragDropModule,
                TranslateModule.forRoot(),
                TablerIconStub,
                UiButtonStub,
                UiTagStub,
                DeleteMigrationDialogStub
            ],
            providers: [
                {
                    provide: CustomFieldApi,
                    useValue: { load$: vi.fn().mockReturnValue(of(fields)) }
                },
                {
                    provide: CustomFieldStore,
                    useValue: {
                        load: vi.fn(),
                        customFieldsByProject$: vi.fn().mockReturnValue(of(fields))
                    }
                }
            ]
        });
        const translate = TestBed.inject(TranslateService);
        translate.setTranslation('en', { CUSTOM_FIELD: { EMPTY: 'No custom fields yet' } });
        translate.use('en');
        const fixture = TestBed.createComponent(ProjectCustomFieldComponent);
        fixture.componentRef.setInput('project', { idProject: 10, name: 'P' });
        fixture.detectChanges();
        return fixture;
    }

    it('tells the user the project has no fields yet', () => {
        const fixture = render([]);

        const empty = fixture.nativeElement.querySelector('[data-testid="custom-field-empty"]');
        expect(empty?.textContent).toContain('No custom fields yet');
    });

    it('drops the empty message once a field exists', () => {
        const fixture = render([field(1, 1)]);

        expect(
            fixture.nativeElement.querySelector('[data-testid="custom-field-empty"]')
        ).toBeNull();
        expect(
            fixture.nativeElement.querySelectorAll('[data-testid="custom-field-row"]').length
        ).toBe(1);
    });

    it('shows the fields of the bound project', () => {
        const component = setup().componentInstance as any;

        expect(component.customFields().map((f: CustomField) => f.key)).toEqual(['k1', 'k2']);
    });

    it('fetches usage before opening the delete dialog', () => {
        const component = setup({ issues: 3, optionUsage: {} }).componentInstance as any;

        component.onDeleteCustomField(field(1, 1));

        expect(customFieldApi.usage$).toHaveBeenCalledWith(10, 1);
        expect(component.isDeleteDialogVisible()).toBe(true);
        expect(component.hasDeleteUsage()).toBe(true);
    });

    it('deletes values only when the field is in use', () => {
        const component = setup({ issues: 2, optionUsage: {} }).componentInstance as any;
        component.onDeleteCustomField(field(1, 1));

        component.onConfirmDelete();

        expect(customFieldApi.delete$).toHaveBeenCalledWith(10, 1, true);
    });

    it('archives and restores through the dedicated call', () => {
        const component = setup().componentInstance as any;

        component.onArchive(field(1, 1), true);
        component.onArchive(field(2, 2, '2026-01-01T00:00:00Z'), false);

        expect(customFieldApi.archive$).toHaveBeenNthCalledWith(1, expect.anything(), true);
        expect(customFieldApi.archive$).toHaveBeenNthCalledWith(2, expect.anything(), false);
    });

    it('sends the migration target when an in-use option was removed', () => {
        const component = setup().componentInstance as any;
        component.pendingField.set(field(1, 1));

        component.onConfirmOptionIntent({ migrateTo: 7 });

        expect(customFieldApi.update$).toHaveBeenCalledWith(expect.anything(), {
            withOptions: true,
            withDefaultValue: true,
            intent: { migrateOptionTo: 7 }
        });
    });

    it('drops the values when no migration target was picked', () => {
        const component = setup().componentInstance as any;
        component.pendingField.set(field(1, 1));

        component.onConfirmOptionIntent({ migrateTo: null });

        expect(customFieldApi.update$).toHaveBeenCalledWith(expect.anything(), {
            withOptions: true,
            withDefaultValue: true,
            intent: { deleteOptionValues: true }
        });
    });

    it('saves an added option before asking where to migrate to', () => {
        const stored = {
            ...field(1, 1),
            fieldType: CustomFieldType.Select,
            options: [option(5, 'Old', 1)]
        };
        const fixture = setup({ issues: 0, optionUsage: {} }, [stored]);
        const component = fixture.componentInstance as any;
        customFieldApi.update$ = vi
            .fn()
            .mockReturnValue(
                of({ ...stored, options: [option(5, 'Old', 1), option(9, 'New', 2)] })
            );

        component.resolveOptionConflict({ ...stored, options: [option(0, 'New', 1)] });

        expect(customFieldApi.update$).toHaveBeenCalledTimes(1);
        expect(customFieldApi.update$.mock.calls[0][0].options).toEqual([
            option(0, 'New', 1),
            option(5, 'Old', 1)
        ]);
        expect(customFieldApi.update$.mock.calls[0][1]).toEqual({
            withOptions: true,
            withDefaultValue: true
        });
        expect(component.optionTargets()).toEqual([{ id: 9, label: 'New' }]);
        expect(component.isOptionDialogVisible()).toBe(true);
    });

    it('migrates to the option that was only just added', () => {
        const stored = {
            ...field(1, 1),
            fieldType: CustomFieldType.Select,
            options: [option(5, 'Old', 1)]
        };
        const fixture = setup({ issues: 0, optionUsage: {} }, [stored]);
        const component = fixture.componentInstance as any;
        customFieldApi.update$ = vi
            .fn()
            .mockReturnValue(
                of({ ...stored, options: [option(5, 'Old', 1), option(9, 'New', 2)] })
            );
        component.resolveOptionConflict({ ...stored, options: [option(0, 'New', 1)] });

        component.onConfirmOptionIntent({ migrateTo: 9 });

        expect(customFieldApi.update$).toHaveBeenCalledTimes(2);
        expect(customFieldApi.update$.mock.calls[1][0].options).toEqual([option(9, 'New', 2)]);
        expect(customFieldApi.update$.mock.calls[1][1].intent).toEqual({ migrateOptionTo: 9 });
    });

    it('asks straight away when every option already has an id', () => {
        const stored = {
            ...field(1, 1),
            fieldType: CustomFieldType.Select,
            options: [option(5, 'Old', 1), option(6, 'Other', 2)]
        };
        const fixture = setup({ issues: 0, optionUsage: {} }, [stored]);
        const component = fixture.componentInstance as any;
        customFieldApi.update$ = vi.fn();

        component.resolveOptionConflict({ ...stored, options: [option(6, 'Other', 2)] });

        expect(customFieldApi.update$).not.toHaveBeenCalled();
        expect(component.optionTargets()).toEqual([{ id: 6, label: 'Other' }]);
        expect(component.isOptionDialogVisible()).toBe(true);
    });

    it('keeps the dialog shut when saving the added option fails', () => {
        const stored = {
            ...field(1, 1),
            fieldType: CustomFieldType.Select,
            options: [option(5, 'Old', 1)]
        };
        const fixture = setup({ issues: 0, optionUsage: {} }, [stored]);
        const component = fixture.componentInstance as any;
        customFieldApi.update$ = vi.fn().mockReturnValue(throwError(() => new Error('nope')));

        component.resolveOptionConflict({ ...stored, options: [option(0, 'New', 1)] });

        expect(component.isOptionDialogVisible()).toBe(false);
        expect(component.isResolvingOption()).toBe(false);
    });

    it('sends the new rank on reorder', () => {
        const component = setup().componentInstance as any;

        component.onReorder({ previousIndex: 0, currentIndex: 1 });

        expect(component.customFields().map((f: CustomField) => f.key)).toEqual(['k2', 'k1']);
        expect(customFieldApi.update$).toHaveBeenCalledWith(
            expect.objectContaining({ key: 'k1', orderRank: 2 })
        );
    });

    it('restores the previous order when the reorder call fails', () => {
        const component = setup().componentInstance as any;
        customFieldApi.update$ = vi.fn().mockReturnValue(throwError(() => new Error('nope')));

        component.onReorder({ previousIndex: 0, currentIndex: 1 });

        expect(component.customFields().map((f: CustomField) => f.key)).toEqual(['k1', 'k2']);
    });
});
