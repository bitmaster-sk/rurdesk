import { Component, Directive, input, output } from '@angular/core';
import { User } from 'src/app/auth/model/user.model';

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

@Component({ selector: 'app-avatar', template: '', standalone: true })
export class AvatarStub {
    public readonly name = input<string>('');
    public readonly bgColor = input<string>('');
    public readonly height = input<number>(0);
    public readonly width = input<number>(0);
    public readonly radius = input<number>(0);
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
    public readonly messageChange = output<string>();
    public readonly cancel = output<void>();
}

@Component({ selector: 'app-message-body', template: '', standalone: true })
export class MessageBodyStub {
    public readonly body = input<string>('');
    public readonly messageKind = input<unknown>(undefined);
    public readonly candidates = input<unknown>(undefined);
    public readonly approvable = input<boolean>(false);
    public readonly selectedMockupRef = input<string | null>(null);
    public readonly useMockup = output<string>();
}

@Component({ selector: 'app-auth-robot', template: '', standalone: true })
export class AuthRobotStub {}

@Component({ selector: 'ui-toast', template: '', standalone: true })
export class UiToastStub {}

@Component({ selector: 'ui-button', template: '<ng-content></ng-content>', standalone: true })
export class UiButtonStub {
    public readonly label = input<string>('');
    public readonly loading = input<boolean>(false);
    public readonly disabled = input<boolean>(false);
    public readonly ariaLabel = input<string>('');
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
export class UiLoaderStub {}

@Component({ selector: 'ui-dialog', template: '<ng-content></ng-content>', standalone: true })
export class UiDialogStub {
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

@Directive({ selector: '[uiTooltip]', standalone: true })
export class UiTooltipStub {
    public readonly uiTooltip = input<string>('');
    public readonly uiTooltipPosition = input<string>('top');
}
