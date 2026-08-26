import { ChangeDetectionStrategy, Component, DestroyRef, inject, signal } from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { FormBuilder } from '@angular/forms';
import { debounceTime } from 'rxjs/operators';
import { ToastNotificationService } from '../../../core/toast-notification.service';
import { SkillApi } from '../../../shared/api/skill.api.service';
import { I18nService } from '../../../shared/i18n/i18n.service';
import { Skill } from '../../../shared/model/skill.model';
import { UiSaveState } from '../../../ui/components/save-status/save-status-chip.component';

type SkillField = 'name' | 'description' | 'content';

@Component({
    selector: 'app-admin-skills',
    templateUrl: './admin-skills.component.html',
    styleUrls: ['./admin-skills.component.scss'],
    standalone: false,
    changeDetection: ChangeDetectionStrategy.OnPush
})
export class AdminSkillsComponent {
    private readonly skillApi = inject(SkillApi);
    private readonly formBuilder = inject(FormBuilder);
    private readonly toast = inject(ToastNotificationService);
    private readonly i18n = inject(I18nService);
    private readonly destroyRef = inject(DestroyRef);

    protected readonly skills = signal<Skill[]>([]);
    protected readonly selected = signal<Skill | null>(null);
    protected readonly isLoading = signal(true);

    private readonly fieldStatus = signal<Record<SkillField, UiSaveState>>({
        name: UiSaveState.Idle,
        description: UiSaveState.Idle,
        content: UiSaveState.Idle
    });

    protected readonly form = this.formBuilder.nonNullable.group({
        name: '',
        description: '',
        content: ''
    });

    private lastSaved: Record<SkillField, string> = { name: '', description: '', content: '' };

    public constructor() {
        this.loadList();

        this.form.valueChanges
            .pipe(debounceTime(600), takeUntilDestroyed())
            .subscribe(() => this.saveChangedFields());
    }

    protected badgeKey(skill: Skill): string {
        if (skill.isEdited) {
            return 'SKILL.BADGE.EDITED';
        }
        return skill.isBuiltin ? 'SKILL.BADGE.BUILTIN' : 'SKILL.BADGE.CUSTOM';
    }

    protected fieldSaveStatus(field: SkillField): UiSaveState {
        return this.fieldStatus()[field];
    }

    protected onSelect(skill: Skill): void {
        this.applyDetail(skill);
    }

    protected onCreate(): void {
        this.skillApi
            .create$({
                name: this.uniqueDefaultName(),
                description: '',
                content: this.i18n.instant('SKILL.DEFAULT_CONTENT')
            })
            .subscribe({
                next: detail => {
                    this.applyDetail(detail);
                    this.loadList();
                },
                error: () => this.toast.showError('SKILL.SAVE_ERROR')
            });
    }

    protected onRestore(): void {
        const current = this.selected();
        if (!current) {
            return;
        }
        this.skillApi
            .restore$(current.idSkill)
            .pipe(takeUntilDestroyed(this.destroyRef))
            .subscribe({
                next: detail => {
                    this.applyDetail(detail);
                    this.loadList();
                },
                error: () => this.toast.showError('SKILL.SAVE_ERROR')
            });
    }

    protected onDelete(): void {
        const current = this.selected();
        if (!current) {
            return;
        }
        this.skillApi
            .delete$(current.idSkill)
            .pipe(takeUntilDestroyed(this.destroyRef))
            .subscribe({
                next: () => {
                    this.selected.set(null);
                    this.loadList();
                },
                error: () => this.toast.showError('SKILL.SAVE_ERROR')
            });
    }

    private loadList(): void {
        this.skillApi
            .load$()
            .pipe(takeUntilDestroyed(this.destroyRef))
            .subscribe({
                next: all => {
                    this.skills.set(all);
                    this.isLoading.set(false);
                },
                error: () => {
                    this.isLoading.set(false);
                    this.toast.showError('SKILL.LOAD_ERROR');
                }
            });
    }

    private applyDetail(detail: Skill): void {
        this.selected.set(detail);
        // emitEvent:false — filling the editor is not an edit and must not save.
        this.form.patchValue(
            { name: detail.name, description: detail.description, content: detail.content },
            { emitEvent: false }
        );
        this.lastSaved = {
            name: detail.name,
            description: detail.description,
            content: detail.content
        };
        this.fieldStatus.set({
            name: UiSaveState.Idle,
            description: UiSaveState.Idle,
            content: UiSaveState.Idle
        });
    }

    private saveChangedFields(): void {
        const current = this.selected();
        if (!current) {
            return;
        }
        const value = this.form.getRawValue();
        const changed = (['name', 'description', 'content'] as SkillField[]).filter(
            field => value[field] !== this.lastSaved[field]
        );
        if (changed.length === 0) {
            return;
        }

        const body: Partial<Record<SkillField, string>> = {};
        changed.forEach(field => {
            body[field] = value[field];
            this.setFieldStatus(field, UiSaveState.Saving);
        });

        this.skillApi
            .update$(current.idSkill, body)
            .pipe(takeUntilDestroyed(this.destroyRef))
            .subscribe({
                next: detail => {
                    this.selected.set(detail);
                    changed.forEach(field => {
                        this.lastSaved[field] = value[field];
                        this.setFieldStatus(field, UiSaveState.Saved);
                    });
                    this.loadList();
                },
                error: () => {
                    changed.forEach(field => this.setFieldStatus(field, UiSaveState.Error));
                    this.toast.showError('SKILL.SAVE_ERROR');
                }
            });
    }

    private setFieldStatus(field: SkillField, status: UiSaveState): void {
        this.fieldStatus.update(all => ({ ...all, [field]: status }));
    }

    private uniqueDefaultName(): string {
        const base = this.i18n.instant('SKILL.DEFAULT_NAME');
        const taken = new Set(this.skills().map(skill => skill.name));
        if (!taken.has(base)) {
            return base;
        }
        let suffix = 2;
        while (taken.has(`${base} ${suffix}`)) {
            suffix++;
        }
        return `${base} ${suffix}`;
    }
}
