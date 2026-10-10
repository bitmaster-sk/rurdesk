import { ChangeDetectionStrategy, Component, computed, inject } from '@angular/core';
import { AttachmentGallery } from '../../service/attachment-gallery.service';
import { AttachmentStore } from '../../store/attachment.store';

@Component({
    selector: 'app-attachment-lightbox',
    changeDetection: ChangeDetectionStrategy.OnPush,
    standalone: false,
    templateUrl: './attachment-lightbox.component.html'
})
export class AttachmentLightboxComponent {
    protected readonly gallery = inject(AttachmentGallery);
    private readonly store = inject(AttachmentStore);

    protected readonly hasMany = computed(() => this.gallery.images().length > 1);
    protected readonly url = computed(() => {
        const idAttachment = this.gallery.active()?.idAttachment;
        const state = idAttachment ? this.store.state(idAttachment) : null;
        return state?.status === 'ready' ? state.url : null;
    });

    protected onVisibleChange(isVisible: boolean): void {
        if (!isVisible) {
            this.gallery.close();
        }
    }
}
