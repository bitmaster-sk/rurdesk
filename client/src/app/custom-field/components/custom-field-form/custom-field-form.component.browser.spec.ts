import { TestBed } from '@angular/core/testing';
import { FormsModule, ReactiveFormsModule } from '@angular/forms';
import { TranslateModule } from '@ngx-translate/core';
import { TablerIconStub } from 'src/testing/stubs';
import { UiModule } from 'src/app/ui/ui.module';
import { CustomFieldFormComponent, CustomFieldFormValue } from './custom-field-form.component';
import { CustomFieldValueInputComponent } from '../custom-field-value-input/custom-field-value-input.component';
import { CustomField } from '../../model/custom-field.model';
import { CustomFieldType } from 'src/app/custom-field/constants/custom-field-type.enum';

describe('CustomFieldFormComponent (browser)', () => {
    const existing = (idCustomField: number, key: string, name: string): CustomField => ({
        idCustomField,
        idProject: 10,
        key,
        name,
        fieldType: CustomFieldType.Text,
        isRequired: false,
        requiredSince: null,
        defaultValue: null,
        orderRank: idCustomField,
        archivedAt: null,
        options: []
    });

    function setup(
        customField: Partial<CustomField>,
        issuesMissing: number | null = null,
        projectFields: CustomField[] = []
    ) {
        TestBed.configureTestingModule({
            declarations: [CustomFieldFormComponent, CustomFieldValueInputComponent],
            imports: [
                FormsModule,
                ReactiveFormsModule,
                UiModule,
                TranslateModule.forRoot(),
                TablerIconStub
            ]
        });
        const fixture = TestBed.createComponent(CustomFieldFormComponent);
        fixture.componentRef.setInput('customField', customField);
        fixture.componentRef.setInput('issuesMissing', issuesMissing);
        fixture.componentRef.setInput('projectFields', projectFields);
        fixture.detectChanges();
        return fixture;
    }

    function keyInput(fixture: ReturnType<typeof setup>): HTMLInputElement {
        return fixture.nativeElement.querySelector('#custom-field-key') as HTMLInputElement;
    }

    function keyEditButton(fixture: ReturnType<typeof setup>): HTMLElement | null {
        return fixture.nativeElement.querySelector('[data-testid="custom-field-key-edit"]');
    }

    function type(fixture: ReturnType<typeof setup>, selector: string, value: string): void {
        const input = fixture.nativeElement.querySelector(selector) as HTMLInputElement;
        input.value = value;
        input.dispatchEvent(new Event('input'));
        fixture.detectChanges();
    }

    function backfillBlock(fixture: ReturnType<typeof setup>): HTMLElement | null {
        return fixture.nativeElement.querySelector('[data-testid="custom-field-backfill"]');
    }

    it('fills the key field from the name and keeps it locked', () => {
        const fixture = setup({ idProject: 10 });
        const component = fixture.componentInstance as any;

        type(fixture, '#custom-field-name', 'Termín dodania');

        expect(keyInput(fixture).value).toBe('termin_dodania');
        expect(keyInput(fixture).disabled).toBe(true);
        expect(keyEditButton(fixture)).not.toBeNull();
        component.onSave();
        expect(component.form.getRawValue().key).toBe('termin_dodania');
    });

    it('unlocks the key field on request', () => {
        const fixture = setup({ idProject: 10 });

        type(fixture, '#custom-field-name', 'Owner');
        (fixture.componentInstance as any).onRevealKey();
        fixture.detectChanges();

        expect(keyInput(fixture).disabled).toBe(false);
        expect(keyEditButton(fixture)).toBeNull();
    });

    it('stops following the name once the key was written by hand', () => {
        const fixture = setup({ idProject: 10 });
        const component = fixture.componentInstance as any;

        type(fixture, '#custom-field-name', 'Owner');
        component.onRevealKey();
        fixture.detectChanges();
        type(fixture, '#custom-field-key', 'responsible');
        type(fixture, '#custom-field-name', 'Owner of the task');

        expect(component.form.getRawValue().key).toBe('responsible');
    });

    it('names the field holding the key when two names slug the same', () => {
        const fixture = setup({ idProject: 10 }, null, [
            existing(3, 'termin', 'Termín reklamácie')
        ]);

        type(fixture, '#custom-field-name', 'Termín');

        const taken = fixture.nativeElement.querySelector('[data-testid="custom-field-key-taken"]');
        expect(taken.textContent).toContain('KEY_TAKEN');
        expect(keyInput(fixture).disabled).toBe(false);
        expect((fixture.componentInstance as any).isSaveDisabled).toBe(true);
    });

    it('lets the user out of a clash by editing the key', () => {
        const fixture = setup({ idProject: 10 }, null, [
            existing(3, 'termin', 'Termín reklamácie')
        ]);

        type(fixture, '#custom-field-name', 'Termín');
        type(fixture, '#custom-field-key', 'termin_dodania');

        expect(
            fixture.nativeElement.querySelector('[data-testid="custom-field-key-taken"]')
        ).toBeNull();
        expect((fixture.componentInstance as any).isSaveDisabled).toBe(false);
    });

    it('treats an archived field as still holding its key', () => {
        const fixture = setup({ idProject: 10 }, null, [
            { ...existing(3, 'owner', 'Owner'), archivedAt: '2026-01-01T00:00:00Z' }
        ]);

        type(fixture, '#custom-field-name', 'Owner');

        expect(
            fixture.nativeElement.querySelector('[data-testid="custom-field-key-taken"]')
        ).not.toBeNull();
    });

    it('asks for a key when the name yields none', () => {
        const fixture = setup({ idProject: 10 });

        type(fixture, '#custom-field-name', '客户编号');

        expect(keyInput(fixture).disabled).toBe(false);
        expect(
            fixture.nativeElement.querySelector('[data-testid="custom-field-key-ungenerated"]')
        ).not.toBeNull();
        expect((fixture.componentInstance as any).isSaveDisabled).toBe(true);
    });

    it('leaves the key of a saved field alone when it is renamed', () => {
        const fixture = setup({ idCustomField: 3, idProject: 10, key: 'impact', name: 'Impact' });
        const component = fixture.componentInstance as any;

        type(fixture, '#custom-field-name', 'Business impact');

        expect(component.form.getRawValue().key).toBe('impact');
        expect(keyInput(fixture).disabled).toBe(true);
        expect(keyEditButton(fixture)).toBeNull();
    });

    it('offers to fill existing tasks only once the field is made required', () => {
        const fixture = setup(
            { idCustomField: 3, idProject: 10, key: 'impact', name: 'Impact' },
            12
        );
        const component = fixture.componentInstance as any;

        expect(backfillBlock(fixture)).toBeNull();

        component.form.controls.isRequired.setValue(true);
        fixture.detectChanges();

        expect(backfillBlock(fixture)).not.toBeNull();
    });

    it('does not offer it when no task is missing a value', () => {
        const fixture = setup(
            { idCustomField: 3, idProject: 10, key: 'impact', name: 'Impact' },
            0
        );
        const component = fixture.componentInstance as any;

        component.form.controls.isRequired.setValue(true);
        fixture.detectChanges();

        expect(backfillBlock(fixture)).toBeNull();
    });

    it('saves without a fill value, because filling is the user’s choice', () => {
        const fixture = setup(
            { idCustomField: 3, idProject: 10, key: 'impact', name: 'Impact' },
            12
        );
        const component = fixture.componentInstance as any;
        const saved: CustomFieldFormValue[] = [];
        component.save.subscribe((value: CustomFieldFormValue) => saved.push(value));

        component.form.controls.isRequired.setValue(true);
        fixture.detectChanges();
        component.onSave();

        expect(saved).toHaveLength(1);
        expect(saved[0].backfill).toBeUndefined();
        expect(saved[0].field.isRequired).toBe(true);
    });

    it('passes the fill value along when one was entered', () => {
        const fixture = setup(
            { idCustomField: 3, idProject: 10, key: 'impact', name: 'Impact' },
            12
        );
        const component = fixture.componentInstance as any;
        const saved: CustomFieldFormValue[] = [];
        component.save.subscribe((value: CustomFieldFormValue) => saved.push(value));

        component.form.controls.isRequired.setValue(true);
        component.onBackfillChange('unknown');
        component.onSave();

        expect(saved[0].backfill).toBe('unknown');
    });

    it('keeps the task count out of the offer while it is unknown', () => {
        const fixture = setup({ idProject: 10 });
        const component = fixture.componentInstance as any;

        component.form.controls.isRequired.setValue(true);
        fixture.detectChanges();

        const block = backfillBlock(fixture);
        expect(block).not.toBeNull();
        expect(block?.querySelector('.custom-field-backfill__count')).toBeNull();
    });

    it('carries the default value on save, whether the field is required or not', () => {
        const fixture = setup({ idCustomField: 3, idProject: 10, key: 'impact', name: 'Impact' });
        const component = fixture.componentInstance as any;
        const saved: CustomFieldFormValue[] = [];
        component.save.subscribe((value: CustomFieldFormValue) => saved.push(value));

        component.onDefaultChange('unknown');
        component.onSave();

        expect(saved[0].field.isRequired).toBe(false);
        expect(saved[0].field.defaultValue).toBe('unknown');
    });

    it('shows the stored default when the field is opened for editing', async () => {
        const fixture = setup({
            idCustomField: 3,
            idProject: 10,
            key: 'impact',
            name: 'Impact',
            fieldType: CustomFieldType.Text,
            defaultValue: 'none'
        });
        await fixture.whenStable();

        const input = fixture.nativeElement.querySelector('#custom-field-default');
        expect(input.value).toBe('none');
    });

    it('withholds a default from a brand-new select field, whose options have no ids yet', () => {
        const fixture = setup({ idProject: 10 });
        const component = fixture.componentInstance as any;

        component.onTypeChange(CustomFieldType.Select);
        fixture.detectChanges();

        expect(fixture.nativeElement.querySelector('#custom-field-default')).toBeNull();
        expect(
            fixture.nativeElement.querySelector('[data-testid="custom-field-default"]').textContent
        ).toContain('DEFAULT_LATER');
    });

    it('drops the fill value when the field stops being required', () => {
        const fixture = setup(
            { idCustomField: 3, idProject: 10, key: 'impact', name: 'Impact' },
            12
        );
        const component = fixture.componentInstance as any;

        component.form.controls.isRequired.setValue(true);
        component.onBackfillChange('unknown');
        component.form.controls.isRequired.setValue(false);
        fixture.detectChanges();

        expect(backfillBlock(fixture)).toBeNull();
        component.onSave();
        expect(component.backfill()).toBeNull();
    });
});
