import { ChangeDetectionStrategy, Component, computed, inject, signal } from '@angular/core';
import { toSignal } from '@angular/core/rxjs-interop';
import { FormControl, NonNullableFormBuilder, Validators } from '@angular/forms';
import { ActivatedRoute, Router } from '@angular/router';
import { I18nService } from 'src/app/shared/i18n/i18n.service';
import { TagStore } from 'src/app/tag/store/tag.store';
import { UiStepperStep } from 'src/app/ui/components/stepper/stepper.component';
import { ReleaseApi } from '../../api/release.api.service';
import { RELEASE_NAME_MAX_LENGTH } from '../../constants/release.constants';
import { ReleaseStatus } from '../../constants/release-status.enum';
import { ReleaseStep } from '../../constants/release-step.enum';

interface ReleaseNewForm {
    idTag: FormControl<number | null>;
    name: FormControl<string>;
}

@Component({
    selector: 'app-release-new',
    templateUrl: './release-new.page.html',
    styleUrls: ['../release/release.page.scss'],
    standalone: false,
    changeDetection: ChangeDetectionStrategy.OnPush
})
export class ReleaseNewPage {
    private readonly route = inject(ActivatedRoute);
    private readonly router = inject(Router);
    private readonly api = inject(ReleaseApi);
    private readonly i18n = inject(I18nService);
    private readonly fb = inject(NonNullableFormBuilder);
    private readonly tagStore = inject(TagStore);

    protected readonly idProject = Number(this.route.snapshot.paramMap.get('idProject'));
    protected readonly maxLength = RELEASE_NAME_MAX_LENGTH;

    protected readonly form = this.fb.group<ReleaseNewForm>({
        idTag: new FormControl<number | null>(null, { validators: [Validators.required] }),
        name: this.fb.control('', [
            Validators.required,
            Validators.maxLength(RELEASE_NAME_MAX_LENGTH),
            Validators.pattern(/\S/)
        ])
    });
    private readonly formValue = toSignal(this.form.valueChanges, {
        initialValue: this.form.getRawValue()
    });

    protected readonly tags = toSignal(this.tagStore.tagsByProject$(this.idProject), {
        initialValue: []
    });
    private readonly releases = toSignal(this.api.load$(this.idProject), { initialValue: [] });
    protected readonly isBusy = signal(false);

    protected readonly steps = computed<UiStepperStep[]>(() => [
        { label: this.i18n.instant('RELEASE.STEP.VERSION') },
        { label: this.i18n.instant('RELEASE.STEP.TASKS'), isDisabled: true },
        { label: this.i18n.instant('RELEASE.STEP.NOTES'), isDisabled: true }
    ]);
    protected readonly tagName = computed(
        () => this.tags().find(tag => tag.idTag === this.formValue().idTag)?.name ?? ''
    );
    protected readonly existingDraft = computed(
        () =>
            this.releases().find(
                release =>
                    release.status === ReleaseStatus.Draft &&
                    release.idTag === this.formValue().idTag
            ) ?? null
    );
    protected readonly isValid = computed(() => {
        this.formValue();
        return this.form.valid && !this.existingDraft();
    });

    public constructor() {
        this.tagStore.load();
    }

    protected onCreate(): void {
        const { idTag, name } = this.form.getRawValue();
        if (idTag === null || !this.isValid() || this.isBusy()) {
            return;
        }
        this.isBusy.set(true);
        this.api.insert$(this.idProject, { idTag, name: name.trim() }).subscribe({
            next: release =>
                void this.router.navigate(
                    ['/project', this.idProject, 'release', release.idRelease],
                    {
                        queryParams: { step: ReleaseStep.Tasks },
                        replaceUrl: true
                    }
                ),
            error: () => this.isBusy.set(false)
        });
    }
}
