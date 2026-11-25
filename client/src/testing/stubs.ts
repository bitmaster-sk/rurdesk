import { NgTemplateOutlet } from '@angular/common';
import {
    Component,
    contentChild,
    Directive,
    forwardRef,
    input,
    output,
    TemplateRef
} from '@angular/core';
import { User } from 'src/app/auth/model/user.model';
import { IssueType } from 'src/app/issue-type/model/issue-type.model';
import { IssueState } from 'src/app/state/model/issue-state.model';
import { ControlValueAccessor, NG_VALUE_ACCESSOR } from '@angular/forms';

/**
 * Shared standalone stubs for browser component specs.
 *
 * Specs must not use NO_ERRORS_SCHEMA / CUSTOM_ELEMENTS_SCHEMA — instead they
 * import the stubs matching the children their template renders, so every
 * element and property binding resolves and the test log stays free of
 * NG0303/NG0304 noise. Stubs declare exactly the inputs/outputs bound in the
 * templates under test; extend them when a template gains a new binding.
 */

// The real TablerIconComponent is not exported by UiModule and needs each icon
// registered via provideTablerIcons — tests don't assert icons, so stub it.
@Component({ selector: 'tabler-icon', template: '', standalone: true })
export class TablerIconStub {
    public readonly icon = input<string>('');
    public readonly size = input<unknown>(undefined);
    public readonly stroke = input<unknown>(undefined);
    public readonly svgClass = input<string>('');
}

@Component({ selector: 'app-attachment-lightbox', template: '', standalone: true })
export class AttachmentLightboxStub {}

@Component({ selector: 'app-avatar', template: '', standalone: true })
export class AvatarStub {
    public readonly name = input<string>('');
    public readonly bgColor = input<string>('');
    public readonly height = input<number>(0);
    public readonly width = input<number>(0);
    public readonly radius = input<number>(0);
    public readonly disabled = input<boolean>(false);
    public readonly manualInitials = input<string | null | undefined>(null);
}

@Component({ selector: 'app-message-editor', template: '', standalone: true })
export class MessageEditorStub {
    public readonly mode = input<string>('create');
    public readonly change = input<string>('onaction');
    public readonly message = input<string>('');
    public readonly mentionCandidates = input<User[]>([]);
    public readonly sendIcon = input<string>('');
    public readonly sendLabel = input<string>('');
    public readonly disableCancelButton = input<boolean>(true);
    public readonly attachmentScope = input<unknown>(null);
    public readonly messageChange = output<string>();
    public readonly cancelled = output<void>();
    /** Text the host put back into the editor through `writeValue`. */
    public written: string | null = null;

    public writeValue(value: string): void {
        this.written = value;
    }
}

@Component({ selector: 'app-agent-feedback-bar', template: '', standalone: true })
export class AgentFeedbackBarStub {
    public readonly run = input<unknown>(null);
    public readonly feedback = input<unknown>(undefined);
    public readonly agentName = input<string | null>(null);
    public readonly hasMockups = input<boolean>(false);
    public readonly isSending = input<boolean>(false);
    public readonly send = output<void>();
    public readonly approve = output<void>();
}

@Component({ selector: 'app-message-body', template: '', standalone: true })
export class MessageBodyStub {
    public readonly body = input<string>('');
    public readonly messageKind = input<unknown>(undefined);
    public readonly candidates = input<unknown>(undefined);
    public readonly approvable = input<boolean>(false);
    public readonly selectedMockupRef = input<string | null>(null);
    public readonly anchorBreaks = input<number[]>([]);
    public readonly anchorSlot = input<unknown>(null);
    public readonly anchorHighlights = input<unknown[]>([]);
    public readonly useMockup = output<string>();
    public readonly anchorClick = output<number>();
}

@Component({ selector: 'app-auth-robot', template: '', standalone: true })
export class AuthRobotStub {}

@Component({ selector: 'ui-toast', template: '', standalone: true })
export class UiToastStub {}

@Component({ selector: 'ui-button', template: '<ng-content></ng-content>', standalone: true })
export class UiButtonStub {
    public readonly label = input<string>('');
    public readonly severity = input<string>('');
    public readonly variant = input<string>('');
    public readonly size = input<string>('');
    public readonly loading = input<boolean>(false);
    public readonly disabled = input<boolean>(false);
    public readonly ariaLabel = input<string>('');
    public readonly isIconOnly = input<boolean>(false);
}

@Component({ selector: 'ui-tag', template: '{{ value() }}', standalone: true })
export class UiTagStub {
    public readonly value = input<string>('');
    public readonly severity = input<string>('secondary');
}

@Component({ selector: 'ui-badge', template: '', standalone: true })
export class UiBadgeStub {
    public readonly value = input<unknown>(undefined);
}

@Component({ selector: 'ui-odometer', template: '', standalone: true })
export class UiOdometerStub {
    public readonly value = input<number>(0);
}

@Component({ selector: 'ui-loader', template: '', standalone: true })
export class UiLoaderStub {
    public readonly size = input<number>(16);
    public readonly ariaLabel = input<string>('');
}

// Keeps the real dialog's slots: the scrolling body and the fixed `#footer`.
@Component({
    selector: 'ui-dialog',
    template: `
        <div class="ui-dialog__content"><ng-content></ng-content></div>
        @if (footer(); as footer) {
            <div class="ui-dialog__footer"><ng-container [ngTemplateOutlet]="footer" /></div>
        }
    `,
    standalone: true,
    imports: [NgTemplateOutlet]
})
export class UiDialogStub {
    protected readonly footer = contentChild<TemplateRef<unknown>>('footer');
    public readonly visible = input<boolean>(false);
    public readonly header = input<string>('');
    public readonly dismissable = input<boolean>(true);
    public readonly closable = input<boolean>(true);
    public readonly closeOnEscape = input<boolean>(true);
    public readonly panelClass = input<string>('');
    public readonly width = input<string>('');
    public readonly height = input<string>('');
    public readonly flush = input<boolean>(false);
    public readonly visibleChange = output<boolean>();
}

@Directive({ selector: '[uiConfirm]', standalone: true })
export class UiConfirmStub {
    public readonly confirmText = input<string>();
    public readonly acceptLabel = input<string>();
    public readonly rejectLabel = input<string>();
    public readonly confirmed = output<void>();
}

@Directive({ selector: '[uiTooltip]', standalone: true })
export class UiTooltipStub {
    public readonly uiTooltip = input<string | TemplateRef<unknown>>('');
    public readonly uiTooltipPosition = input<string>('top');
}

@Component({
    selector: 'ui-day-select',
    template: '',
    standalone: true,
    providers: [
        {
            provide: NG_VALUE_ACCESSOR,
            useExisting: forwardRef(() => UiDaySelectStub),
            multi: true
        }
    ]
})
export class UiDaySelectStub implements ControlValueAccessor {
    public readonly maxDate = input<Date | null>(null);
    public readonly format = input<unknown>(undefined);
    public readonly inputId = input<string | null>(null);
    public readonly ariaLabel = input<string | null>(null);
    public readonly disabled = input<boolean>(false);
    public readonly fluid = input<boolean>(false);

    public writeValue(): void {}

    public registerOnChange(): void {}

    public registerOnTouched(): void {}
}

@Component({
    selector: 'app-tag-input',
    template: '',
    standalone: true,
    providers: [
        { provide: NG_VALUE_ACCESSOR, useExisting: forwardRef(() => TagInputStub), multi: true }
    ]
})
export class TagInputStub implements ControlValueAccessor {
    public readonly tags = input<unknown>(null);
    public readonly canCreate = input(false);
    public readonly colors = input<unknown>(null);
    public readonly defaultColor = input<string>('');
    public readonly inputId = input<string>('');
    public readonly createTag = output<unknown>();

    public writeValue(): void {}

    public registerOnChange(): void {}

    public registerOnTouched(): void {}
}

@Component({ selector: 'app-issue-tags', template: '', standalone: true })
export class IssueTagsStub {
    public readonly idsTag = input<number[] | null | undefined>([]);
    public readonly max = input(3);
}

@Component({ selector: 'app-split-dialog', template: '', standalone: true })
export class SplitDialogStub {
    public readonly idProject = input<number>(0);
    public readonly issue = input<any>(null);
    public readonly accepted = output<any>();
    public readonly cancelled = output<void>();
}

@Component({ selector: 'app-state-badge', template: '{{ state()?.name }}', standalone: true })
export class StateBadgeStub {
    public readonly state = input<IssueState | undefined>(undefined);
    public readonly size = input<string>('default');
}

@Component({ selector: 'app-severity-circle', template: '', standalone: true })
export class SeverityCircleStub {
    public readonly color = input<string | null | undefined>(undefined);
    public readonly size = input<number>(1);
}

@Component({
    selector: 'app-issue-type-badge',
    template: '{{ issueType()?.name }}',
    standalone: true
})
export class IssueTypeBadgeStub {
    public readonly issueType = input<IssueType | undefined>(undefined);
    public readonly size = input<string>('default');
}

@Component({ selector: 'app-tag-chip', template: '{{ name() }}', standalone: true })
export class TagChipStub {
    public readonly name = input<string>('');
    public readonly color = input<string | null>(null);
}

/**
 * Form-bound pickers. `pick()` stands in for the user choosing an option;
 * `value` and `isDisabled` show what the form wrote into the control.
 */
abstract class FormPickerStub implements ControlValueAccessor {
    public value: unknown = null;
    public isDisabled = false;
    private onChange: (value: unknown) => void = () => {};

    public pick(value: unknown): void {
        this.value = value;
        this.onChange(value);
    }

    public writeValue(value: unknown): void {
        this.value = value;
    }

    public registerOnChange(fn: (value: unknown) => void): void {
        this.onChange = fn;
    }

    public registerOnTouched(): void {}

    public setDisabledState(isDisabled: boolean): void {
        this.isDisabled = isDisabled;
    }
}

@Component({
    selector: 'ui-choice',
    template: '',
    standalone: true,
    providers: [
        { provide: NG_VALUE_ACCESSOR, useExisting: forwardRef(() => UiChoiceStub), multi: true }
    ]
})
export class UiChoiceStub extends FormPickerStub {
    public readonly options = input<readonly unknown[]>([]);
    public readonly optionLabel = input<string>('');
    public readonly optionValue = input<string>('');
    public readonly size = input<string>('default');
    public readonly allowEmpty = input<boolean>(true);
}

@Component({
    selector: 'ui-select',
    template: '',
    standalone: true,
    providers: [
        { provide: NG_VALUE_ACCESSOR, useExisting: forwardRef(() => UiSelectStub), multi: true }
    ]
})
export class UiSelectStub extends FormPickerStub {
    public readonly options = input<readonly unknown[] | null>([]);
    public readonly optionLabel = input<string>('');
    public readonly optionValue = input<string>('');
    public readonly placeholder = input<string>('');
    public readonly filter = input<boolean>(false);
    public readonly size = input<string | undefined>(undefined);
    public readonly emptyMessage = input<string>('');
    public readonly inputId = input<string>('');
}
