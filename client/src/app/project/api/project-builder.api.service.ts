import { HttpClient } from '@angular/common/http';
import { Injectable, inject } from '@angular/core';
import { Observable } from 'rxjs';
import {
    ProjectBuilderAcceptRes,
    ProjectBuilderGenerateReq,
    ProjectBuilderGenerateRes,
    ProjectBuilderIssue
} from '../model/project-builder.model';

@Injectable({ providedIn: 'root' })
export class ProjectBuilderApi {
    constructor(private http: HttpClient) {}

    public generate$(
        idProject: number,
        req: ProjectBuilderGenerateReq
    ): Observable<ProjectBuilderGenerateRes> {
        return this.http.post<ProjectBuilderGenerateRes>(
            `/api/private/project/${idProject}/project-builder/generate`,
            req
        );
    }

    public accept$(
        idProject: number,
        issues: ProjectBuilderIssue[]
    ): Observable<ProjectBuilderAcceptRes> {
        return this.http.post<ProjectBuilderAcceptRes>(
            `/api/private/project/${idProject}/project-builder/accept`,
            { issues }
        );
    }
}
