import {
    ChangeDetectionStrategy,
    Component,
    computed,
    inject,
    input,
    output,
    signal
} from '@angular/core';
import { toObservable, toSignal } from '@angular/core/rxjs-interop';
import { FormControl, FormGroup, NonNullableFormBuilder } from '@angular/forms';
import { switchMap } from 'rxjs/operators';
import { TagStore } from 'src/app/tag/store/tag.store';
import { ReleaseApi } from '../../api/release.api.service';
import { ReleaseStatus } from '../../constants/release-status.enum';
import { ReleaseTagAction } from '../../constants/release-tag-action.enum';
import { Release, ReleaseView } from '../../model/release.model';

interface ReleaseDeleteForm {
    action: FormControl<ReleaseTagAction>;
    idTagTarget: FormControl<number | null>;
}

@Component({
    selector: 'app-release-delete-dialog',
    templateUrl: './release-delete-dialog.component.html',
    standalone: false,
    changeDetection: ChangeDetectionStrategy.OnPush
})
export class ReleaseDeleteDialogComponent {
    public readonly release = input.required<ReleaseView>();

    public readonly deleted = output<Release>();
    public readonly closed = output<void>();

    private readonly api = inject(ReleaseApi);
    private readonly tagStore = inject(TagStore);
    private readonly fb = inject(NonNullableFormBuilder);

    protected readonly ReleaseTagAction = ReleaseTagAction;
    protected readonly visible = signal(true);
    protected readonly isBusy = signal(false);

    protected readonly form: FormGroup<ReleaseDeleteForm> = this.fb.group<ReleaseDeleteForm>({
        action: this.fb.control(ReleaseTagAction.Keep),
        idTagTarget: new FormControl<number | null>(null)
    });
    private readonly formValue = toSignal(this.form.valueChanges, {
        initialValue: this.form.getRawValue()
    });

    private readonly projectTags = toSignal(
        toObservable(this.release).pipe(
            switchMap(release => this.tagStore.tagsByProject$(release.idProject))
        ),
        { initialValue: [] }
    );

    protected readonly targets = computed(() =>
        this.projectTags().filter(tag => tag.idTag !== this.release().idTag)
    );
    protected readonly versionTag = computed(
        () => this.projectTags().find(tag => tag.idTag === this.release().idTag) ?? null
    );
    protected readonly isDraft = computed(() => this.release().status === ReleaseStatus.Draft);
    protected readonly isMove = computed(
        () => !this.isDraft() && this.formValue().action !== ReleaseTagAction.Keep
    );
    protected readonly isValid = computed(
        () => !this.isMove() || (this.formValue().idTagTarget ?? null) !== null
    );

    protected onDelete(): void {
        if (this.isBusy()) {
            return;
        }
        const release = this.release();
        const { action, idTagTarget } = this.form.getRawValue();
        this.isBusy.set(true);
        this.api
            .delete$(
                release.idProject,
                release.idRelease,
                this.isDraft()
                    ? {}
                    : {
                          tag: action,
                          ...(this.isMove() && idTagTarget !== null ? { idTagTarget } : {})
                      }
            )
            .subscribe({
                next: deleted => {
                    this.isBusy.set(false);
                    this.deleted.emit(deleted);
                    this.visible.set(false);
                },
                error: () => this.isBusy.set(false)
            });
    }

    protected onCancel(): void {
        this.visible.set(false);
    }

    protected onHide(): void {
        this.closed.emit();
    }
}
