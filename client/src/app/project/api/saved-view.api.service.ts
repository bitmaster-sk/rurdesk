import { HttpClient } from '@angular/common/http';
import { Injectable, inject } from '@angular/core';
import { Observable } from 'rxjs';
import { SavedView, SavedViewReq } from '../model/saved-view.model';

@Injectable({ providedIn: 'root' })
export class SavedViewApi {
    private readonly http = inject(HttpClient);

    public loadByProject$(idProject: number): Observable<SavedView[]> {
        return this.http.get<SavedView[]>(`/api/private/project/${idProject}/saved-view`);
    }

    public create$(idProject: number, body: SavedViewReq): Observable<SavedView> {
        return this.http.post<SavedView>(`/api/private/project/${idProject}/saved-view`, body);
    }

    public edit$(idSavedView: number, body: SavedViewReq): Observable<SavedView> {
        return this.http.patch<SavedView>(`/api/private/saved-view/${idSavedView}`, body);
    }

    public delete$(idSavedView: number): Observable<void> {
        return this.http.delete<void>(`/api/private/saved-view/${idSavedView}`);
    }
}
