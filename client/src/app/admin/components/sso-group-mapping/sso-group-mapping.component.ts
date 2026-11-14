import {
    ChangeDetectionStrategy,
    Component,
    computed,
    effect,
    inject,
    input,
    output
} from '@angular/core';
import { FormBuilder, Validators } from '@angular/forms';
import { Team } from 'src/app/team/model/team.model';
import { InsertSsoMappingReq, SsoGroupMapping } from '../../model/admin-sso.model';

@Component({
    selector: 'app-sso-group-mapping',
    templateUrl: './sso-group-mapping.component.html',
    standalone: false,
    changeDetection: ChangeDetectionStrategy.OnPush
})
export class SsoGroupMappingComponent {
    private readonly fb = inject(FormBuilder);

    public readonly mappings = input.required<SsoGroupMapping[]>();
    public readonly teams = input.required<Team[]>();
    public readonly suggestedGroups = input<string[]>([]);
    public readonly isDisabled = input(false);
    public readonly add = output<InsertSsoMappingReq>();
    public readonly remove = output<number>();

    protected readonly form = this.fb.group({
        groupValue: this.fb.nonNullable.control('', [
            Validators.required,
            Validators.maxLength(500)
        ]),
        idTeam: this.fb.control<number | null>(null, [Validators.required])
    });

    protected readonly rows = computed(() => {
        const teamsById = new Map(this.teams().map(team => [team.idTeam, team]));
        return this.mappings().map(mapping => ({
            mapping,
            team: teamsById.get(mapping.idTeam) ?? { name: mapping.teamName, color: '' }
        }));
    });

    protected readonly unmappedGroups = computed(() => {
        const mapped = new Set(this.mappings().map(mapping => mapping.groupValue));
        return this.suggestedGroups().filter(group => !mapped.has(group));
    });

    public constructor() {
        effect(() => {
            if (this.isDisabled()) {
                this.form.disable({ emitEvent: false });
            } else {
                this.form.enable({ emitEvent: false });
            }
        });
    }

    protected onUseGroup(group: string): void {
        this.form.controls.groupValue.setValue(group);
    }

    protected onAdd(): void {
        const { groupValue, idTeam } = this.form.getRawValue();
        if (this.form.invalid || idTeam === null) {
            return;
        }
        this.add.emit({ groupValue: groupValue.trim(), idTeam });
        this.form.reset({ groupValue: '', idTeam: null });
    }
}
