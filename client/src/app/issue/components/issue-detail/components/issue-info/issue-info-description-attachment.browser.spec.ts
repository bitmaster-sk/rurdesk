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
import { MessageRecipientType } from 'src/app/message/constant/message-recipient-type.enum';
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

    public registerOnChange(): void {
        return;
    }

    public registerOnTouched(): void {
        return;
    }
}

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
    public focus(): void {
        return;
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

const NEW = { idProject: 7, title: 'New issue', description: 'text' } as unknown as Issue;

async function render(issue: Issue): Promise<ComponentFixture<IssueInfoComponent>> {
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
            { provide: IssueApi, useValue: { update$: vi.fn().mockReturnValue(NEVER) } },
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

function descriptionEditor(fixture: ComponentFixture<IssueInfoComponent>): DescriptionEditorStub {
    return fixture.debugElement.query(By.directive(DescriptionEditorStub))
        .componentInstance as DescriptionEditorStub;
}

describe('IssueInfoComponent description attachments', () => {
    it('uploads into the project while the task does not exist yet', async () => {
        const fixture = await render(NEW);

        expect(descriptionEditor(fixture).attachmentScope()).toEqual({
            idMessageRecipientType: MessageRecipientType.project,
            idRecipient: 7
        });
    });

    it('uploads into the task once it exists', async () => {
        const fixture = await render(EXISTING);
        fixture.componentInstance.onToggleInputDescription();
        fixture.detectChanges();

        expect(descriptionEditor(fixture).attachmentScope()).toEqual({
            idMessageRecipientType: MessageRecipientType.issue,
            idRecipient: 10
        });
    });

    it('does not save a new task while a description file is uploading', async () => {
        const fixture = await render(NEW);
        const save = (fixture.nativeElement as HTMLElement).querySelector(
            '[data-testid="issue-create-save"] button'
        ) as HTMLButtonElement;
        expect(save.disabled).toBe(false);

        descriptionEditor(fixture).isUploading.set(true);
        fixture.detectChanges();

        expect(save.disabled).toBe(true);
    });

    it('keeps the description editor open until its upload has finished', async () => {
        const fixture = await render(EXISTING);
        fixture.componentInstance.onToggleInputDescription();
        fixture.detectChanges();
        const editor = descriptionEditor(fixture);

        editor.isUploading.set(true);
        editor.messageChange.emit('Existing text');
        fixture.detectChanges();
        expect(fixture.componentInstance.showInputDescription()).toBe(true);

        editor.isUploading.set(false);
        fixture.detectChanges();
        expect(fixture.componentInstance.showInputDescription()).toBe(false);
    });

    it('stays open when the user comes back to the editor before the upload ends', async () => {
        const fixture = await render(EXISTING);
        fixture.componentInstance.onToggleInputDescription();
        fixture.detectChanges();
        const editor = descriptionEditor(fixture);

        editor.isUploading.set(true);
        editor.messageChange.emit('Existing text');
        fixture.debugElement
            .query(By.directive(DescriptionEditorStub))
            .nativeElement.parentElement.dispatchEvent(new FocusEvent('focusin'));
        editor.isUploading.set(false);
        fixture.detectChanges();

        expect(fixture.componentInstance.showInputDescription()).toBe(true);
    });
});
