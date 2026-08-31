import { AbstractControl, ValidationErrors } from '@angular/forms';

export abstract class NotBlankValidator {
    public static validate(this: void, control: AbstractControl<string>): ValidationErrors | null {
        return control.value.trim() ? null : { required: true };
    }
}
