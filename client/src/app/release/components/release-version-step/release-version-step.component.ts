import { ChangeDetectionStrategy, Component, input } from '@angular/core';
import { FormGroup } from '@angular/forms';
import { RELEASE_NAME_MAX_LENGTH } from '../../constants/release.constants';
import { ReleaseForm } from '../../entity/release-form.entity';

@Component({
    selector: 'app-release-version-step',
    templateUrl: './release-version-step.component.html',
    standalone: false,
    changeDetection: ChangeDetectionStrategy.OnPush
})
export class ReleaseVersionStepComponent {
    public readonly form = input.required<FormGroup<ReleaseForm>>();
    public readonly tagName = input('');
    public readonly tagColor = input('');
    public readonly isNameSameAsTag = input(false);

    protected readonly maxLength = RELEASE_NAME_MAX_LENGTH;
}
