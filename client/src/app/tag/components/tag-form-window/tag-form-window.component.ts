import { ChangeDetectionStrategy, Component, inject } from '@angular/core';
import { WindowConfig } from 'src/app/shared/window/entity/window-config';
import { WindowReference } from 'src/app/shared/window/window.reference';
import { Tag, EditTagReq } from '../../model/tag.model';
import { TagStore } from '../../store/tag.store';

export interface TagWindowData {
    tag: Partial<EditTagReq>;
    defaultColor: string;
}

@Component({
    selector: 'app-tag-form-window',
    templateUrl: './tag-form-window.component.html',
    standalone: false,
    changeDetection: ChangeDetectionStrategy.OnPush
})
export class TagFormWindowComponent {
    private readonly winRef = inject(WindowReference);
    private readonly tagStore = inject(TagStore);
    protected readonly winCfg = inject<WindowConfig<TagWindowData>>(WindowConfig);

    protected onSave(tag: EditTagReq): void {
        const saver = tag.idTag ? this.tagStore.update$(tag) : this.tagStore.insert$(tag);
        saver.subscribe((saved: Tag) => this.winRef.close(saved));
    }

    protected onCancel(): void {
        this.winRef.close(null);
    }
}
