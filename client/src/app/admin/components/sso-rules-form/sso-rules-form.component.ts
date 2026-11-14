import {
    ChangeDetectionStrategy,
    Component,
    computed,
    effect,
    inject,
    input,
    output,
    untracked
} from '@angular/core';
import { FormBuilder, Validators } from '@angular/forms';
import { SsoRulesConverter } from '../../converter/sso-rules.converter';
import { SsoRules, UpdateSsoRulesReq } from '../../model/admin-sso.model';

@Component({
    selector: 'app-sso-rules-form',
    templateUrl: './sso-rules-form.component.html',
    standalone: false,
    changeDetection: ChangeDetectionStrategy.OnPush
})
export class SsoRulesFormComponent {
    private readonly fb = inject(FormBuilder);

    public readonly rules = input.required<SsoRules>();
    public readonly isDisabled = input(false);
    public readonly save = output<UpdateSsoRulesReq>();

    private readonly storedRules = computed(() => this.rules(), {
        equal: (a, b) => JSON.stringify(a) === JSON.stringify(b)
    });

    protected readonly form = this.fb.nonNullable.group({
        allowedDomains: [''],
        isAutoCreate: [false],
        adminGroup: ['', [Validators.maxLength(500)]]
    });

    public constructor() {
        effect(() => {
            const rules = this.storedRules();
            untracked(() =>
                this.form.patchValue({
                    allowedDomains: SsoRulesConverter.toText(rules.allowedDomains),
                    isAutoCreate: rules.isAutoCreate,
                    adminGroup: rules.adminGroup
                })
            );
        });
        effect(() => {
            if (this.isDisabled()) {
                this.form.disable({ emitEvent: false });
            } else {
                this.form.enable({ emitEvent: false });
            }
        });
    }

    protected onSave(): void {
        if (this.form.invalid) {
            return;
        }
        const value = this.form.getRawValue();
        this.save.emit({
            allowedDomains: SsoRulesConverter.toDomains(value.allowedDomains),
            isAutoCreate: value.isAutoCreate,
            adminGroup: value.adminGroup.trim()
        });
    }
}
