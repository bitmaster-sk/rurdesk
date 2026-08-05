import { ChangeDetectionStrategy, Component, OnInit, inject, input, output } from '@angular/core';
import { FormBuilder, Validators } from '@angular/forms';
import { SavedView } from 'src/app/project/model/saved-view.model';
import { SavedViewFormValue } from '../saved-view-menu/saved-view-menu.component';

@Component({
    selector: 'app-saved-view-dialog',
    templateUrl: './saved-view-dialog.component.html',
    standalone: false,
    changeDetection: ChangeDetectionStrategy.OnPush
})
export class SavedViewDialogComponent implements OnInit {
    private readonly fb = inject(FormBuilder);

    public readonly view = input<SavedView | null>(null);

    public readonly saved = output<SavedViewFormValue>();

    public readonly cancelled = output<void>();

    protected readonly form = this.fb.nonNullable.group({
        name: this.fb.nonNullable.control('', [Validators.required, Validators.maxLength(60)]),
        isShared: this.fb.nonNullable.control(false)
    });

    public ngOnInit(): void {
        const view = this.view();
        this.form.reset({ name: view?.name ?? '', isShared: view?.isShared ?? false });
    }

    protected onSave(): void {
        if (this.form.invalid) {
            return;
        }
        this.saved.emit(this.form.getRawValue());
    }
}
