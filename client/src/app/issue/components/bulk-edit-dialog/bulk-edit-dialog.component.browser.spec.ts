import { ComponentFixture, TestBed } from '@angular/core/testing';
import { By } from '@angular/platform-browser';
import { Component, forwardRef, input } from '@angular/core';
import { ControlValueAccessor, FormsModule, NG_VALUE_ACCESSOR } from '@angular/forms';
import { TranslateModule } from '@ngx-translate/core';
import { provideNoopAnimations } from '@angular/platform-browser/animations';
import { of } from 'rxjs';
import { TablerIconStub } from 'src/testing/stubs';
import { UiModule } from 'src/app/ui/ui.module';
import { StateStore } from 'src/app/state/store/state.store';
import { SeverityStore } from 'src/app/severity/store/severity.store';
import { IssueTypeStore } from 'src/app/issue-type/store/issue-type.store';
import { ProjectMemberStore } from 'src/app/project/project-member.store';
import { TagStore } from 'src/app/tag/store/tag.store';
import { UiToastService } from 'src/app/ui/service/ui-toast.service';
import { User } from 'src/app/auth/model/user.model';
import { Tag } from 'src/app/tag/model/tag.model';
import { IssueBulkApi } from '../../api/issue-bulk.api.service';
import { Issue } from '../../model/issue.model';
import { BulkEditDialogComponent } from './bulk-edit-dialog.component';

@Component({ selector: 'app-tag-chip', template: '{{ name() }}', standalone: true })
class TagChipStub {
    public readonly name = input<string>('');
    public readonly color = input<string | null>(null);
}

abstract class CvaStub implements ControlValueAccessor {
    public value: unknown = null;
    public emit: (value: unknown) => void = () => {};
    public writeValue(value: unknown): void {
        this.value = value;
    }
    public registerOnChange(fn: (value: unknown) => void): void {
        this.emit = fn;
    }
    public registerOnTouched(): void {}
}

@Component({
    selector: 'app-state-dropdown',
    template: '',
    standalone: true,
    providers: [
        {
            provide: NG_VALUE_ACCESSOR,
            useExisting: forwardRef(() => StateSelectorStub),
            multi: true
        }
    ]
})
class StateSelectorStub extends CvaStub {
    public readonly states = input<unknown[]>([]);
    public readonly placeholder = input<string>();
    public readonly inputId = input<string>();
}

@Component({
    selector: 'app-severity-dropdown',
    template: '',
    standalone: true,
    providers: [
        {
            provide: NG_VALUE_ACCESSOR,
            useExisting: forwardRef(() => SeveritySelectorStub),
            multi: true
        }
    ]
})
class SeveritySelectorStub extends CvaStub {
    public readonly severities = input<unknown[]>([]);
    public readonly placeholder = input<string>();
    public readonly inputId = input<string>();
}

@Component({
    selector: 'app-issue-type-dropdown',
    template: '',
    standalone: true,
    providers: [
        { provide: NG_VALUE_ACCESSOR, useExisting: forwardRef(() => TypeSelectorStub), multi: true }
    ]
})
class TypeSelectorStub extends CvaStub {
    public readonly issueTypes = input<unknown[]>([]);
    public readonly placeholder = input<string>();
    public readonly inputId = input<string>();
}

@Component({
    selector: 'app-user-dropdown',
    template: '',
    standalone: true,
    providers: [
        { provide: NG_VALUE_ACCESSOR, useExisting: forwardRef(() => UserDropdownStub), multi: true }
    ]
})
class UserDropdownStub extends CvaStub {
    public readonly users = input<unknown[]>([]);
    public readonly placeholder = input<string>();
    public readonly filter = input(false);
}

const issue = (idIssuePublic: number, extra: Partial<Issue> = {}): Issue => ({
    idIssue: idIssuePublic,
    idIssuePublic,
    idProject: 1,
    idState: 1,
    idSeverity: null,
    idIssueType: null,
    title: `Task ${idIssuePublic}`,
    description: '',
    tracked: 0,
    ...extra
});

const tags: Tag[] = [
    { idTag: 10, idProject: 1, name: 'next', color: '#0f0', orderRank: 0, issueCount: 1 },
    { idTag: 20, idProject: 1, name: 'ux', color: '#f0f', orderRank: 1, issueCount: 0 }
];

const users: User[] = [
    { idUser: 1, name: 'Tomas', email: '', colorAvatarBg: '' },
    { idUser: 5, name: 'Agent 003', email: '', colorAvatarBg: '', isAgent: true }
];

describe('BulkEditDialogComponent (TestBed)', () => {
    let fixture: ComponentFixture<BulkEditDialogComponent>;
    let comp: any;
    let update$: ReturnType<typeof vi.fn>;
    let toastShow: ReturnType<typeof vi.fn>;
    let applied: Issue[][];

    const byTestId = (id: string): HTMLElement | null =>
        document.querySelector(`[data-testid="${id}"]`);

    const buttonIn = (id: string): HTMLButtonElement => byTestId(id)!.querySelector('button')!;

    beforeEach(async () => {
        update$ = vi.fn(() => of([issue(1), issue(2)]));
        toastShow = vi.fn();
        await TestBed.configureTestingModule({
            declarations: [BulkEditDialogComponent],
            imports: [
                TranslateModule.forRoot(),
                FormsModule,
                UiModule,
                TablerIconStub,
                TagChipStub,
                StateSelectorStub,
                SeveritySelectorStub,
                TypeSelectorStub,
                UserDropdownStub
            ],
            providers: [
                provideNoopAnimations(),
                {
                    provide: StateStore,
                    useValue: {
                        states$: of([
                            { idState: 1, idProject: 1, name: 'NEW' },
                            { idState: 4, idProject: 1, name: 'DONE' }
                        ])
                    }
                },
                { provide: SeverityStore, useValue: { severities$: of([]) } },
                { provide: IssueTypeStore, useValue: { issueTypes$: of([]) } },
                { provide: ProjectMemberStore, useValue: { users$: of(users) } },
                { provide: TagStore, useValue: { tagsByProject$: () => of(tags) } },
                { provide: IssueBulkApi, useValue: { update$ } },
                { provide: UiToastService, useValue: { show: toastShow } }
            ]
        }).compileComponents();

        fixture = TestBed.createComponent(BulkEditDialogComponent);
        comp = fixture.componentInstance;
        fixture.componentRef.setInput('idProject', 1);
        fixture.componentRef.setInput('issues', [
            issue(1, { idsTag: [10], assignedTo: 5 }),
            issue(2, { assignedTo: 1 })
        ]);
        applied = [];
        comp.applied.subscribe((updated: Issue[]) => applied.push(updated));
        fixture.detectChanges();
        await fixture.whenStable();
        fixture.detectChanges();
    });

    afterEach(() => {
        fixture.destroy();
    });

    it('keeps Review disabled until something changes', () => {
        expect(buttonIn('bulk-review').disabled).toBe(true);

        comp.idState.set(4);
        fixture.detectChanges();

        expect(buttonIn('bulk-review').disabled).toBe(false);
    });

    it('clearing a dropdown returns the field to No change', () => {
        const state = fixture.debugElement.query(By.directive(StateSelectorStub))
            .componentInstance as StateSelectorStub;
        expect(state.placeholder()).toBe('ISSUE.BULK_EDIT.NO_CHANGE');

        state.emit(4);
        fixture.detectChanges();
        expect(buttonIn('bulk-review').disabled).toBe(false);

        state.emit(null);
        fixture.detectChanges();
        expect(comp.idState()).toBeNull();
        expect(buttonIn('bulk-review').disabled).toBe(true);
    });

    it('offers Unassigned and people but no agents, and × returns to No change', () => {
        const assignee = fixture.debugElement.query(By.directive(UserDropdownStub))
            .componentInstance as UserDropdownStub;
        const options = assignee.users() as User[];
        expect(options.map(user => user.name)).toEqual(['ISSUE.BULK_EDIT.UNASSIGN', 'Tomas']);
        expect(assignee.placeholder()).toBe('ISSUE.BULK_EDIT.NO_CHANGE');

        assignee.emit(options[0].idUser);
        fixture.detectChanges();
        expect(comp.assignee()).toBeNull();

        assignee.emit(null);
        fixture.detectChanges();
        expect(comp.assignee()).toBeUndefined();
        expect(buttonIn('bulk-review').disabled).toBe(true);
    });

    it('shows how many selected tasks carry each tag and cycles a mixed tag', () => {
        const mixed = byTestId('bulk-tag-10')!;
        expect(mixed.getAttribute('aria-checked')).toBe('mixed');
        expect(mixed.textContent).toContain('ISSUE.BULK_EDIT.TAG_COUNT');

        mixed.click();
        fixture.detectChanges();
        expect(byTestId('bulk-tag-10')!.getAttribute('aria-checked')).toBe('true');
        expect(byTestId('bulk-tag-10')!.textContent).toContain('ISSUE.BULK_EDIT.TAG_ADD');

        byTestId('bulk-tag-10')!.click();
        fixture.detectChanges();
        expect(byTestId('bulk-tag-10')!.textContent).toContain('ISSUE.BULK_EDIT.TAG_REMOVE');
    });

    it('reviews the changes before applying them', () => {
        comp.idState.set(4);
        comp.assignee.set(1);
        fixture.detectChanges();
        buttonIn('bulk-review').click();
        fixture.detectChanges();

        const summary = byTestId('bulk-summary')!.textContent;
        expect(summary).toContain('DONE');
        expect(summary).toContain('Tomas');
    });

    it('applies one entry per task with only the changed fields', () => {
        comp.idState.set(4);
        byTestId('bulk-tag-20')!.click();
        fixture.detectChanges();
        buttonIn('bulk-review').click();
        fixture.detectChanges();
        buttonIn('bulk-apply').click();
        fixture.detectChanges();

        expect(update$).toHaveBeenCalledWith(1, [
            { idIssuePublic: 1, idState: 4, addIdsTag: [20] },
            { idIssuePublic: 2, idState: 4, addIdsTag: [20] }
        ]);
        expect(toastShow).toHaveBeenCalledWith(expect.objectContaining({ severity: 'success' }));
        expect(applied).toHaveLength(1);
    });

    it('sends an explicit null assignee for Unassign', () => {
        comp.assignee.set(null);
        comp.onApply();

        expect(update$).toHaveBeenCalledWith(1, [
            { idIssuePublic: 1, idUserAssigned: null },
            { idIssuePublic: 2, idUserAssigned: null }
        ]);
    });
});
