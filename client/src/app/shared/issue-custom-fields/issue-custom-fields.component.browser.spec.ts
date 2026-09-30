import { TestBed } from '@angular/core/testing';
import { FormsModule } from '@angular/forms';
import { TranslateModule } from '@ngx-translate/core';
import { of, throwError } from 'rxjs';
import { IssueCustomFieldsComponent } from './issue-custom-fields.component';
import { IssueApi } from 'src/app/issue/api/issue.api.service';
import { CustomFieldStore } from 'src/app/custom-field/store/custom-field.store';
import { CustomFieldType } from 'src/app/custom-field/constants/custom-field-type.enum';
import { CustomField } from 'src/app/custom-field/model/custom-field.model';
import { UiModule } from 'src/app/ui/ui.module';
import { UiToastService } from 'src/app/ui/service/ui-toast.service';

describe('IssueCustomFieldsComponent (browser)', () => {
    const field = (
        idCustomField: number,
        key: string,
        fieldType: CustomFieldType,
        archivedAt: string | null = null
    ): CustomField => ({
        idCustomField,
        idProject: 10,
        key,
        name: key,
        fieldType,
        isRequired: false,
        requiredSince: null,
        defaultValue: null,
        orderRank: idCustomField,
        archivedAt,
        options:
            fieldType === CustomFieldType.Select
                ? [{ idOption: 5, idCustomField, label: 'Acme', orderRank: 1 }]
                : []
    });

    let issueApi: { update$: any };
    let toast: { show: any };

    const required = (base: CustomField, requiredSince: string): CustomField => ({
        ...base,
        isRequired: true,
        requiredSince
    });

    function setup(
        fields: CustomField[],
        customFields: Record<string, string | number | boolean | null> = {},
        issueOver: Record<string, unknown> = {},
        isNewIssue = false
    ) {
        issueApi = { update$: vi.fn().mockReturnValue(of({})) };
        toast = { show: vi.fn() };

        TestBed.configureTestingModule({
            declarations: [IssueCustomFieldsComponent],
            imports: [FormsModule, UiModule, TranslateModule.forRoot()],
            providers: [
                { provide: IssueApi, useValue: issueApi },
                { provide: UiToastService, useValue: toast },
                {
                    provide: CustomFieldStore,
                    useValue: { customFieldsByProject$: () => of(fields), ensureLoaded: vi.fn() }
                }
            ]
        });
        const fixture = TestBed.createComponent(IssueCustomFieldsComponent);
        fixture.componentRef.setInput('issue', {
            idIssue: 1,
            idIssuePublic: 7,
            idProject: 10,
            customFields,
            ...issueOver
        });
        fixture.componentRef.setInput('project', { idProject: 10, name: 'P' });
        fixture.componentRef.setInput('isNewIssue', isNewIssue);
        fixture.detectChanges();
        return fixture;
    }

    it('renders one control per field of the project', () => {
        const fixture = setup([
            field(1, 'note', CustomFieldType.Text),
            field(2, 'impact', CustomFieldType.Number),
            field(3, 'due', CustomFieldType.Date),
            field(4, 'customer', CustomFieldType.Select),
            field(5, 'reviewed', CustomFieldType.Boolean)
        ]);

        const rendered = fixture.nativeElement.querySelectorAll(
            '[data-testid="issue-custom-field"]'
        );
        expect(rendered.length).toBe(5);
    });

    it('renders nothing when the project has no fields', () => {
        const fixture = setup([]);

        expect(
            fixture.nativeElement.querySelectorAll('[data-testid="issue-custom-field"]').length
        ).toBe(0);
    });

    it('saves a single key when a value changes', () => {
        const fixture = setup([field(1, 'note', CustomFieldType.Text)]);
        const component = fixture.componentInstance as any;

        component.onValueChange(field(1, 'note', CustomFieldType.Text), 'hello');

        expect(issueApi.update$).toHaveBeenCalledWith(10, 7, {
            customFields: { note: 'hello' }
        });
    });

    it('restores the previous value and warns when the save fails', () => {
        const fixture = setup([field(1, 'note', CustomFieldType.Text)], { note: 'old' });
        const component = fixture.componentInstance as any;
        issueApi.update$ = vi.fn().mockReturnValue(throwError(() => new Error('400')));

        component.onValueChange(field(1, 'note', CustomFieldType.Text), 'new');

        expect(component.valueOf(field(1, 'note', CustomFieldType.Text))).toBe('old');
        expect(toast.show).toHaveBeenCalled();
    });

    it('stays quiet when the API already explained the refusal', () => {
        const fixture = setup([field(1, 'note', CustomFieldType.Text)], { note: 'old' });
        const component = fixture.componentInstance as any;
        issueApi.update$ = vi
            .fn()
            .mockReturnValue(
                throwError(() => ({ error: { translateKey: 'error.custom_field_required' } }))
            );

        component.onValueChange(field(1, 'note', CustomFieldType.Text), null);

        expect(component.valueOf(field(1, 'note', CustomFieldType.Text))).toBe('old');
        expect(toast.show).not.toHaveBeenCalled();
    });

    it('shows the values of the task it is given now, not the one it started with', () => {
        const fixture = setup([field(1, 'note', CustomFieldType.Text)], { note: 'first' });
        const component = fixture.componentInstance as any;

        fixture.componentRef.setInput('issue', {
            idIssue: 2,
            idIssuePublic: 8,
            idProject: 10,
            customFields: { note: 'second' }
        });
        fixture.detectChanges();

        expect(component.valueOf(field(1, 'note', CustomFieldType.Text))).toBe('second');
    });

    it('marks a required field and reports it as missing while it is empty', () => {
        const fixture = setup([
            required(field(1, 'impact', CustomFieldType.Number), '2026-01-01T00:00:00Z')
        ]);
        const component = fixture.componentInstance as any;

        expect(component.missingRequiredKeys()).toEqual(['impact']);
        expect(fixture.nativeElement.querySelector('label.required')).not.toBeNull();

        component.onValueChange(field(1, 'impact', CustomFieldType.Number), '4');

        expect(component.missingRequiredKeys()).toEqual([]);
    });

    it('explains an empty required field on a task older than the rule', () => {
        const fixture = setup(
            [required(field(1, 'impact', CustomFieldType.Number), '2026-05-01T00:00:00Z')],
            {},
            { createAt: '2026-01-01T00:00:00Z' }
        );
        fixture.detectChanges();

        const hint = fixture.nativeElement.querySelector(
            '[data-testid="issue-custom-field-missing"]'
        );
        expect(hint?.textContent).toContain('CUSTOM_FIELD.MISSING_PREDATES');
    });

    it('says nothing about an empty field on a task created after the rule', () => {
        const fixture = setup(
            [required(field(1, 'impact', CustomFieldType.Number), '2026-01-01T00:00:00Z')],
            {},
            { createAt: '2026-05-01T00:00:00Z' }
        );
        fixture.detectChanges();

        expect(
            fixture.nativeElement.querySelector('[data-testid="issue-custom-field-missing"]')
        ).toBeNull();
    });

    it('prefills a default on a new task', () => {
        const fixture = setup(
            [{ ...field(1, 'impact', CustomFieldType.Number), defaultValue: 3 }],
            {},
            {},
            true
        );
        const component = fixture.componentInstance as any;

        expect(component.valueOf(field(1, 'impact', CustomFieldType.Number))).toBe(3);
    });

    it('leaves an existing task blank even when the field has a default', () => {
        const fixture = setup([{ ...field(1, 'impact', CustomFieldType.Number), defaultValue: 3 }]);
        const component = fixture.componentInstance as any;

        expect(component.valueOf(field(1, 'impact', CustomFieldType.Number))).toBeNull();
    });

    it('hands values to the parent instead of patching while the task is new', () => {
        const fixture = setup([field(1, 'note', CustomFieldType.Text)], {}, {}, true);
        const component = fixture.componentInstance as any;
        const emitted: Record<string, unknown>[] = [];
        component.valuesChange.subscribe((v: Record<string, unknown>) => emitted.push(v));

        component.onValueChange(field(1, 'note', CustomFieldType.Text), 'fresh');
        fixture.detectChanges();

        expect(issueApi.update$).not.toHaveBeenCalled();
        expect(emitted.at(-1)).toEqual({ note: 'fresh' });
    });

    it('gives the datepicker a Date, not the stored string', () => {
        const fixture = setup([field(1, 'due', CustomFieldType.Date)], {
            due: '2026-03-04T00:00:00.000Z'
        });
        const component = fixture.componentInstance as any;

        const value = component.dateValueOf(field(1, 'due', CustomFieldType.Date));
        expect(value).toBeInstanceOf(Date);
        expect((value as Date).toISOString()).toBe('2026-03-04T00:00:00.000Z');
    });

    it('hands the datepicker the same Date on every check, not a fresh one', () => {
        const fixture = setup([field(1, 'due', CustomFieldType.Date)], {
            due: '2026-03-04T00:00:00.000Z'
        });
        const component = fixture.componentInstance as any;

        const first = component.dateValueOf(field(1, 'due', CustomFieldType.Date));
        fixture.detectChanges();

        expect(component.dateValueOf(field(1, 'due', CustomFieldType.Date))).toBe(first);
    });

    it('keeps the clear affordance off a required select', () => {
        const fixture = setup([
            required(field(1, 'customer', CustomFieldType.Select), '2026-01-01T00:00:00Z'),
            field(2, 'vendor', CustomFieldType.Select)
        ]);
        const component = fixture.componentInstance as any;

        expect(component.isRequired(component.fields()[0])).toBe(true);
        expect(component.isRequired(component.fields()[1])).toBe(false);
    });

    it('keeps an archived field visible while the task still has its value', () => {
        const fixture = setup(
            [
                field(1, 'legacy', CustomFieldType.Text, '2026-01-01T00:00:00Z'),
                field(2, 'gone', CustomFieldType.Text, '2026-01-01T00:00:00Z')
            ],
            { legacy: 'old' }
        );
        const component = fixture.componentInstance as any;

        expect(component.fields().map((f: CustomField) => f.key)).toEqual(['legacy']);
    });
});
