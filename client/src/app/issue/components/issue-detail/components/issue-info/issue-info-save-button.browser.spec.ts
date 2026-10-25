import { Component, forwardRef, input, output, signal } from '@angular/core';
import { ComponentFixture, TestBed } from '@angular/core/testing';
import { By } from '@angular/platform-browser';
import { ControlValueAccessor, NG_VALUE_ACCESSOR, ReactiveFormsModule } from '@angular/forms';
import { Router } from '@angular/router';
import { TranslateModule } from '@ngx-translate/core';
import { NEVER, of } from 'rxjs';
import { describe, expect, it, vi } from 'vitest';
import { AuthStore } from 'src/app/auth/store/auth.store';
import { IssueTypeStore } from 'src/app/issue-type/store/issue-type.store';
import { TagStore } from 'src/app/tag/store/tag.store';
import { IssueApi } from 'src/app/issue/api/issue.api.service';
import { MrDiffApi } from 'src/app/issue/api/mr-diff.api.service';
import { Issue } from 'src/app/issue/model/issue.model';
import { PinApi } from 'src/app/pin/api/pin.api.service';
import { GitIntegrationApi } from 'src/app/project/api/git-integration.api.service';
import { ProjectMemberStore } from 'src/app/project/project-member.store';
import { ProjectStore } from 'src/app/project/project.store';
import { SeverityStore } from 'src/app/severity/store/severity.store';
import { SecDurationPipe } from 'src/app/shared/duration/sec-duration.pipe';
import { StateStore } from 'src/app/state/store/state.store';
import { UiModule } from 'src/app/ui/ui.module';
import { MessageBodyStub, TablerIconStub, TagInputStub } from 'src/testing/stubs';
import { IssueInfoComponent } from './issue-info.component';

abstract class ValueAccessorStub implements ControlValueAccessor {
    public writeValue(): void {
        return;
    }

    public registerOnChange(_fn: unknown): void {
        return;
    }

    public registerOnTouched(_fn: unknown): void {
        return;
    }
}

// MessageEditorComponent.propagateChange() fires the CVA change AND touched
// callbacks on every keystroke, which is what commits a blur-updated control
// early. The stub keeps the registered callbacks so a test can "type" into the
// description without touching the editor's DOM.
@Component({
    selector: 'app-message-editor',
    template: '',
    standalone: true,
    providers: [
        {
            provide: NG_VALUE_ACCESSOR,
            useExisting: forwardRef(() => DescriptionEditorStub),
            multi: true
        }
    ]
})
class DescriptionEditorStub extends ValueAccessorStub {
    public readonly disableSendButton = input(false);
    public readonly attachmentScope = input<unknown>(null);
    public readonly messageChange = output<string>();
    public readonly isUploading = signal(false);

    private changeFn: (value: string) => void = () => undefined;
    private touchFn: () => void = () => undefined;

    public override registerOnChange(fn: (value: string) => void): void {
        this.changeFn = fn;
    }

    public override registerOnTouched(fn: () => void): void {
        this.touchFn = fn;
    }

    public type(value: string): void {
        this.changeFn(value);
        this.touchFn();
    }
}

@Component({
    selector: 'app-issue-type-dropdown',
    template: '',
    standalone: true,
    providers: [
        {
            provide: NG_VALUE_ACCESSOR,
            useExisting: forwardRef(() => IssueTypeDropdownStub),
            multi: true
        }
    ]
})
class IssueTypeDropdownStub extends ValueAccessorStub {
    public readonly issueTypes = input<unknown>(null);
}

@Component({
    selector: 'app-state-dropdown',
    template: '',
    standalone: true,
    providers: [
        {
            provide: NG_VALUE_ACCESSOR,
            useExisting: forwardRef(() => StateDropdownStub),
            multi: true
        }
    ]
})
class StateDropdownStub extends ValueAccessorStub {
    public readonly states = input<unknown>(null);
}

@Component({
    selector: 'app-severity-dropdown',
    template: '',
    standalone: true,
    providers: [
        {
            provide: NG_VALUE_ACCESSOR,
            useExisting: forwardRef(() => SeverityDropdownStub),
            multi: true
        }
    ]
})
class SeverityDropdownStub extends ValueAccessorStub {
    public readonly severities = input<unknown>(null);
}

@Component({
    selector: 'app-user-dropdown',
    template: '',
    standalone: true,
    providers: [
        { provide: NG_VALUE_ACCESSOR, useExisting: forwardRef(() => UserDropdownStub), multi: true }
    ]
})
class UserDropdownStub extends ValueAccessorStub {
    public readonly users = input<unknown>(null);
    public readonly hasAgentFeatures = input(false);
    public readonly idProject = input<number | null>(null);
    public readonly idIssuePublic = input<number | null>(null);
    public readonly agentRunCreated = output<unknown>();
}

@Component({ selector: 'app-tracker', template: '', standalone: true })
class TrackerStub {
    public readonly issue = input<unknown>(null);
    public readonly project = input<unknown>(null);
    public readonly trackAdded = output<unknown>();
}

@Component({ selector: 'app-issue-custom-fields', template: '', standalone: true })
class IssueCustomFieldsStub {
    public readonly issue = input<unknown>(null);
    public readonly project = input<unknown>(null);
    public readonly isNewIssue = input(false);
    public readonly valuesChange = output<unknown>();
    public readonly missingRequiredChange = output<boolean>();
}

@Component({ selector: 'app-quality-panel', template: '', standalone: true })
class QualityPanelStub {
    public readonly idProject = input<unknown>(null);
    public readonly idIssue = input<unknown>(null);
    public readonly title = input<unknown>(null);
    public readonly description = input<unknown>(null);
    public readonly titleChanged = output<string>();
    public readonly descriptionChanged = output<string>();
}

@Component({ selector: 'app-issue-participants', template: '', standalone: true })
class IssueParticipantsStub {
    public readonly idProject = input<unknown>(null);
    public readonly idIssue = input<unknown>(null);
    public readonly idIssuePublic = input<unknown>(null);
    public readonly members = input<unknown>(null);
    public readonly currentUserId = input<unknown>(null);
}

@Component({ selector: 'app-issue-wiki-panel', template: '', standalone: true })
class IssueWikiPanelStub {
    public readonly idProject = input<unknown>(null);
    public readonly idIssue = input<unknown>(null);
    public readonly canEdit = input(false);
}

const EXISTING = {
    idIssue: 10,
    idIssuePublic: 42,
    idProject: 7,
    title: 'Some issue',
    description: 'Existing text',
    assignedTo: null,
    estimated: 0,
    idGitIntegration: null,
    mrId: null
} as unknown as Issue;

// The blank draft the create route hands to the form: no ids and no text —
// exactly what a user gets when starting a brand-new task.
const BLANK = { idProject: 7, title: '', description: '' } as unknown as Issue;

let apiUpdate$: ReturnType<typeof vi.fn>;

async function render(issue: Issue): Promise<ComponentFixture<IssueInfoComponent>> {
    apiUpdate$ = vi.fn().mockReturnValue(of(EXISTING));
    await TestBed.configureTestingModule({
        declarations: [IssueInfoComponent, SecDurationPipe],
        imports: [
            ReactiveFormsModule,
            TranslateModule.forRoot(),
            UiModule,
            TablerIconStub,
            MessageBodyStub,
            DescriptionEditorStub,
            IssueTypeDropdownStub,
            StateDropdownStub,
            SeverityDropdownStub,
            UserDropdownStub,
            TrackerStub,
            IssueCustomFieldsStub,
            QualityPanelStub,
            IssueParticipantsStub,
            IssueWikiPanelStub,
            TagInputStub
        ],
        providers: [
            { provide: IssueApi, useValue: { update$: apiUpdate$ } },
            { provide: StateStore, useValue: { statesByProject$: () => of([]) } },
            { provide: SeverityStore, useValue: { severitiesByProject$: () => of([]) } },
            { provide: IssueTypeStore, useValue: { issueTypesByProject$: () => of([]) } },
            {
                provide: TagStore,
                useValue: { tagsByProject$: () => of([]), isDeleted: () => false }
            },
            {
                provide: ProjectMemberStore,
                useValue: { users$: of([]), usersMap$: of(new Map()) }
            },
            { provide: ProjectStore, useValue: { project$: of({ idProject: 7, name: 'p' }) } },
            { provide: AuthStore, useValue: { getUser: () => ({ idUser: 1 }) } },
            { provide: PinApi, useValue: { insert$: () => NEVER } },
            { provide: MrDiffApi, useValue: { loadStatus$: () => NEVER, load$: () => NEVER } },
            { provide: GitIntegrationApi, useValue: { loadOne$: () => NEVER } },
            { provide: Router, useValue: { navigate: vi.fn() } }
        ]
    }).compileComponents();
    const fixture = TestBed.createComponent(IssueInfoComponent);
    fixture.componentRef.setInput('issue', issue);
    fixture.componentRef.setInput('project', { idProject: 7, name: 'p' });
    fixture.detectChanges();
    await fixture.whenStable();
    return fixture;
}

function saveButton(fixture: ComponentFixture<IssueInfoComponent>): HTMLButtonElement {
    const el = fixture.nativeElement as HTMLElement;
    return el.querySelector('[data-testid="issue-create-save"] button') as HTMLButtonElement;
}

function titleInput(fixture: ComponentFixture<IssueInfoComponent>): HTMLInputElement {
    const el = fixture.nativeElement as HTMLElement;
    return el.querySelector('#issue-title') as HTMLInputElement;
}

function descriptionEditor(fixture: ComponentFixture<IssueInfoComponent>): DescriptionEditorStub {
    return fixture.debugElement.query(By.directive(DescriptionEditorStub))
        .componentInstance as DescriptionEditorStub;
}

// Types into the title while it keeps focus: the value goes through the native
// input event and NO blur/focusout is dispatched, so an updateOn: 'blur'
// control would leave the value uncommitted until the input loses focus.
function typeInTitle(fixture: ComponentFixture<IssueInfoComponent>, value: string): void {
    const input = titleInput(fixture);
    input.focus();
    input.value = value;
    input.dispatchEvent(new Event('input', { bubbles: true }));
    fixture.detectChanges();
}

describe('IssueInfoComponent create save button (browser)', () => {
    it('enables the save button while the cursor is still in the title input', async () => {
        const fixture = await render(BLANK);
        expect(saveButton(fixture).disabled).toBe(true);

        descriptionEditor(fixture).type('typed description');
        fixture.detectChanges();
        // The required title is still empty, so the button stays off.
        expect(saveButton(fixture).disabled).toBe(true);

        typeInTitle(fixture, 'typed title');
        expect(saveButton(fixture).disabled).toBe(false);
    });

    it('disables the save button again when the title is cleared without blurring', async () => {
        const fixture = await render(BLANK);
        descriptionEditor(fixture).type('typed description');
        typeInTitle(fixture, 'typed title');
        expect(saveButton(fixture).disabled).toBe(false);

        typeInTitle(fixture, '');
        expect(saveButton(fixture).disabled).toBe(true);
    });

    it('keeps edit-mode autosave on blur: no update while typing, one update on blur', async () => {
        const fixture = await render(EXISTING);

        typeInTitle(fixture, 'edited title');
        expect(apiUpdate$).not.toHaveBeenCalled();

        titleInput(fixture).dispatchEvent(new Event('blur'));
        fixture.detectChanges();

        expect(apiUpdate$).toHaveBeenCalledTimes(1);
        expect(apiUpdate$).toHaveBeenCalledWith(
            7,
            42,
            expect.objectContaining({ title: 'edited title' })
        );
    });
});
