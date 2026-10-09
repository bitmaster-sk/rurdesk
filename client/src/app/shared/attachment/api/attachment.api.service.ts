import { HttpClient } from '@angular/common/http';
import { Injectable, inject } from '@angular/core';
import { Observable } from 'rxjs';
import { RequestContext } from 'src/app/core/request-context';
import { AttachmentScope } from '../entity/attachment-scope.entity';
import { UploadedAttachment } from '../model/attachment.model';

@Injectable({ providedIn: 'root' })
export class AttachmentApi {
    private readonly http = inject(HttpClient);

    public upload$(file: File, scope: AttachmentScope): Observable<UploadedAttachment> {
        const body = new FormData();
        body.append('idMessageRecipientType', String(scope.idMessageRecipientType));
        body.append('idRecipient', String(scope.idRecipient));
        body.append('file', file, file.name);
        return this.http.post<UploadedAttachment>('/api/private/attachment', body);
    }

    public download$(idAttachment: string): Observable<Blob> {
        return this.http.get(`/api/private/attachment/${idAttachment}`, {
            responseType: 'blob',
            context: RequestContext.disableErrorToast()
        });
    }
}
