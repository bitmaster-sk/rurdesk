import { Injectable, inject } from '@angular/core';
import { Observable } from 'rxjs';
import { map } from 'rxjs/operators';
import { AttachmentApi } from '../api/attachment.api.service';
import { AttachmentLinkConverter } from '../converter/attachment-link.converter';
import { AttachmentScope } from '../entity/attachment-scope.entity';
import { UploadedAttachmentLink } from '../entity/uploaded-attachment-link.entity';

@Injectable({ providedIn: 'root' })
export class AttachmentUploader {
    private readonly api = inject(AttachmentApi);

    public upload$(file: File, scope: AttachmentScope): Observable<UploadedAttachmentLink> {
        return this.api
            .upload$(file, scope)
            .pipe(map(attachment => AttachmentLinkConverter.toUploadedLink(attachment)));
    }
}
