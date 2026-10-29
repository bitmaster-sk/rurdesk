import { HttpClient } from '@angular/common/http';
import { Injectable, inject } from '@angular/core';
import { Observable } from 'rxjs';
import { CreateTagReq, EditTagReq, Tag } from '../model/tag.model';

@Injectable({
    providedIn: 'root'
})
export class TagApi {
    private readonly http = inject(HttpClient);

    public load$(): Observable<Tag[]> {
        return this.http.get<Tag[]>('/api/private/tag');
    }

    public insert$(tag: CreateTagReq): Observable<Tag> {
        return this.http.post<Tag>('/api/private/tag', tag);
    }

    public update$(tag: EditTagReq): Observable<Tag> {
        return this.http.patch<Tag>(`/api/private/tag/${tag.idTag}`, tag);
    }

    public delete$(idProject: number, idTag: number): Observable<Tag> {
        return this.http.delete<Tag>(`/api/private/tag/${idTag}/project/${idProject}`);
    }
}
